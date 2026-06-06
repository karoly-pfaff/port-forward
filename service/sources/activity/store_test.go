package activity

import (
	"fmt"
	"sync"
	"testing"
)

func TestStoreAddReturnsEventWithIDAndTimestamp(t *testing.T) {
	var s Store
	e := s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, Message: "created"})
	if e.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if e.Timestamp == "" {
		t.Fatal("expected non-empty Timestamp")
	}
	if e.Type != EventRuleCreated {
		t.Fatalf("type = %q, want %q", e.Type, EventRuleCreated)
	}
}

func TestStoreNewestFirst(t *testing.T) {
	var s Store
	s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, Message: "first"})
	s.Add(ActivityEventInput{Type: "rule.updated", Severity: SeverityInfo, Message: "second"})
	s.Add(ActivityEventInput{Type: "rule.deleted", Severity: SeverityWarning, Message: "third"})

	events := s.List(ListParams{})
	if len(events) != 3 {
		t.Fatalf("count = %d, want 3", len(events))
	}
	if events[0].Message != "third" {
		t.Fatalf("first event message = %q, want third", events[0].Message)
	}
	if events[2].Message != "first" {
		t.Fatalf("last event message = %q, want first", events[2].Message)
	}
}

func TestStoreBoundedToMaxEvents(t *testing.T) {
	var s Store
	for i := 0; i < MaxEvents+10; i++ {
		s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, Message: fmt.Sprintf("event %d", i)})
	}
	events := s.List(ListParams{Limit: MaxEvents})
	if len(events) != MaxEvents {
		t.Fatalf("count = %d, want %d", len(events), MaxEvents)
	}
	// Newest (highest index) should be first.
	if events[0].Message != fmt.Sprintf("event %d", MaxEvents+9) {
		t.Fatalf("first event = %q", events[0].Message)
	}
}

func TestStoreDefaultLimit(t *testing.T) {
	var s Store
	for i := 0; i < DefaultLimit+50; i++ {
		s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, Message: "x"})
	}
	events := s.List(ListParams{})
	if len(events) != DefaultLimit {
		t.Fatalf("default limit = %d, want %d", len(events), DefaultLimit)
	}
}

func TestStoreMaxLimitCapped(t *testing.T) {
	var s Store
	for i := 0; i < MaxEvents; i++ {
		s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, Message: "x"})
	}
	events := s.List(ListParams{Limit: MaxEvents + 999})
	if len(events) != MaxEvents {
		t.Fatalf("capped limit = %d, want %d", len(events), MaxEvents)
	}
}

func TestStoreFilterByRuleID(t *testing.T) {
	var s Store
	ruleA := "rule-a"
	ruleB := "rule-b"
	s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, RuleID: &ruleA, Message: "a"})
	s.Add(ActivityEventInput{Type: "rule.created", Severity: SeverityInfo, RuleID: &ruleB, Message: "b"})
	s.Add(ActivityEventInput{Type: "rule.updated", Severity: SeverityInfo, RuleID: &ruleA, Message: "a2"})

	events := s.List(ListParams{RuleID: &ruleA})
	if len(events) != 2 {
		t.Fatalf("count = %d, want 2", len(events))
	}
	for _, e := range events {
		if e.RuleID == nil || *e.RuleID != ruleA {
			t.Fatalf("unexpected ruleId = %v", e.RuleID)
		}
	}
}

func TestStoreFilterByType(t *testing.T) {
	var s Store
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, Message: "c"})
	s.Add(ActivityEventInput{Type: EventRuleUpdated, Severity: SeverityInfo, Message: "u"})
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, Message: "c2"})

	typeFilter := EventRuleCreated
	events := s.List(ListParams{Type: &typeFilter})
	if len(events) != 2 {
		t.Fatalf("count = %d, want 2", len(events))
	}
	for _, e := range events {
		if e.Type != EventRuleCreated {
			t.Fatalf("unexpected type = %q", e.Type)
		}
	}
}

func TestStoreFilterBySeverity(t *testing.T) {
	var s Store
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, Message: "ok"})
	s.Add(ActivityEventInput{Type: EventRuleError, Severity: SeverityError, Message: "err"})
	s.Add(ActivityEventInput{Type: EventRuleUpdated, Severity: SeverityInfo, Message: "info"})

	sev := SeverityError
	events := s.List(ListParams{Severity: &sev})
	if len(events) != 1 {
		t.Fatalf("count = %d, want 1", len(events))
	}
	if events[0].Severity != SeverityError {
		t.Fatalf("severity = %q, want error", events[0].Severity)
	}
}

func TestStoreFilterByMultipleParams(t *testing.T) {
	var s Store
	ruleA := "rule-a"
	ruleB := "rule-b"
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, RuleID: &ruleA, Message: "a-created"})
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, RuleID: &ruleB, Message: "b-created"})
	s.Add(ActivityEventInput{Type: EventRuleError, Severity: SeverityError, RuleID: &ruleA, Message: "a-error"})

	typeFilter := EventRuleCreated
	events := s.List(ListParams{RuleID: &ruleA, Type: &typeFilter})
	if len(events) != 1 {
		t.Fatalf("count = %d, want 1", len(events))
	}
	if events[0].Message != "a-created" {
		t.Fatalf("message = %q", events[0].Message)
	}
}

func TestStoreUnknownFilterReturnsEmpty(t *testing.T) {
	var s Store
	s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeveritySuccess, Message: "x"})

	unknownType := "no.such.type"
	events := s.List(ListParams{Type: &unknownType})
	if len(events) != 0 {
		t.Fatalf("expected empty result for unknown type, got %d", len(events))
	}
}

func TestStoreThreadSafety(t *testing.T) {
	var s Store
	var wg sync.WaitGroup
	goroutines := 50
	eventsEach := 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < eventsEach; j++ {
				s.Add(ActivityEventInput{Type: EventRuleCreated, Severity: SeverityInfo, Message: fmt.Sprintf("g%d-e%d", n, j)})
			}
		}(i)
	}
	wg.Wait()

	events := s.List(ListParams{Limit: MaxEvents})
	// Total added = goroutines * eventsEach = 1000, bounded to MaxEvents = 500
	if len(events) != MaxEvents {
		t.Fatalf("after concurrent adds: count = %d, want %d", len(events), MaxEvents)
	}
}
