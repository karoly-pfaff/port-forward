package forwarders

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/domain"
)

const DefaultUDPSessionTimeout = 60 * time.Second
const udpEventThrottleInterval = int64(time.Second) // nanoseconds

// udpSession holds a per-client target socket for bidirectional-multi-client mode.
// The gen field is used to ensure that a stale idle timer does not expire a
// session that was renewed by a subsequent packet before the timer fired.
type udpSession struct {
	conn       *net.UDPConn
	clientAddr *net.UDPAddr
	timer      *time.Timer
	gen        int
}

type UDPForwarder struct {
	rule           domain.ForwardRule
	log            LogFunc
	onEvent        activity.EventFunc
	sessionTimeout time.Duration

	mu         sync.Mutex
	listenConn *net.UDPConn
	targetConn *net.UDPConn // shared socket for one-way and bidirectional-last-client
	lastClient *net.UDPAddr // most-recent client for bidirectional-last-client
	sessions   map[string]*udpSession
	running    bool
	started    string
	lastErr    string

	packetsIn  int64
	packetsOut int64
	bytesIn    int64
	bytesOut   int64

	// Throttle timestamps for packet activity events (unix nanoseconds, atomic).
	lastForwardEventNs int64
	lastReturnEventNs  int64

	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewUDPForwarder(rule domain.ForwardRule, log LogFunc, onEvent activity.EventFunc) *UDPForwarder {
	return NewUDPForwarderWithTimeout(rule, log, onEvent, DefaultUDPSessionTimeout)
}

func NewUDPForwarderWithTimeout(rule domain.ForwardRule, log LogFunc, onEvent activity.EventFunc, sessionTimeout time.Duration) *UDPForwarder {
	return &UDPForwarder{
		rule:           rule,
		log:            log,
		onEvent:        onEvent,
		sessionTimeout: sessionTimeout,
		sessions:       make(map[string]*udpSession),
	}
}

func (f *UDPForwarder) Start() error {
	listenAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(f.rule.ListenHost, fmt.Sprintf("%d", f.rule.ListenPort)))
	if err != nil {
		f.setLastError(err)
		return err
	}

	listenConn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		f.setLastError(err)
		return err
	}

	mode := f.udpMode()
	var targetConn *net.UDPConn

	if mode == domain.UdpModeOneWay || mode == domain.UdpModeBidirectionalLast {
		targetAddr, rerr := net.ResolveUDPAddr("udp4", net.JoinHostPort(f.rule.TargetHost, fmt.Sprintf("%d", f.rule.TargetPort)))
		if rerr != nil {
			_ = listenConn.Close()
			f.setLastError(rerr)
			return rerr
		}
		tc, derr := net.DialUDP("udp4", nil, targetAddr)
		if derr != nil {
			_ = listenConn.Close()
			f.setLastError(derr)
			return derr
		}
		targetConn = tc
	}

	f.mu.Lock()
	f.listenConn = listenConn
	f.targetConn = targetConn
	f.running = true
	f.started = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	f.lastErr = ""
	f.mu.Unlock()

	f.logInfo("UDP rule started", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "listen", listenAddr.String(), "mode", string(mode))

	f.wg.Add(1)
	go f.listenLoop(listenConn)

	if mode == domain.UdpModeBidirectionalLast && targetConn != nil {
		f.wg.Add(1)
		go f.targetReadLoop(listenConn, targetConn)
	}

	return nil
}

func (f *UDPForwarder) Stop() {
	f.stopOnce.Do(func() {
		f.mu.Lock()
		f.running = false
		listenConn := f.listenConn
		targetConn := f.targetConn
		f.listenConn = nil
		f.targetConn = nil
		f.lastClient = nil

		sessions := make([]*udpSession, 0, len(f.sessions))
		for _, s := range f.sessions {
			sessions = append(sessions, s)
		}
		f.sessions = make(map[string]*udpSession)
		f.mu.Unlock()

		for _, s := range sessions {
			s.timer.Stop()
			_ = s.conn.Close()
		}
		if targetConn != nil {
			_ = targetConn.Close()
		}
		if listenConn != nil {
			_ = listenConn.Close()
		}

		waitDone := make(chan struct{})
		go func() {
			f.wg.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
			f.setLastError(fmt.Errorf("timed out waiting for UDP forwarder shutdown"))
		}

		f.logInfo("UDP rule stopped", "ruleId", f.rule.ID, "ruleName", f.rule.Name)
	})
}

