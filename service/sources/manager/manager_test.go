package manager

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/config"
	"portier/service/sources/domain"
	"portier/service/sources/validation"
)

func TestStartsWithLoadedRules(t *testing.T) {
	manager, err := New([]domain.ForwardRule{tcpRule()})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	rules := manager.ListRules()
	if len(rules) != 1 || rules[0].ID != "tcp-1" {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestListRulesReturnsCopy(t *testing.T) {
	manager, err := New([]domain.ForwardRule{tcpRule()})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	rules := manager.ListRules()
	rules[0].Name = "mutated"

	nextRules := manager.ListRules()
	if nextRules[0].Name == "mutated" {
		t.Fatal("ListRules exposed mutable manager state")
	}
}

func TestReturnsNonRunningPlaceholderStatuses(t *testing.T) {
	multiMode := domain.UdpModeBidirectionalMulti
	udp := udpRule()
	udp.UdpMode = &multiMode
	manager, err := New([]domain.ForwardRule{tcpRule(), udp})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	statuses := manager.ListStatus()
	if len(statuses) != 2 {
		t.Fatalf("status count = %d, want 2", len(statuses))
	}
	if statuses[0].Running || statuses[0].BytesIn != 0 || statuses[0].BytesOut != 0 {
		t.Fatalf("tcp status = %#v", statuses[0])
	}
	if statuses[0].ActiveConnections == nil || *statuses[0].ActiveConnections != 0 {
		t.Fatalf("tcp active connections = %#v", statuses[0].ActiveConnections)
	}
	if statuses[1].PacketsIn == nil || *statuses[1].PacketsIn != 0 {
		t.Fatalf("udp packetsIn = %#v", statuses[1].PacketsIn)
	}
	if statuses[1].PacketsOut == nil || *statuses[1].PacketsOut != 0 {
		t.Fatalf("udp packetsOut = %#v", statuses[1].PacketsOut)
	}
	if statuses[1].ActiveUdpSessions == nil || *statuses[1].ActiveUdpSessions != 0 {
		t.Fatalf("udp active sessions = %#v", statuses[1].ActiveUdpSessions)
	}
}

func TestExportsConfigWithRules(t *testing.T) {
	manager, err := New([]domain.ForwardRule{tcpRule()})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	exported := manager.ExportConfig()
	if exported.Version != "1" {
		t.Fatalf("version = %q, want 1", exported.Version)
	}
	if exported.ExportedAt == "" {
		t.Fatal("expected exportedAt")
	}
	if len(exported.Rules) != 1 || exported.Rules[0].ID != "tcp-1" {
		t.Fatalf("rules = %#v", exported.Rules)
	}
}

func TestRejectsDuplicateBindings(t *testing.T) {
	ruleA := tcpRule()
	ruleB := tcpRule()
	ruleB.ID = "tcp-2"
	ruleB.Name = "Duplicate"

	if _, err := New([]domain.ForwardRule{ruleA, ruleB}); err == nil {
		t.Fatal("expected duplicate binding error")
	}
}

func TestCreateRule(t *testing.T) {
	manager := testManager(t, nil)
	defer manager.StopAll()
	ruleInput := tcpRule()
	ruleInput.ListenPort = freeTCPPort(t)

	rule, err := manager.CreateRule(inputForRule(ruleInput, ""))
	if err != nil {
		t.Fatalf("CreateRule returned error: %v", err)
	}
	if rule.ID == "" {
		t.Fatal("expected generated id")
	}
	if len(manager.ListRules()) != 1 {
		t.Fatalf("rules = %#v", manager.ListRules())
	}
	statuses := manager.ListStatus()
	if len(statuses) != 1 || !statuses[0].Running {
		t.Fatalf("enabled created TCP rule should start forwarding: %#v", statuses)
	}
	assertPersistedCount(t, manager, 1)
}

func TestCreateRejectsDuplicateBinding(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	duplicate := tcpRule()
	duplicate.ID = "tcp-2"

	if _, err := manager.CreateRule(inputForRule(duplicate, duplicate.ID)); err == nil {
		t.Fatal("expected duplicate conflict")
	}
}

func TestUpdateRule(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	nextPort := 48010
	patch := validation.ForwardRulePatch{ListenPort: &nextPort}

	updated, err := manager.UpdateRule("tcp-1", patch)
	if err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	if updated.ListenPort != nextPort || updated.Name != "TCP" {
		t.Fatalf("updated rule = %#v", updated)
	}
	assertPersistedCount(t, manager, 1)
}

func TestUpdateAbsentFieldsDoNotOverwrite(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	enabled := false

	updated, err := manager.UpdateRule("tcp-1", validation.ForwardRulePatch{Enabled: &enabled})
	if err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	if updated.Name != "TCP" || updated.ListenHost != "127.0.0.1" || updated.TargetHost != "127.0.0.1" {
		t.Fatalf("updated rule = %#v", updated)
	}
	if updated.Enabled {
		t.Fatal("enabled false patch was not applied")
	}
}

func TestUpdateRejectsDuplicateBinding(t *testing.T) {
	ruleB := tcpRule()
	ruleB.ID = "tcp-2"
	ruleB.Name = "Other"
	ruleB.ListenPort = 48002
	manager := testManager(t, []domain.ForwardRule{tcpRule(), ruleB})
	conflictPort := 48001

	if _, err := manager.UpdateRule("tcp-2", validation.ForwardRulePatch{ListenPort: &conflictPort}); err == nil {
		t.Fatal("expected duplicate conflict")
	}
}

func TestDeleteRule(t *testing.T) {
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()
	if _, err := manager.StartRule("tcp-1"); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}

	if err := manager.DeleteRule("tcp-1"); err != nil {
		t.Fatalf("DeleteRule returned error: %v", err)
	}
	if len(manager.ListRules()) != 0 {
		t.Fatalf("rules = %#v", manager.ListRules())
	}
	assertPersistedCount(t, manager, 0)
}

func TestReorderRules(t *testing.T) {
	ruleB := tcpRule()
	ruleB.ID = "tcp-2"
	ruleB.Name = "Other"
	ruleB.ListenPort = 48002
	manager := testManager(t, []domain.ForwardRule{tcpRule(), ruleB})

	if err := manager.ReorderRules([]string{"tcp-2", "tcp-1"}); err != nil {
		t.Fatalf("ReorderRules returned error: %v", err)
	}
	rules := manager.ListRules()
	if rules[0].ID != "tcp-2" || rules[1].ID != "tcp-1" {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestImportReplace(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	defer manager.StopAll()
	udp := udpRule()
	udp.Enabled = true
	udp.ListenPort = freeUDPPort(t)

	result, err := manager.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{udp}}, "replace")
	if err != nil {
		t.Fatalf("ImportConfig returned error: %v", err)
	}
	if result.Imported != 1 || len(result.Errors) != 0 {
		t.Fatalf("result = %#v", result)
	}
	rules := manager.ListRules()
	if len(rules) != 1 || rules[0].ID != "udp-1" {
		t.Fatalf("rules = %#v", rules)
	}
	statuses := manager.ListStatus()
	if len(statuses) != 1 || !statuses[0].Running || statuses[0].StartedAt == "" {
		t.Fatalf("enabled imported rule should be running: %#v", statuses)
	}
}

func TestImportMerge(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	defer manager.StopAll()
	udp := udpRule()
	udp.Enabled = true
	udp.ListenPort = freeUDPPort(t)

	result, err := manager.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{udp}}, "merge")
	if err != nil {
		t.Fatalf("ImportConfig returned error: %v", err)
	}
	if result.Imported != 1 || len(manager.ListRules()) != 2 {
		t.Fatalf("result = %#v rules=%#v", result, manager.ListRules())
	}
	statuses := manager.ListStatus()
	if !statusByRuleID(statuses, "udp-1").Running {
		t.Fatalf("enabled merged rule should be running: %#v", statuses)
	}
}

