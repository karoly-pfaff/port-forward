package forwarders

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

type LogFunc func(message string, args ...any)

type TCPForwarder struct {
	rule     domain.ForwardRule
	log      LogFunc
	onEvent  activity.EventFunc
	registry *connections.TcpConnectionRegistry

	mu       sync.Mutex
	listener net.Listener
	running  bool
	started  string
	lastErr  string
	conns    map[net.Conn]struct{}

	activeConnections int64
	bytesIn           int64
	bytesOut          int64

	stopOnce sync.Once
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewTCPForwarder(rule domain.ForwardRule, log LogFunc, onEvent activity.EventFunc) *TCPForwarder {
	return &TCPForwarder{
		rule:    rule,
		log:     log,
		onEvent: onEvent,
		conns:   make(map[net.Conn]struct{}),
		done:    make(chan struct{}),
	}
}

// NewTCPForwarderWithRegistry creates a TCPForwarder that tracks live connections in reg.
func NewTCPForwarderWithRegistry(rule domain.ForwardRule, log LogFunc, onEvent activity.EventFunc, reg *connections.TcpConnectionRegistry) *TCPForwarder {
	f := NewTCPForwarder(rule, log, onEvent)
	f.registry = reg
	return f
}

func (f *TCPForwarder) Start() error {
	address := net.JoinHostPort(f.rule.ListenHost, fmt.Sprintf("%d", f.rule.ListenPort))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		f.setLastError(err)
		return err
	}

	f.mu.Lock()
	f.listener = listener
	f.running = true
	f.started = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	f.lastErr = ""
	f.mu.Unlock()

	f.logInfo("TCP rule started", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "listen", address)
	f.wg.Add(1)
	go f.acceptLoop()
	return nil
}

func (f *TCPForwarder) Stop() {
	f.stopOnce.Do(func() {
		f.mu.Lock()
		f.running = false
		listener := f.listener
		active := make([]net.Conn, 0, len(f.conns))
		for conn := range f.conns {
			active = append(active, conn)
		}
		f.mu.Unlock()

		if listener != nil {
			_ = listener.Close()
		}
		for _, conn := range active {
			_ = conn.Close()
		}

		waitDone := make(chan struct{})
		go func() {
			f.wg.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
			f.setLastError(errors.New("timed out waiting for TCP forwarder shutdown"))
		}

		if f.registry != nil {
			f.registry.CloseConnectionsForRule(f.rule.ID)
		}
		close(f.done)
		f.logInfo("TCP rule stopped", "ruleId", f.rule.ID, "ruleName", f.rule.Name)
	})
}

func (f *TCPForwarder) Status() domain.ForwardStatus {
	f.mu.Lock()
	running := f.running
	started := f.started
	lastErr := f.lastErr
	f.mu.Unlock()

	active := int(atomic.LoadInt64(&f.activeConnections))
	status := domain.ForwardStatus{
		RuleID:            f.rule.ID,
		Running:           running,
		ActiveConnections: &active,
		BytesIn:           atomic.LoadInt64(&f.bytesIn),
		BytesOut:          atomic.LoadInt64(&f.bytesOut),
		LastError:         lastErr,
	}
	if running {
		status.StartedAt = started
	}
	return status
}

func (f *TCPForwarder) acceptLoop() {
	defer f.wg.Done()
	for {
		clientConn, err := f.listener.Accept()
		if err != nil {
			if f.isRunning() {
				f.setLastError(err)
				f.logInfo("TCP accept error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", err)
			}
			return
		}

		f.wg.Add(1)
		go f.handleClient(clientConn)
	}
}