func (f *UDPForwarder) Status() domain.ForwardStatus {
	f.mu.Lock()
	running := f.running
	started := f.started
	lastErr := f.lastErr
	numSessions := len(f.sessions)
	f.mu.Unlock()

	pIn := atomic.LoadInt64(&f.packetsIn)
	pOut := atomic.LoadInt64(&f.packetsOut)

	status := domain.ForwardStatus{
		RuleID:     f.rule.ID,
		Running:    running,
		BytesIn:    atomic.LoadInt64(&f.bytesIn),
		BytesOut:   atomic.LoadInt64(&f.bytesOut),
		PacketsIn:  &pIn,
		PacketsOut: &pOut,
		LastError:  lastErr,
	}
	if running {
		status.StartedAt = started
	}
	if f.udpMode() == domain.UdpModeBidirectionalMulti {
		status.ActiveUdpSessions = &numSessions
	}
	return status
}

func (f *UDPForwarder) listenLoop(listenConn *net.UDPConn) {
	defer f.wg.Done()
	buf := make([]byte, 65536)
	mode := f.udpMode()

	for {
		n, clientAddr, err := listenConn.ReadFromUDP(buf)
		if err != nil {
			if f.isRunning() {
				f.setLastError(err)
				f.logInfo("UDP listen error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", err)
			}
			return
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])
		atomic.AddInt64(&f.packetsIn, 1)
		atomic.AddInt64(&f.bytesIn, int64(n))

		switch mode {
		case domain.UdpModeOneWay:
			f.mu.Lock()
			tc := f.targetConn
			f.mu.Unlock()
			if tc != nil {
				if _, werr := tc.Write(packet); werr != nil && f.isRunning() {
					f.setLastError(werr)
					f.logInfo("UDP packet forward error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", werr)
					f.emitPacketError(werr.Error())
				} else if werr == nil {
					f.maybeEmitForwarded(clientAddr, n)
				}
			}

		case domain.UdpModeBidirectionalLast:
			f.mu.Lock()
			f.lastClient = clientAddr
			tc := f.targetConn
			f.mu.Unlock()
			if tc != nil {
				if _, werr := tc.Write(packet); werr != nil && f.isRunning() {
					f.setLastError(werr)
					f.logInfo("UDP packet forward error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", werr)
					f.emitPacketError(werr.Error())
				} else if werr == nil {
					f.maybeEmitForwarded(clientAddr, n)
				}
			}

		case domain.UdpModeBidirectionalMulti:
			f.handleMultiClientPacket(listenConn, clientAddr, packet)
		}
	}
}

// targetReadLoop reads responses from the shared target socket and routes them
// to the most recent client. Used only in bidirectional-last-client mode.
// NOTE: With multiple concurrent clients, responses may be routed to the wrong
// client because only the most recently seen client address is tracked.
func (f *UDPForwarder) targetReadLoop(listenConn *net.UDPConn, targetConn *net.UDPConn) {
	defer f.wg.Done()
	buf := make([]byte, 65536)

	for {
		n, err := targetConn.Read(buf)
		if err != nil {
			return
		}

		f.mu.Lock()
		client := f.lastClient
		f.mu.Unlock()

		if client == nil {
			continue
		}

		atomic.AddInt64(&f.packetsOut, 1)
		atomic.AddInt64(&f.bytesOut, int64(n))

		packet := make([]byte, n)
		copy(packet, buf[:n])

		if _, werr := listenConn.WriteToUDP(packet, client); werr != nil && f.isRunning() {
			f.setLastError(werr)
			f.logInfo("UDP return error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", werr)
			f.emitPacketError(werr.Error())
		} else if werr == nil {
			f.maybeEmitReturned(client, n)
		}
	}
}