func TestImportRejectsInvalidConfigWithoutPartialApply(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	invalid := udpRule()
	invalid.ListenPort = 0

	result, err := manager.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{invalid}}, "merge")
	if err != nil {
		t.Fatalf("ImportConfig returned error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected import errors")
	}
	rules := manager.ListRules()
	if len(rules) != 1 || rules[0].ID != "tcp-1" {
		t.Fatalf("partial apply occurred: %#v", rules)
	}
}

func TestImportMergeRejectsConflictWithoutPartialApply(t *testing.T) {
	manager := testManager(t, []domain.ForwardRule{tcpRule()})
	conflict := udpRule()
	conflict.ID = "udp-conflict"
	conflict.Protocol = domain.ProtocolTCP
	conflict.ListenPort = 48001
	conflict.UdpMode = nil

	result, err := manager.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{conflict}}, "merge")
	if err != nil {
		t.Fatalf("ImportConfig returned error: %v", err)
	}
	if len(result.Errors) == 0 || result.Imported != 0 {
		t.Fatalf("expected conflict errors, got %#v", result)
	}
	rules := manager.ListRules()
	if len(rules) != 1 || rules[0].ID != "tcp-1" {
		t.Fatalf("partial apply occurred: %#v", rules)
	}
}

func TestStartStopUDPRule(t *testing.T) {
	rule := udpRule()
	rule.ListenPort = freeUDPPort(t)
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	status, err := manager.StartRule("udp-1")
	if err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if !status.Running || status.StartedAt == "" {
		t.Fatalf("status after start = %#v", status)
	}
	if status.PacketsIn == nil {
		t.Fatal("PacketsIn should be non-nil for UDP rule")
	}

	status, err = manager.StopRule("udp-1")
	if err != nil {
		t.Fatalf("StopRule returned error: %v", err)
	}
	if status.Running || status.StartedAt != "" {
		t.Fatalf("status after stop = %#v", status)
	}
}

