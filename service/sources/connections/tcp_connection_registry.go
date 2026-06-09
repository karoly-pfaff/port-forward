package connections

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TcpConnectionInput holds the data needed to open a tracked TCP connection.
type TcpConnectionInput struct {
	RuleID        string
	RuleName      string
	ClientAddress string
	ClientPort    int
	TargetAddress string
	TargetPort    int
}

// TcpConnectionInfo is a serializable snapshot of one live TCP connection.
type TcpConnectionInfo struct {
	ID            string `json:"id"`
	RuleID        string `json:"ruleId"`
	RuleName      string `json:"ruleName"`
	Protocol      string `json:"protocol"`
	ClientAddress string `json:"clientAddress"`
	ClientPort    int    `json:"clientPort"`
	TargetAddress string `json:"targetAddress"`
	TargetPort    int    `json:"targetPort"`
	StartedAt     string `json:"startedAt"`
	DurationMs    int64  `json:"durationMs"`
	BytesIn       int64  `json:"bytesIn"`
	BytesOut      int64  `json:"bytesOut"`
	Status        string `json:"status"`
}

type tcpConnectionEntry struct {
	id            string
	ruleID        string
	ruleName      string
	clientAddress string
	clientPort    int
	targetAddress string
	targetPort    int
	startedAt     time.Time
	bytesIn       int64 // updated via atomic ops
	bytesOut      int64 // updated via atomic ops
}

// TcpConnectionRegistry tracks live TCP connections across all forwarding rules.
// All methods are safe for concurrent use.
type TcpConnectionRegistry struct {
	mu      sync.Mutex
	entries map[string]*tcpConnectionEntry
}

// NewTcpConnectionRegistry returns an empty, ready-to-use registry.
func NewTcpConnectionRegistry() *TcpConnectionRegistry {
	return &TcpConnectionRegistry{
		entries: make(map[string]*tcpConnectionEntry),
	}
}

// OpenConnection records a new live TCP connection and returns its registry ID.
func (r *TcpConnectionRegistry) OpenConnection(input TcpConnectionInput) string {
	id := generateConnectionID()
	entry := &tcpConnectionEntry{
		id:            id,
		ruleID:        input.RuleID,
		ruleName:      input.RuleName,
		clientAddress: input.ClientAddress,
		clientPort:    input.ClientPort,
		targetAddress: input.TargetAddress,
		targetPort:    input.TargetPort,
		startedAt:     time.Now().UTC(),
	}
	r.mu.Lock()
	r.entries[id] = entry
	r.mu.Unlock()
	return id
}

// AddBytesIn increments the bytesIn counter for the given connection ID.
// No-op if the ID is not found.
func (r *TcpConnectionRegistry) AddBytesIn(id string, n int64) {
	r.mu.Lock()
	entry, ok := r.entries[id]
	r.mu.Unlock()
	if ok {
		atomic.AddInt64(&entry.bytesIn, n)
	}
}

// AddBytesOut increments the bytesOut counter for the given connection ID.
// No-op if the ID is not found.
func (r *TcpConnectionRegistry) AddBytesOut(id string, n int64) {
	r.mu.Lock()
	entry, ok := r.entries[id]
	r.mu.Unlock()
	if ok {
		atomic.AddInt64(&entry.bytesOut, n)
	}
}

// CloseConnection removes the connection from the registry.
// No-op if the ID is not found.
func (r *TcpConnectionRegistry) CloseConnection(id string) {
	r.mu.Lock()
	delete(r.entries, id)
	r.mu.Unlock()
}

// CloseConnectionsForRule removes all connections belonging to the given rule.
func (r *TcpConnectionRegistry) CloseConnectionsForRule(ruleID string) {
	r.mu.Lock()
	for id, entry := range r.entries {
		if entry.ruleID == ruleID {
			delete(r.entries, id)
		}
	}
	r.mu.Unlock()
}

// Snapshot returns a point-in-time view of all live connections.
// DurationMs is calculated relative to now.
// Returns an empty (non-nil) slice when no connections are active.
func (r *TcpConnectionRegistry) Snapshot(now time.Time) []TcpConnectionInfo {
	r.mu.Lock()
	entries := make([]*tcpConnectionEntry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}
	r.mu.Unlock()

	result := make([]TcpConnectionInfo, 0, len(entries))
	for _, e := range entries {
		result = append(result, r.toInfo(e, now))
	}
	return result
}

// SnapshotForRule returns a point-in-time view of connections for a specific rule.
// Returns an empty (non-nil) slice when no matching connections are active.
func (r *TcpConnectionRegistry) SnapshotForRule(ruleID string, now time.Time) []TcpConnectionInfo {
	r.mu.Lock()
	entries := make([]*tcpConnectionEntry, 0)
	for _, e := range r.entries {
		if e.ruleID == ruleID {
			entries = append(entries, e)
		}
	}
	r.mu.Unlock()

	result := make([]TcpConnectionInfo, 0, len(entries))
	for _, e := range entries {
		result = append(result, r.toInfo(e, now))
	}
	return result
}

func (r *TcpConnectionRegistry) toInfo(e *tcpConnectionEntry, now time.Time) TcpConnectionInfo {
	return TcpConnectionInfo{
		ID:            e.id,
		RuleID:        e.ruleID,
		RuleName:      e.ruleName,
		Protocol:      "tcp",
		ClientAddress: e.clientAddress,
		ClientPort:    e.clientPort,
		TargetAddress: e.targetAddress,
		TargetPort:    e.targetPort,
		StartedAt:     e.startedAt.Format("2006-01-02T15:04:05.000Z"),
		DurationMs:    now.Sub(e.startedAt).Milliseconds(),
		BytesIn:       atomic.LoadInt64(&e.bytesIn),
		BytesOut:      atomic.LoadInt64(&e.bytesOut),
		Status:        "active",
	}
}

func generateConnectionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("conn-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	enc := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", enc[0:8], enc[8:12], enc[12:16], enc[16:20], enc[20:32])
}