func (f *UDPForwarder) handleMultiClientPacket(listenConn *net.UDPConn, clientAddr *net.UDPAddr, packet []byte) {
	sessionKey := clientAddr.String()

	f.mu.Lock()
	session, exists := f.sessions[sessionKey]

	if exists {
		// Reset idle timer using gen to prevent a stale timer from expiring
		// a session that has been renewed.
		session.timer.Stop()
		session.gen++
		gen := session.gen
		session.timer = time.AfterFunc(f.sessionTimeout, func() {
			f.expireSession(sessionKey, session, gen)
		})
		f.mu.Unlock()
	} else {
		targetAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(f.rule.TargetHost, fmt.Sprintf("%d", f.rule.TargetPort)))
		if err != nil {
			f.mu.Unlock()
			f.setLastError(err)
			return
		}
		tc, err := net.DialUDP("udp4", nil, targetAddr)
		if err != nil {
			f.mu.Unlock()
			f.setLastError(err)
			f.logInfo("UDP session open error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "client", clientAddr.String(), "error", err)
			return
		}

		session = &udpSession{
			conn:       tc,
			clientAddr: clientAddr,
			gen:        0,
		}
		gen := session.gen
		session.timer = time.AfterFunc(f.sessionTimeout, func() {
			f.expireSession(sessionKey, session, gen)
		})

		f.sessions[sessionKey] = session
		f.mu.Unlock()

		f.logInfo("UDP session opened", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "client", clientAddr.String())
		f.emitSessionOpened(clientAddr)

		f.wg.Add(1)
		go f.sessionReadLoop(listenConn, session, clientAddr)
	}

	f.maybeEmitForwarded(clientAddr, len(packet))

	if _, werr := session.conn.Write(packet); werr != nil && f.isRunning() {
		f.setLastError(werr)
		f.logInfo("UDP session write error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "client", clientAddr.String(), "error", werr)
		f.emitPacketError(werr.Error())
	}
}

func (f *UDPForwarder) sessionReadLoop(listenConn *net.UDPConn, session *udpSession, clientAddr *net.UDPAddr) {
	defer f.wg.Done()
	buf := make([]byte, 65536)

	for {
		n, err := session.conn.Read(buf)
		if err != nil {
			return
		}

		if !f.isRunning() {
			return
		}

		atomic.AddInt64(&f.packetsOut, 1)
		atomic.AddInt64(&f.bytesOut, int64(n))

		packet := make([]byte, n)
		copy(packet, buf[:n])

		if _, werr := listenConn.WriteToUDP(packet, clientAddr); werr != nil && f.isRunning() {
			f.setLastError(werr)
			f.logInfo("UDP session return error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "client", clientAddr.String(), "error", werr)
			f.emitPacketError(werr.Error())
		} else if werr == nil {
			f.maybeEmitReturned(clientAddr, n)
		}
	}
}

// expireSession removes a session from the map and closes its target socket.
// The gen parameter ensures stale timers cannot expire sessions that have been
// renewed by subsequent packets arriving before the timer fired.
func (f *UDPForwarder) expireSession(key string, s *udpSession, capturedGen int) {
	f.mu.Lock()
	existing, ok := f.sessions[key]
	if !ok || existing != s || existing.gen != capturedGen {
		f.mu.Unlock()
		return
	}
	delete(f.sessions, key)
	f.mu.Unlock()

	_ = s.conn.Close()
	f.logInfo("UDP session expired", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "client", s.clientAddr.String())
	f.emitSessionClosed(s.clientAddr)
}

// maybeEmitForwarded emits udp.packet.forwarded throttled to once per second.
func (f *UDPForwarder) maybeEmitForwarded(clientAddr *net.UDPAddr, bytes int) {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&f.lastForwardEventNs)
	if now-last >= udpEventThrottleInterval && atomic.CompareAndSwapInt64(&f.lastForwardEventNs, last, now) {
		id, name, proto := f.rule.ID, f.rule.Name, "udp"
		f.emitEvent(activity.ActivityEventInput{
			Type:     activity.EventUDPPacketForwarded,
			Severity: activity.SeverityInfo,
			RuleID:   &id,
			RuleName: &name,
			Protocol: &proto,
			Message: fmt.Sprintf("UDP packet forwarded from %s to %s:%d.",
				clientAddr.String(), f.rule.TargetHost, f.rule.TargetPort),
			Details: map[string]any{
				"fromAddress": clientAddr.IP.String(),
				"fromPort":    clientAddr.Port,
				"targetHost":  f.rule.TargetHost,
				"targetPort":  f.rule.TargetPort,
				"bytes":       bytes,
			},
		})
	}
}

// maybeEmitReturned emits udp.packet.returned throttled to once per second.
func (f *UDPForwarder) maybeEmitReturned(clientAddr *net.UDPAddr, bytes int) {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&f.lastReturnEventNs)
	if now-last >= udpEventThrottleInterval && atomic.CompareAndSwapInt64(&f.lastReturnEventNs, last, now) {
		id, name, proto := f.rule.ID, f.rule.Name, "udp"
		f.emitEvent(activity.ActivityEventInput{
			Type:     activity.EventUDPPacketReturned,
			Severity: activity.SeverityInfo,
			RuleID:   &id,
			RuleName: &name,
			Protocol: &proto,
			Message:  fmt.Sprintf("UDP reply returned to %s.", clientAddr.String()),
			Details: map[string]any{
				"toAddress": clientAddr.IP.String(),
				"toPort":    clientAddr.Port,
				"bytes":     bytes,
			},
		})
	}
}