func TestStatusReflectsUDPRunningState(t *testing.T) {
	rule := udpRule()
	rule.ListenPort = freeUDPPort(t)
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	if _, err := manager.StartRule("udp-1"); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}

	statuses := manager.ListStatus()
	if len(statuses) != 1 || !statuses[0].Running {
		t.Fatalf("statuses = %#v", statuses)
	}
}

func TestTCPForwardingAndStatus(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "one")
	defer stopTarget()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	status, err := manager.StartRule(rule.ID)
	if err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if !status.Running || status.ActiveConnections == nil || *status.ActiveConnections != 0 {
		t.Fatalf("start status = %#v", status)
	}

	conn := dialTCP(t, rule.ListenPort)
	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write through forwarder: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read echo through forwarder: %v", err)
	}
	if line != "one:hello\n" {
		t.Fatalf("line = %q", line)
	}

	// Wait for both byte counters to be updated. The increment happens after
	// the write syscall, so the client can receive data before bytesOut is
	// recorded — poll to avoid a data race in this assertion.
	waitFor(t, func() bool {
		s := statusByRuleID(manager.ListStatus(), rule.ID)
		return s.BytesIn > 0 && s.BytesOut > 0
	})
	statuses := manager.ListStatus()
	status = statusByRuleID(statuses, rule.ID)
	if status.ActiveConnections == nil || *status.ActiveConnections != 1 {
		t.Fatalf("active connections while open = %#v", status)
	}
	if status.BytesIn == 0 || status.BytesOut == 0 {
		t.Fatalf("bytes were not counted: %#v", status)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitFor(t, func() bool {
		status := statusByRuleID(manager.ListStatus(), rule.ID)
		return status.ActiveConnections != nil && *status.ActiveConnections == 0
	})
}

