package activity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	MaxEvents    = 500
	DefaultLimit = 100
)

// ListParams are optional filters for Store.List.
type ListParams struct {
	Limit    int     // 0 means DefaultLimit; capped at MaxEvents
	RuleID   *string // nil means no filter
	Type     *string // nil means no filter
	Severity *string // nil means no filter
}

// Store is a bounded in-memory activity event log. Thread-safe. Resets on service restart.
type Store struct {
	mu     sync.Mutex
	events []ActivityEvent
}

// Add records a new event, prepending it so the slice stays newest-first.
// The store is capped at MaxEvents; oldest events are dropped.
func (s *Store) Add(input ActivityEventInput) ActivityEvent {
	event := ActivityEvent{
		ID:        randomEventID(),
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Type:      input.Type,
		Severity:  input.Severity,
		RuleID:    input.RuleID,
		RuleName:  input.RuleName,
		Protocol:  input.Protocol,
		Message:   input.Message,
		Details:   input.Details,
	}

	s.mu.Lock()
	s.events = append([]ActivityEvent{event}, s.events...)
	if len(s.events) > MaxEvents {
		s.events = s.events[:MaxEvents]
	}
	s.mu.Unlock()

	return event
}

// List returns events newest-first, applying optional filters before the limit.
func (s *Store) List(params ListParams) []ActivityEvent {
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxEvents {
		limit = MaxEvents
	}

	s.mu.Lock()
	snapshot := make([]ActivityEvent, len(s.events))
	copy(snapshot, s.events)
	s.mu.Unlock()

	result := make([]ActivityEvent, 0, limit)
	for _, e := range snapshot {
		if params.RuleID != nil && (e.RuleID == nil || *e.RuleID != *params.RuleID) {
			continue
		}
		if params.Type != nil && e.Type != *params.Type {
			continue
		}
		if params.Severity != nil && e.Severity != *params.Severity {
			continue
		}
		result = append(result, e)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func randomEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	enc := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", enc[0:8], enc[8:12], enc[12:16], enc[16:20], enc[20:32])
}
