package connections

import (
	"fmt"
	"sync"
	"time"
)

const (
	// UDPSessionIdleDuration is the threshold after which a session is considered idle.
	UDPSessionIdleDuration = 30 * time.Second
	// UDPSessionExpireDuration is the threshold after which a session is pruned from the registry.
	UDPSessionExpireDuration = 5 * time.Minute
)

// UdpSessionInput holds the data needed to open or touch a tracked UDP session.
type UdpSessionInput struct {
	RuleID        string
	RuleName      string
	Mode          string // "one-way", "bidirectional-last-client", "bidirectional-multi-client"
	ClientAddress string
	ClientPort    int
	TargetAddress string
	TargetPort    int
}

// UdpSessionInfo is a serializable snapshot of one UDP session.
type UdpSessionInfo struct {
	ID            string `json:"id"`
	RuleID        string `json:"ruleId"`
	RuleName      string `json:"ruleName"`
	Protocol      string `json:"protocol"`
	Mode          string `json:"mode"`
	ClientAddress string `json:"clientAddress"`
	ClientPort    int    `json:"clientPort"`
	TargetAddress string `json:"targetAddress"`
	TargetPort    int    `json:"targetPort"`
	StartedAt     string `json:"startedAt"`
	LastSeenAt    string `json:"lastSeenAt"`
	IdleMs        int64  `json:"idleMs"`
	PacketsIn     int64  `json:"packetsIn"`
	PacketsOut    int64  `json:"packetsOut"`
	BytesIn       int64  `json:"bytesIn"`
	BytesOut      int64  `json:"bytesOut"`
	Status        string `json:"status"`
}

type udpSessionEntry struct {
	id            string
	key           string
	ruleID        string
	ruleName      string
	mode          string
	clientAddress string
	clientPort    int
	targetAddress string
	targetPort    int
	startedAt     time.Time
	lastSeenAt    time.Time
	packetsIn     int64
	packetsOut    int64
	bytesIn       int64
	bytesOut      int64
}

// UdpSessionRegistry tracks active and recently-seen UDP sessions across all forwarding rules.
// All methods are safe for concurrent use.
type UdpSessionRegistry struct {
	mu      sync.Mutex
	entries map[string]*udpSessionEntry // id → entry
	keyToID map[string]string           // session key → id
}

// NewUdpSessionRegistry returns an empty, ready-to-use registry.
func NewUdpSessionRegistry() *UdpSessionRegistry {
	return &UdpSessionRegistry{
		entries: make(map[string]*udpSessionEntry),
		keyToID: make(map[string]string),
	}
}

// OpenOrTouchSession opens a new session or updates lastSeenAt on an existing one.
// Session identity is based on ruleID, mode, clientAddress, and clientPort.
// Returns the session ID.
func (r *UdpSessionRegistry) OpenOrTouchSession(input UdpSessionInput) string {
	key := udpSessionKey(input.RuleID, input.Mode, input.ClientAddress, input.ClientPort)

	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.keyToID[key]; ok {
		if entry, ok := r.entries[id]; ok {
			entry.lastSeenAt = time.Now().UTC()
			return id
		}
		// Defensive: stale key — entry was removed without cleaning keyToID.
		delete(r.keyToID, key)
	}

	id := generateConnectionID()
	now := time.Now().UTC()
	entry := &udpSessionEntry{
		id:            id,
		key:           key,
		ruleID:        input.RuleID,
		ruleName:      input.RuleName,
		mode:          input.Mode,
		clientAddress: input.ClientAddress,
		clientPort:    input.ClientPort,
		targetAddress: input.TargetAddress,
		targetPort:    input.TargetPort,
		startedAt:     now,
		lastSeenAt:    now,
	}
	r.entries[id] = entry
	r.keyToID[key] = id
	return id
}

// RecordInbound increments the inbound packet and byte counters and updates lastSeenAt.
// No-op if the ID is not found.
func (r *UdpSessionRegistry) RecordInbound(id string, byteCount int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		entry.packetsIn++
		entry.bytesIn += byteCount
		entry.lastSeenAt = time.Now().UTC()
	}
}