func TestTCPStopClosesListener(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "stop")
	defer stopTarget()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	manager := testManager(t, []domain.ForwardRule{rule})

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if _, err := manager.StopRule(rule.ID); err != nil {
		t.Fatalf("StopRule returned error: %v", err)
	}
	if status := statusByRuleID(manager.ListStatus(), rule.ID); status.Running {
		t.Fatalf("status after stop = %#v", status)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", rule.ListenPort)), 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected stopped listener to reject new connections")
	}
}

func TestTCPUnreachableTargetRecordsLastError(t *testing.T) {
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = freeTCPPort(t)
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	conn := dialTCP(t, rule.ListenPort)
	_, _ = conn.Write([]byte("hello\n"))
	_ = conn.Close()

	waitFor(t, func() bool {
		return statusByRuleID(manager.ListStatus(), rule.ID).LastError != ""
	})
}

func TestTCPStartBindFailureReturnsErrorAndLastError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listen port: %v", err)
	}
	defer listener.Close()

	rule := tcpRule()
	rule.ListenPort = listener.Addr().(*net.TCPAddr).Port
	manager := testManager(t, []domain.ForwardRule{rule})

	status, err := manager.StartRule(rule.ID)
	if err == nil {
		t.Fatal("expected bind failure")
	}
	if status.Running || status.LastError == "" {
		t.Fatalf("status = %#v", status)
	}
}

func TestTCPUpdateNameDoesNotRestartListener(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "name")
	defer stopTarget()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()
	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	startedAt := statusByRuleID(manager.ListStatus(), rule.ID).StartedAt

	name := "Renamed TCP"
	if _, err := manager.UpdateRule(rule.ID, validation.ForwardRulePatch{Name: &name}); err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	status := statusByRuleID(manager.ListStatus(), rule.ID)
	if !status.Running || status.StartedAt != startedAt {
		t.Fatalf("non-forwarding update restarted listener: before=%q after=%#v", startedAt, status)
	}
}

func TestTCPUpdateForwardingFieldRestartsListener(t *testing.T) {
	firstPort, stopFirst := startEchoServer(t, "first")
	defer stopFirst()
	secondPort, stopSecond := startEchoServer(t, "second")
	defer stopSecond()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = firstPort
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()
	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}

	if got := requestThroughForwarder(t, rule.ListenPort, "ping"); got != "first:ping\n" {
		t.Fatalf("before update response = %q", got)
	}
	if _, err := manager.UpdateRule(rule.ID, validation.ForwardRulePatch{TargetPort: &secondPort}); err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	if got := requestThroughForwarder(t, rule.ListenPort, "ping"); got != "second:ping\n" {
		t.Fatalf("after update response = %q", got)
	}
}

func TestTCPDeleteStopsListener(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "delete")
	defer stopTarget()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	manager := testManager(t, []domain.ForwardRule{rule})
	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if err := manager.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule returned error: %v", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", rule.ListenPort)), 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected deleted rule listener to reject new connections")
	}
}

func TestStartEnabledAutostartsTCPRule(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "auto")
	defer stopTarget()

	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	rule.Enabled = true
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	started, err := manager.StartEnabled()
	if err != nil {
		t.Fatalf("StartEnabled returned error: %v", err)
	}
	if started != 1 {
		t.Fatalf("started = %d, want 1", started)
	}
	if got := requestThroughForwarder(t, rule.ListenPort, "boot"); got != "auto:boot\n" {
		t.Fatalf("autostart response = %q", got)
	}
}

