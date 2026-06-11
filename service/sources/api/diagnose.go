package api

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"portier/service/sources/advisory"
	"portier/service/sources/domain"
)

const diagTimeout = 2 * time.Second

func diagnoseRule(rule domain.ForwardRule, isRunning bool) domain.RuleDiagnosticsResult {
	// Each phase appends exactly one check; order here is the response check order.
	checks := make([]domain.DiagnosticCheck, 0, 10)

	checks = append(checks, checkListenHost(rule))
	checks = append(checks, checkLanExposure(rule))
	checks = append(checks, checkPrivilegedPort(rule))
	checks = append(checks, checkCommonPort(rule))
	checks = append(checks, checkListenBind(rule, isRunning))

	targetHostCheck := checkTargetHost(rule)
	checks = append(checks, targetHostCheck)
	targetResolved := targetHostCheck.Status == domain.DiagnosticPass
	checks = append(checks, checkTargetConnect(rule, targetResolved))

	if rule.Protocol == domain.ProtocolUDP {
		checks = append(checks, checkUdpMode(rule))
	}

	summary := buildSummary(checks)

	return domain.RuleDiagnosticsResult{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Protocol:    rule.Protocol,
		Summary:     summary,
		Checks:      checks,
		DiagnosedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// checkListenHost flags whether the rule binds all interfaces (0.0.0.0) or a
// specific address.
func checkListenHost(rule domain.ForwardRule) domain.DiagnosticCheck {
	if rule.ListenHost == "0.0.0.0" {
		return domain.DiagnosticCheck{
			ID:      "listen-host",
			Label:   "Listen address",
			Status:  domain.DiagnosticWarn,
			Message: "Listening on 0.0.0.0 exposes this port on all interfaces.",
			Details: map[string]any{"listenHost": rule.ListenHost},
		}
	}
	return domain.DiagnosticCheck{
		ID:      "listen-host",
		Label:   "Listen address",
		Status:  domain.DiagnosticPass,
		Message: fmt.Sprintf("Listen address %s is specific.", rule.ListenHost),
		Details: map[string]any{"listenHost": rule.ListenHost},
	}
}

// checkLanExposure reports whether the rule is reachable from other LAN devices.
func checkLanExposure(rule domain.ForwardRule) domain.DiagnosticCheck {
	if rule.ListenHost == "0.0.0.0" {
		return domain.DiagnosticCheck{
			ID:      "lan-exposure",
			Label:   "LAN exposure",
			Status:  domain.DiagnosticWarn,
			Message: "Rule is accessible to other devices on the network because it listens on 0.0.0.0.",
			Details: map[string]any{"listenHost": rule.ListenHost},
		}
	}
	return domain.DiagnosticCheck{
		ID:      "lan-exposure",
		Label:   "LAN exposure",
		Status:  domain.DiagnosticPass,
		Message: "Rule is bound to a specific interface and not exposed on the LAN.",
		Details: map[string]any{"listenHost": rule.ListenHost},
	}
}

// checkPrivilegedPort flags listen ports below 1024 that may require elevation.
func checkPrivilegedPort(rule domain.ForwardRule) domain.DiagnosticCheck {
	if rule.ListenPort < 1024 {
		return domain.DiagnosticCheck{
			ID:      "privileged-port",
			Label:   "Privileged port",
			Status:  domain.DiagnosticWarn,
			Message: fmt.Sprintf("Port %d is privileged and may require elevated permissions to bind.", rule.ListenPort),
			Details: map[string]any{"listenPort": rule.ListenPort},
		}
	}
	return domain.DiagnosticCheck{
		ID:      "privileged-port",
		Label:   "Privileged port",
		Status:  domain.DiagnosticPass,
		Message: fmt.Sprintf("Port %d does not require elevated permissions.", rule.ListenPort),
		Details: map[string]any{"listenPort": rule.ListenPort},
	}
}

// checkCommonPort flags listen ports that collide with a well-known service.
func checkCommonPort(rule domain.ForwardRule) domain.DiagnosticCheck {
	if info, ok := advisory.GetCommonPortInfo(rule.ListenPort); ok {
		return domain.DiagnosticCheck{
			ID:      "common-port",
			Label:   "Common port",
			Status:  domain.DiagnosticWarn,
			Message: fmt.Sprintf("Port %d is commonly used by %s.", rule.ListenPort, info.Label),
			Details: map[string]any{"listenPort": rule.ListenPort, "service": info.Label},
		}
	}
	return domain.DiagnosticCheck{
		ID:      "common-port",
		Label:   "Common port",
		Status:  domain.DiagnosticPass,
		Message: fmt.Sprintf("Port %d is not a well-known service port.", rule.ListenPort),
		Details: map[string]any{"listenPort": rule.ListenPort},
	}
}

// checkListenBind attempts to bind the listen socket, unless the rule is already
// running (in which case Portier already owns the port).
func checkListenBind(rule domain.ForwardRule, isRunning bool) domain.DiagnosticCheck {
	if isRunning {
		return domain.DiagnosticCheck{
			ID:      "listen-bind",
			Label:   "Listen bind",
			Status:  domain.DiagnosticPass,
			Message: "Rule is currently running; the listen port is already owned by Portier.",
			Details: map[string]any{"listenHost": rule.ListenHost, "listenPort": rule.ListenPort, "ruleRunning": true},
		}
	}
	bindStatus, bindMsg := tryBind(rule)
	return domain.DiagnosticCheck{
		ID:      "listen-bind",
		Label:   "Listen bind",
		Status:  bindStatus,
		Message: bindMsg,
		Details: map[string]any{"listenHost": rule.ListenHost, "listenPort": rule.ListenPort, "ruleRunning": false},
	}
}

// checkTargetHost resolves the target hostname via DNS. A pass status means the
// host resolved (used to gate the target-connect check).
func checkTargetHost(rule domain.ForwardRule) domain.DiagnosticCheck {
	addrs, err := net.LookupHost(rule.TargetHost)
	if err != nil || len(addrs) == 0 {
		return domain.DiagnosticCheck{
			ID:      "target-host",
			Label:   "Target hostname",
			Status:  domain.DiagnosticFail,
			Message: fmt.Sprintf("Target host %s could not be resolved.", rule.TargetHost),
			Details: map[string]any{"targetHost": rule.TargetHost},
		}
	}
	return domain.DiagnosticCheck{
		ID:      "target-host",
		Label:   "Target hostname",
		Status:  domain.DiagnosticPass,
		Message: fmt.Sprintf("Target host %s resolves successfully.", rule.TargetHost),
		Details: map[string]any{"targetHost": rule.TargetHost},
	}
}

// checkTargetConnect attempts a TCP connection to the target. UDP is always
// skipped (reachability is not verifiable without a protocol response), and TCP
// is skipped when the target host did not resolve.
func checkTargetConnect(rule domain.ForwardRule, targetResolved bool) domain.DiagnosticCheck {
	if rule.Protocol != domain.ProtocolTCP {
		return domain.DiagnosticCheck{
			ID:      "target-connect",
			Label:   "Target connection",
			Status:  domain.DiagnosticSkip,
			Message: "UDP reachability cannot be verified without a protocol-specific response from the target.",
			Details: map[string]any{"targetHost": rule.TargetHost, "targetPort": rule.TargetPort, "protocol": "udp"},
		}
	}
	if !targetResolved {
		return domain.DiagnosticCheck{
			ID:      "target-connect",
			Label:   "Target connection",
			Status:  domain.DiagnosticSkip,
			Message: "Skipped because target host resolution failed.",
			Details: map[string]any{"targetHost": rule.TargetHost, "targetPort": rule.TargetPort},
		}
	}
	connectStatus, connectMsg := tryTCPConnect(rule.TargetHost, rule.TargetPort)
	return domain.DiagnosticCheck{
		ID:      "target-connect",
		Label:   "Target connection",
		Status:  connectStatus,
		Message: connectMsg,
		Details: map[string]any{"targetHost": rule.TargetHost, "targetPort": rule.TargetPort},
	}
}

// checkUdpMode describes the configured UDP forwarding mode and warns on the
// last-client mode's single-client limitation. Only called for UDP rules.
func checkUdpMode(rule domain.ForwardRule) domain.DiagnosticCheck {
	mode := domain.UdpModeOneWay
	if rule.UdpMode != nil {
		mode = *rule.UdpMode
	}
	if mode == domain.UdpModeBidirectionalLast {
		return domain.DiagnosticCheck{
			ID:     "udp-mode",
			Label:  "UDP mode",
			Status: domain.DiagnosticWarn,
			Message: "bidirectional-last-client sends replies to the most recently seen client only. " +
				"Suitable for single-client use cases; not reliable for concurrent clients.",
			Details: map[string]any{"udpMode": string(mode)},
		}
	}
	var modeMsg string
	if mode == domain.UdpModeOneWay {
		modeMsg = "one-way: packets flow from clients to the target only; replies are not forwarded."
	} else {
		modeMsg = "bidirectional-multi-client: each client gets its own reply path."
	}
	return domain.DiagnosticCheck{
		ID:      "udp-mode",
		Label:   "UDP mode",
		Status:  domain.DiagnosticPass,
		Message: modeMsg,
		Details: map[string]any{"udpMode": string(mode)},
	}
}

func buildSummary(checks []domain.DiagnosticCheck) domain.DiagnosticSummary {
	var failed, warned []string
	for _, c := range checks {
		if c.Status == domain.DiagnosticFail {
			failed = append(failed, c.Label)
		} else if c.Status == domain.DiagnosticWarn {
			warned = append(warned, c.Label)
		}
	}
	if len(failed) > 0 {
		return domain.DiagnosticSummary{
			Status:  "fail",
			Message: fmt.Sprintf("%d check(s) failed: %s.", len(failed), strings.Join(failed, ", ")),
		}
	}
	if len(warned) > 0 {
		return domain.DiagnosticSummary{
			Status:  "warn",
			Message: fmt.Sprintf("%d check(s) need attention: %s.", len(warned), strings.Join(warned, ", ")),
		}
	}
	return domain.DiagnosticSummary{Status: "pass", Message: "All checks passed."}
}

func tryBind(rule domain.ForwardRule) (domain.DiagnosticStatus, string) {
	addr := net.JoinHostPort(rule.ListenHost, strconv.Itoa(rule.ListenPort))
	if rule.Protocol == domain.ProtocolTCP {
		return tryTCPBind(addr)
	}
	return tryUDPBind(addr)
}

func tryTCPBind(addr string) (domain.DiagnosticStatus, string) {
	ln, err := net.Listen("tcp4", addr)
	if err != nil {
		return domain.DiagnosticFail, fmt.Sprintf("TCP bind to %s failed: %v.", addr, err)
	}
	_ = ln.Close()
	return domain.DiagnosticPass, fmt.Sprintf("TCP bind to %s succeeded.", addr)
}

func tryUDPBind(addr string) (domain.DiagnosticStatus, string) {
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return domain.DiagnosticFail, fmt.Sprintf("UDP bind to %s failed: %v.", addr, err)
	}
	_ = pc.Close()
	return domain.DiagnosticPass, fmt.Sprintf("UDP bind to %s succeeded.", addr)
}

func tryTCPConnect(host string, port int) (domain.DiagnosticStatus, string) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp4", addr, diagTimeout)
	if err != nil {
		return domain.DiagnosticFail, fmt.Sprintf("TCP connection to %s failed: %v.", addr, err)
	}
	_ = conn.Close()
	return domain.DiagnosticPass, fmt.Sprintf("TCP connection to %s succeeded.", addr)
}