// RecordOutbound increments the outbound packet and byte counters.
// Does not update lastSeenAt (responses do not refresh session activity).
// No-op if the ID is not found.
func (r *UdpSessionRegistry) RecordOutbound(id string, byteCount int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		entry.packetsOut++
		entry.bytesOut += byteCount
	}
}

// CloseSession removes the session from the registry.
// No-op if the ID is not found.
func (r *UdpSessionRegistry) CloseSession(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		delete(r.keyToID, entry.key)
		delete(r.entries, id)
	}
}

// CloseSessionsForRule removes all sessions belonging to the given rule.
func (r *UdpSessionRegistry) CloseSessionsForRule(ruleID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, entry := range r.entries {
		if entry.ruleID == ruleID {
			delete(r.keyToID, entry.key)
			delete(r.entries, id)
		}
	}
}

// Len reports the number of sessions currently held in the registry, including
// any that are expired but not yet pruned. Snapshot hides expired sessions, so
// this is the accurate measure of retained memory — used to verify reclaim.
func (r *UdpSessionRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// PruneExpired removes sessions that have been idle for longer than UDPSessionExpireDuration.
func (r *UdpSessionRegistry) PruneExpired(now time.Time) {
	cutoff := now.Add(-UDPSessionExpireDuration)
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, entry := range r.entries {
		if entry.lastSeenAt.Before(cutoff) {
			delete(r.keyToID, entry.key)
			delete(r.entries, id)
		}
	}
}

// Snapshot returns a point-in-time view of all non-expired sessions.
// IdleMs and Status are calculated relative to now.
// Returns an empty (non-nil) slice when no sessions are present.
func (r *UdpSessionRegistry) Snapshot(now time.Time) []UdpSessionInfo {
	cutoff := now.Add(-UDPSessionExpireDuration)
	r.mu.Lock()
	snaps := make([]udpSessionEntry, 0, len(r.entries))
	for _, e := range r.entries {
		if !e.lastSeenAt.Before(cutoff) {
			snaps = append(snaps, *e)
		}
	}
	r.mu.Unlock()

	result := make([]UdpSessionInfo, 0, len(snaps))
	for i := range snaps {
		result = append(result, toUdpSessionInfo(&snaps[i], now))
	}
	return result
}

// SnapshotForRule returns a point-in-time view of non-expired sessions for a specific rule.
// Returns an empty (non-nil) slice when no matching sessions are present.
func (r *UdpSessionRegistry) SnapshotForRule(ruleID string, now time.Time) []UdpSessionInfo {
	cutoff := now.Add(-UDPSessionExpireDuration)
	r.mu.Lock()
	snaps := make([]udpSessionEntry, 0)
	for _, e := range r.entries {
		if e.ruleID == ruleID && !e.lastSeenAt.Before(cutoff) {
			snaps = append(snaps, *e)
		}
	}
	r.mu.Unlock()

	result := make([]UdpSessionInfo, 0, len(snaps))
	for i := range snaps {
		result = append(result, toUdpSessionInfo(&snaps[i], now))
	}
	return result
}

func udpSessionKey(ruleID, mode, clientAddress string, clientPort int) string {
	return fmt.Sprintf("%s:%s:%s:%d", ruleID, mode, clientAddress, clientPort)
}

func toUdpSessionInfo(e *udpSessionEntry, now time.Time) UdpSessionInfo {
	idle := now.Sub(e.lastSeenAt)
	status := "active"
	if idle >= UDPSessionIdleDuration {
		status = "idle"
	}
	return UdpSessionInfo{
		ID:            e.id,
		RuleID:        e.ruleID,
		RuleName:      e.ruleName,
		Protocol:      "udp",
		Mode:          e.mode,
		ClientAddress: e.clientAddress,
		ClientPort:    e.clientPort,
		TargetAddress: e.targetAddress,
		TargetPort:    e.targetPort,
		StartedAt:     e.startedAt.Format("2006-01-02T15:04:05.000Z"),
		LastSeenAt:    e.lastSeenAt.Format("2006-01-02T15:04:05.000Z"),
		IdleMs:        idle.Milliseconds(),
		PacketsIn:     e.packetsIn,
		PacketsOut:    e.packetsOut,
		BytesIn:       e.bytesIn,
		BytesOut:      e.bytesOut,
		Status:        status,
	}
}