func TestForwardingFieldsChanged(t *testing.T) {
	base := tcpRule()
	same := base
	if ForwardingFieldsChanged(base, same) {
		t.Fatal("expected same rules to be unchanged")
	}

	changed := base
	changed.TargetPort = 4000
	if !ForwardingFieldsChanged(base, changed) {
		t.Fatal("expected targetPort change to be forwarding-affecting")
	}

	nameOnly := base
	nameOnly.Name = "Renamed"
	if ForwardingFieldsChanged(base, nameOnly) {
		t.Fatal("name change should not be forwarding-affecting")
	}

	enabledOnly := base
	enabledOnly.Enabled = false
	if ForwardingFieldsChanged(base, enabledOnly) {
		t.Fatal("enabled change should not be forwarding-affecting")
	}

	protocolChanged := base
	protocolChanged.Protocol = domain.ProtocolUDP
	mode := domain.UdpModeOneWay
	protocolChanged.UdpMode = &mode
	if !ForwardingFieldsChanged(base, protocolChanged) {
		t.Fatal("expected protocol change to be forwarding-affecting")
	}

	udpBase := udpRule()
	udpChanged := udpBase
	multiMode := domain.UdpModeBidirectionalMulti
	udpChanged.UdpMode = &multiMode
	if !ForwardingFieldsChanged(udpBase, udpChanged) {
		t.Fatal("expected udpMode change to be forwarding-affecting")
	}
}

func TestUDPForwardingAndStatus(t *testing.T) {
	targetPort, stopTarget := startUDPEchoServer(t, "echo")
	defer stopTarget()

	mode := domain.UdpModeBidirectionalLast
	rule := domain.ForwardRule{
		ID:         "udp-fwd",
		Name:       "UDP Fwd",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	status, err := manager.StartRule(rule.ID)
	if err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if !status.Running || status.PacketsIn == nil {
		t.Fatalf("start status = %#v", status)
	}

	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: rule.ListenPort})
	if err != nil {
		t.Fatalf("dial UDP: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if string(buf[:n]) != "echo:ping" {
		t.Fatalf("reply = %q, want echo:ping", string(buf[:n]))
	}

	waitFor(t, func() bool {
		s := statusByRuleID(manager.ListStatus(), rule.ID)
		return s.PacketsIn != nil && *s.PacketsIn >= 1
	})
	s := statusByRuleID(manager.ListStatus(), rule.ID)
	if *s.PacketsIn == 0 || s.BytesIn == 0 {
		t.Fatalf("inbound stats not updated: %#v", s)
	}
}

func TestUDPStopClosesSocket(t *testing.T) {
	mode := domain.UdpModeOneWay
	rule := domain.ForwardRule{
		ID:         "udp-stop",
		Name:       "UDP Stop",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: freeUDPPort(t),
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if _, err := manager.StopRule(rule.ID); err != nil {
		t.Fatalf("StopRule returned error: %v", err)
	}

	s := statusByRuleID(manager.ListStatus(), rule.ID)
	if s.Running {
		t.Fatalf("status after stop = %#v", s)
	}
}

func TestUDPStartBindFailureReturnsErrorAndLastError(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	reserved, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	defer reserved.Close()
	port := reserved.LocalAddr().(*net.UDPAddr).Port

	mode := domain.UdpModeOneWay
	rule := domain.ForwardRule{
		ID:         "udp-fail",
		Name:       "UDP Fail",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: port,
		TargetHost: "127.0.0.1",
		TargetPort: 9000,
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})

	status, err := manager.StartRule(rule.ID)
	if err == nil {
		t.Fatal("expected bind failure")
	}
	if status.Running || status.LastError == "" {
		t.Fatalf("status = %#v", status)
	}
}