func (f *UDPForwarder) emitPacketError(msg string) {
	id, name, proto := f.rule.ID, f.rule.Name, "udp"
	f.emitEvent(activity.ActivityEventInput{
		Type:     activity.EventUDPPacketError,
		Severity: activity.SeverityError,
		RuleID:   &id,
		RuleName: &name,
		Protocol: &proto,
		Message:  fmt.Sprintf("UDP packet error: %s", msg),
	})
}

func (f *UDPForwarder) emitSessionOpened(clientAddr *net.UDPAddr) {
	id, name, proto := f.rule.ID, f.rule.Name, "udp"
	f.emitEvent(activity.ActivityEventInput{
		Type:     activity.EventUDPSessionOpened,
		Severity: activity.SeverityInfo,
		RuleID:   &id,
		RuleName: &name,
		Protocol: &proto,
		Message:  fmt.Sprintf("UDP session opened for %s.", clientAddr.String()),
		Details: map[string]any{
			"clientAddress": clientAddr.IP.String(),
			"clientPort":    clientAddr.Port,
			"targetHost":    f.rule.TargetHost,
			"targetPort":    f.rule.TargetPort,
		},
	})
}

func (f *UDPForwarder) emitSessionClosed(clientAddr *net.UDPAddr) {
	id, name, proto := f.rule.ID, f.rule.Name, "udp"
	f.emitEvent(activity.ActivityEventInput{
		Type:     activity.EventUDPSessionClosed,
		Severity: activity.SeverityInfo,
		RuleID:   &id,
		RuleName: &name,
		Protocol: &proto,
		Message:  fmt.Sprintf("UDP session expired for %s (idle timeout).", clientAddr.String()),
		Details: map[string]any{
			"clientAddress": clientAddr.IP.String(),
			"clientPort":    clientAddr.Port,
		},
	})
}

func (f *UDPForwarder) emitEvent(input activity.ActivityEventInput) {
	if f.onEvent != nil {
		f.onEvent(input)
	}
}

func (f *UDPForwarder) udpMode() domain.UdpMode {
	if f.rule.UdpMode == nil {
		return domain.UdpModeOneWay
	}
	return *f.rule.UdpMode
}

func (f *UDPForwarder) isRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *UDPForwarder) setLastError(err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	f.lastErr = err.Error()
	f.mu.Unlock()
}

func (f *UDPForwarder) logInfo(message string, args ...any) {
	if f.log != nil {
		f.log(message, args...)
	}
}