func (f *TCPForwarder) handleClient(clientConn net.Conn) {
	defer f.wg.Done()
	atomic.AddInt64(&f.activeConnections, 1)
	f.addConn(clientConn)

	remote := clientConn.RemoteAddr().String()
	remoteHost, remotePortStr, _ := net.SplitHostPort(remote)
	remotePort, _ := strconv.Atoi(remotePortStr)

	var connID string
	if f.registry != nil {
		connID = f.registry.OpenConnection(connections.TcpConnectionInput{
			RuleID:        f.rule.ID,
			RuleName:      f.rule.Name,
			ClientAddress: remoteHost,
			ClientPort:    remotePort,
			TargetAddress: f.rule.TargetHost,
			TargetPort:    f.rule.TargetPort,
		})
	}

	id, name, proto := f.rule.ID, f.rule.Name, "tcp"
	var loggedError int32 // 0 = no error, 1 = error already emitted (CAS guard)

	f.emitEvent(activity.ActivityEventInput{
		Type:     activity.EventTCPConnectionOpened,
		Severity: activity.SeverityInfo,
		RuleID:   &id,
		RuleName: &name,
		Protocol: &proto,
		Message:  fmt.Sprintf("TCP connection opened from %s:%d.", remoteHost, remotePort),
		Details: map[string]any{
			"remoteAddress": remoteHost,
			"remotePort":    remotePort,
			"targetHost":    f.rule.TargetHost,
			"targetPort":    f.rule.TargetPort,
		},
	})

	defer func() {
		f.removeConn(clientConn)
		_ = clientConn.Close()
		atomic.AddInt64(&f.activeConnections, -1)
		if connID != "" {
			f.registry.CloseConnection(connID)
		}
		f.logInfo("TCP connection closed", "ruleId", f.rule.ID, "ruleName", f.rule.Name)
		if atomic.LoadInt32(&loggedError) == 0 {
			f.emitEvent(activity.ActivityEventInput{
				Type:     activity.EventTCPConnectionClosed,
				Severity: activity.SeverityInfo,
				RuleID:   &id,
				RuleName: &name,
				Protocol: &proto,
				Message:  fmt.Sprintf("TCP connection closed from %s:%d.", remoteHost, remotePort),
				Details:  map[string]any{"remoteAddress": remoteHost, "remotePort": remotePort},
			})
		}
	}()

	f.logInfo("TCP connection opened", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "remote", remote)

	targetAddress := net.JoinHostPort(f.rule.TargetHost, fmt.Sprintf("%d", f.rule.TargetPort))
	targetConn, err := net.Dial("tcp", targetAddress)
	if err != nil {
		f.setLastError(err)
		f.logInfo("TCP target connection error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "target", targetAddress, "error", err)
		if atomic.CompareAndSwapInt32(&loggedError, 0, 1) {
			f.emitEvent(activity.ActivityEventInput{
				Type:     activity.EventTCPConnectionError,
				Severity: activity.SeverityError,
				RuleID:   &id,
				RuleName: &name,
				Protocol: &proto,
				Message:  fmt.Sprintf("TCP connection error: %s", err.Error()),
				Details:  map[string]any{"remoteAddress": remoteHost, "remotePort": remotePort},
			})
		}
		return
	}
	f.addConn(targetConn)
	defer func() {
		f.removeConn(targetConn)
		_ = targetConn.Close()
	}()

	// errEmitOnce fires the connection error event at most once across both copy goroutines.
	errEmitOnce := func(copyErr error) {
		if atomic.CompareAndSwapInt32(&loggedError, 0, 1) {
			f.emitEvent(activity.ActivityEventInput{
				Type:     activity.EventTCPConnectionError,
				Severity: activity.SeverityError,
				RuleID:   &id,
				RuleName: &name,
				Protocol: &proto,
				Message:  fmt.Sprintf("TCP connection error: %s", copyErr.Error()),
				Details:  map[string]any{"remoteAddress": remoteHost, "remotePort": remotePort},
			})
		}
	}

	var onBytesIn, onBytesOut func(int64)
	if connID != "" {
		reg := f.registry
		cid := connID
		onBytesIn = func(n int64) { reg.AddBytesIn(cid, n) }
		onBytesOut = func(n int64) { reg.AddBytesOut(cid, n) }
	}

	copyDone := make(chan struct{}, 2)
	go f.copyAndClose(targetConn, clientConn, &f.bytesIn, onBytesIn, copyDone, errEmitOnce)
	go f.copyAndClose(clientConn, targetConn, &f.bytesOut, onBytesOut, copyDone, errEmitOnce)

	<-copyDone
	_ = clientConn.Close()
	_ = targetConn.Close()
	<-copyDone
}

func (f *TCPForwarder) copyAndClose(dst net.Conn, src net.Conn, counter *int64, onBytes func(int64), done chan<- struct{}, onCopyError func(error)) {
	_, err := io.Copy(countingWriter{writer: dst, counter: counter, onBytes: onBytes}, src)
	if err != nil && f.isRunning() {
		f.setLastError(err)
		f.logInfo("TCP copy error", "ruleId", f.rule.ID, "ruleName", f.rule.Name, "error", err)
		if onCopyError != nil {
			onCopyError(err)
		}
	}
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}
	done <- struct{}{}
}

type countingWriter struct {
	writer  io.Writer
	counter *int64
	onBytes func(int64) // nil-safe per-connection byte callback
}

func (w countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	atomic.AddInt64(w.counter, int64(n))
	if w.onBytes != nil {
		w.onBytes(int64(n))
	}
	return n, err
}

func (f *TCPForwarder) addConn(conn net.Conn) {
	f.mu.Lock()
	f.conns[conn] = struct{}{}
	f.mu.Unlock()
}

func (f *TCPForwarder) removeConn(conn net.Conn) {
	f.mu.Lock()
	delete(f.conns, conn)
	f.mu.Unlock()
}

func (f *TCPForwarder) isRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *TCPForwarder) setLastError(err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	f.lastErr = err.Error()
	f.mu.Unlock()
}

func (f *TCPForwarder) logInfo(message string, args ...any) {
	if f.log != nil {
		f.log(message, args...)
	}
}

func (f *TCPForwarder) emitEvent(input activity.ActivityEventInput) {
	if f.onEvent != nil {
		f.onEvent(input)
	}
}