func TestUDPUpdateNameDoesNotRestartListener(t *testing.T) {
	mode := domain.UdpModeOneWay
	rule := domain.ForwardRule{
		ID:         "udp-name",
		Name:       "UDP Name",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: freeUDPPort(t),
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	startedAt := statusByRuleID(manager.ListStatus(), rule.ID).StartedAt

	name := "Renamed UDP"
	if _, err := manager.UpdateRule(rule.ID, validation.ForwardRulePatch{Name: &name}); err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	s := statusByRuleID(manager.ListStatus(), rule.ID)
	if !s.Running || s.StartedAt != startedAt {
		t.Fatalf("name update restarted listener: before=%q after=%#v", startedAt, s)
	}
}

func TestUDPUpdateForwardingFieldRestartsForwarder(t *testing.T) {
	targetPort, stopTarget := startUDPEchoServer(t, "first")
	defer stopTarget()

	mode := domain.UdpModeBidirectionalLast
	rule := domain.ForwardRule{
		ID:         "udp-restart",
		Name:       "UDP Restart",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	startedAt := statusByRuleID(manager.ListStatus(), rule.ID).StartedAt

	secondTarget, stopSecond := startUDPEchoServer(t, "second")
	defer stopSecond()

	if _, err := manager.UpdateRule(rule.ID, validation.ForwardRulePatch{TargetPort: &secondTarget}); err != nil {
		t.Fatalf("UpdateRule returned error: %v", err)
	}
	s := statusByRuleID(manager.ListStatus(), rule.ID)
	if !s.Running {
		t.Fatal("rule should still be running after restart")
	}
	if s.StartedAt == startedAt {
		t.Fatal("startedAt should change after forwarder restart")
	}
}

func TestUDPDeleteStopsForwarder(t *testing.T) {
	mode := domain.UdpModeOneWay
	rule := domain.ForwardRule{
		ID:         "udp-del",
		Name:       "UDP Del",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: freeUDPPort(t),
		Enabled:    false,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})

	if _, err := manager.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule returned error: %v", err)
	}
	if err := manager.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule returned error: %v", err)
	}
	if len(manager.ListRules()) != 0 {
		t.Fatal("rule should be deleted")
	}
}

func TestStartEnabledAutostartsUDPRule(t *testing.T) {
	targetPort, stopTarget := startUDPEchoServer(t, "auto")
	defer stopTarget()

	mode := domain.UdpModeBidirectionalLast
	rule := domain.ForwardRule{
		ID:         "udp-auto",
		Name:       "UDP Auto",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: freeUDPPort(t),
		TargetHost: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
		UdpMode:    &mode,
	}
	manager := testManager(t, []domain.ForwardRule{rule})
	defer manager.StopAll()

	started, err := manager.StartEnabled()
	if err != nil {
		t.Fatalf("StartEnabled returned error: %v", err)
	}
	if started != 1 {
		t.Fatalf("started = %d, want 1", started)
	}

	s := statusByRuleID(manager.ListStatus(), rule.ID)
	if !s.Running || s.StartedAt == "" {
		t.Fatalf("autostarted UDP rule not running: %#v", s)
	}

	// Verify packets actually flow through the forwarder.
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: rule.ListenPort})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("boot")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if string(buf[:n]) != "auto:boot" {
		t.Fatalf("reply = %q, want auto:boot", string(buf[:n]))
	}
}

func TestDuplicateUDPListenBindingRejected(t *testing.T) {
	mode := domain.UdpModeOneWay
	port := freeUDPPort(t)
	ruleA := domain.ForwardRule{
		ID: "udp-a", Name: "A", Protocol: domain.ProtocolUDP,
		ListenHost: "127.0.0.1", ListenPort: port,
		TargetHost: "127.0.0.1", TargetPort: 9000,
		Enabled: false, UdpMode: &mode,
	}
	ruleB := ruleA
	ruleB.ID = "udp-b"
	ruleB.Name = "B"

	if _, err := New([]domain.ForwardRule{ruleA, ruleB}); err == nil {
		t.Fatal("expected duplicate binding error")
	}
}

func TestCreateRuleRecordsActivity(t *testing.T) {
	m, store := testManagerWithActivity(t, nil)
	input := tcpRule()
	input.ListenPort = freeTCPPort(t)
	input.Enabled = false
	if _, err := m.CreateRule(inputForRule(input, "")); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	assertActivityType(t, store, activity.EventRuleCreated)
}

func TestUpdateRuleRecordsActivity(t *testing.T) {
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.Enabled = false
	m, store := testManagerWithActivity(t, []domain.ForwardRule{rule})
	newName := "Renamed"
	if _, err := m.UpdateRule(rule.ID, validation.ForwardRulePatch{Name: &newName}); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	assertActivityType(t, store, activity.EventRuleUpdated)
}

func TestDeleteRuleRecordsActivity(t *testing.T) {
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.Enabled = false
	m, store := testManagerWithActivity(t, []domain.ForwardRule{rule})
	if err := m.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	assertActivityType(t, store, activity.EventRuleDeleted)
}

func TestStartRuleRecordsActivity(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "act")
	defer stopTarget()
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	rule.Enabled = false
	m, store := testManagerWithActivity(t, []domain.ForwardRule{rule})
	defer m.StopAll()
	if _, err := m.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule: %v", err)
	}
	assertActivityType(t, store, activity.EventRuleStarted)
}

func TestStopRuleRecordsActivity(t *testing.T) {
	targetPort, stopTarget := startEchoServer(t, "stop-act")
	defer stopTarget()
	rule := tcpRule()
	rule.ListenPort = freeTCPPort(t)
	rule.TargetPort = targetPort
	rule.Enabled = false
	m, store := testManagerWithActivity(t, []domain.ForwardRule{rule})
	if _, err := m.StartRule(rule.ID); err != nil {
		t.Fatalf("StartRule: %v", err)
	}
	if _, err := m.StopRule(rule.ID); err != nil {
		t.Fatalf("StopRule: %v", err)
	}
	assertActivityType(t, store, activity.EventRuleStopped)
}

func TestFailedStartRecordsRuleErrorActivity(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()
	rule := tcpRule()
	rule.ListenPort = listener.Addr().(*net.TCPAddr).Port
	rule.Enabled = false
	m, store := testManagerWithActivity(t, []domain.ForwardRule{rule})
	if _, err := m.StartRule(rule.ID); err == nil {
		t.Fatal("expected bind failure")
	}
	assertActivityType(t, store, activity.EventRuleError)
}

func TestImportConfigRecordsActivity(t *testing.T) {
	m, store := testManagerWithActivity(t, nil)
	defer m.StopAll()
	rule := tcpRule()
	rule.Enabled = false
	result, err := m.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{rule}}, "replace")
	if err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("import errors: %v", result.Errors)
	}
	assertActivityType(t, store, activity.EventConfigImported)
}

func TestSetStartLogger(t *testing.T) {
	m, err := New([]domain.ForwardRule{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	called := false
	m.SetStartLogger(func(rule domain.ForwardRule) { called = true })
	if m.onStartLog == nil {
		t.Fatal("onStartLog should be set")
	}
	m.onStartLog(tcpRule())
	if !called {
		t.Fatal("SetStartLogger did not wire the function")
	}
}

func TestSetEventLogger(t *testing.T) {
	m, err := New([]domain.ForwardRule{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	called := false
	m.SetEventLogger(func(message string, args ...any) { called = true })
	if m.onEventLog == nil {
		t.Fatal("onEventLog should be set")
	}
	m.onEventLog("test message")
	if !called {
		t.Fatal("SetEventLogger did not wire the function")
	}
}

func TestListActivityNoStore(t *testing.T) {
	m, err := New([]domain.ForwardRule{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events := m.ListActivity(activity.ListParams{})
	if len(events) != 0 {
		t.Fatalf("expected empty slice, got %d events", len(events))
	}
}

func TestClearActivityNoStore(t *testing.T) {
	m, err := New([]domain.ForwardRule{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.ClearActivity()
}

func TestImportConfigInvalidVersion(t *testing.T) {
	m := testManager(t, nil)
	_, err := m.ImportConfig(domain.ExportedConfig{Version: "2", Rules: nil}, "replace")
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestImportConfigInvalidMode(t *testing.T) {
	m := testManager(t, nil)
	_, err := m.ImportConfig(domain.ExportedConfig{Version: "1", Rules: nil}, "upsert")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestImportConfigRejectsDuplicateBindingsInImport(t *testing.T) {
	m := testManager(t, nil)
	rule := tcpRule()
	ruleB := tcpRule()
	ruleB.ID = "tcp-2"
	ruleB.Name = "Duplicate"
	result, err := m.ImportConfig(domain.ExportedConfig{Version: "1", Rules: []domain.ForwardRule{rule, ruleB}}, "replace")
	if err != nil {
		t.Fatalf("expected ImportResult with errors, got Go error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected import errors for duplicate bindings")
	}
}

func tcpRule() domain.ForwardRule {
	return domain.ForwardRule{
		ID:         "tcp-1",
		Name:       "TCP",
		Protocol:   domain.ProtocolTCP,
		ListenHost: "127.0.0.1",
		ListenPort: 48001,
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
		Enabled:    true,
	}
}

func udpRule() domain.ForwardRule {
	mode := domain.UdpModeOneWay
	return domain.ForwardRule{
		ID:         "udp-1",
		Name:       "UDP",
		Protocol:   domain.ProtocolUDP,
		ListenHost: "127.0.0.1",
		ListenPort: 48002,
		TargetHost: "127.0.0.1",
		TargetPort: 9000,
		Enabled:    false,
		UdpMode:    &mode,
	}
}

func testManager(t *testing.T, rules []domain.ForwardRule) *Manager {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "nested", "forwards.json"))
	manager, err := NewWithStore(&store, rules)
	if err != nil {
		t.Fatalf("NewWithStore returned error: %v", err)
	}
	return manager
}

func testManagerWithActivity(t *testing.T, rules []domain.ForwardRule) (*Manager, *activity.Store) {
	t.Helper()
	m := testManager(t, rules)
	store := &activity.Store{}
	m.SetActivityStore(store)
	return m, store
}

func assertActivityType(t *testing.T, store *activity.Store, eventType string) {
	t.Helper()
	events := store.List(activity.ListParams{})
	for _, e := range events {
		if e.Type == eventType {
			return
		}
	}
	t.Fatalf("expected event type %q not found; got: %#v", eventType, events)
}

func inputForRule(rule domain.ForwardRule, id string) validation.ForwardRuleInput {
	input := validation.InputFromRule(rule)
	if id == "" {
		input.ID = nil
	}
	return input
}

func assertPersistedCount(t *testing.T, manager *Manager, want int) {
	t.Helper()
	rules, err := manager.store.Load()
	if err != nil {
		t.Fatalf("load persisted rules: %v", err)
	}
	if len(rules) != want {
		t.Fatalf("persisted rule count = %d, want %d", len(rules), want)
	}
}

func statusByRuleID(statuses []domain.ForwardStatus, ruleID string) domain.ForwardStatus {
	for _, status := range statuses {
		if status.RuleID == ruleID {
			return status
		}
	}
	return domain.ForwardStatus{}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral UDP port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

func startUDPEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve UDP echo addr: %v", err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("start UDP echo server: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	go func() {
		buf := make([]byte, 65536)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			response := append([]byte(prefix+":"), buf[:n]...)
			_, _ = conn.WriteToUDP(response, remote)
		}
	}()
	return port, func() { conn.Close() }
}

func startEchoServer(t *testing.T, prefix string) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start echo server: %v", err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go func() {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							return
						}
						return
					}
					_, _ = conn.Write([]byte(prefix + ":" + line))
				}
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, func() {
		close(stop)
		_ = listener.Close()
	}
}

func dialTCP(t *testing.T, port int) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port)), time.Second)
	if err != nil {
		t.Fatalf("dial tcp forwarder: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	return conn
}

func requestThroughForwarder(t *testing.T, port int, message string) string {
	t.Helper()
	conn := dialTCP(t, port)
	defer conn.Close()
	if _, err := conn.Write([]byte(message + "\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return line
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
