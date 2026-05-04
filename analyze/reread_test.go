package analyze

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func makeRereadEvents(sessionID string, fileOps []fileOpsSpec, cacheReads []int64) []source.TokenEvent {
	events := make([]source.TokenEvent, 0, len(fileOps))
	for i, spec := range fileOps {
		var cr int64
		if i < len(cacheReads) {
			cr = cacheReads[i]
		}
		var fileOps []source.FileOp
		for _, fo := range spec {
			fileOps = append(fileOps, source.FileOp{Path: fo.path, Operation: fo.op})
		}
		events = append(events, source.TokenEvent{
			SessionID:       sessionID,
			Project:         "p",
			Harness:         "h",
			Model:           "test-model",
			Timestamp:       time.Date(2026, 5, 1, 10, i, 0, 0, time.UTC),
			InputTokens:     100,
			OutputTokens:    50,
			CostUSD:         0.01,
			CacheRead:       cr,
			EventIndex:      i + 1,
			FileOps:         fileOps,
		})
	}
	return events
}

type fileOpsSpec []struct {
	path string
	op   string
}

func TestFileReRead_SingleRead(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"a.go", "read"}},
	}, nil)

	signals := detectFileReReads(events, 3)
	if len(signals) != 0 {
		t.Errorf("expected no signal for single read, got %d", len(signals))
	}
}

func TestFileReRead_ReadWithCache(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"a.go", "read"}},
		{{"b.go", "read"}},
		{{"a.go", "read"}},
		{{"a.go", "read"}},
	}, []int64{0, 0, 5000, 0})

	signals := detectFileReReads(events, 3)
	if len(signals) != 0 {
		t.Errorf("expected no signal for cached re-reads, got %d", len(signals))
	}
}

func TestFileReRead_ReReadNoCache(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"config/settings.json", "read"}},
		{{"src/types.go", "edit"}},
		{{"config/settings.json", "read"}},
		{{"config/settings.json", "read"}},
		{{"config/settings.json", "read"}},
	}, nil)

	signals := detectFileReReads(events, 3)
	if len(signals) == 0 {
		t.Fatal("expected signal for uncached re-reads")
	}
	s := signals[0]
	if s.SessionID != "ses" {
		t.Errorf("SessionID = %s, want ses", s.SessionID)
	}
	if s.Severity != "medium" {
		t.Errorf("Severity = %s, want medium", s.Severity)
	}
	if s.Reason != "file_reread" {
		t.Errorf("Reason = %s, want file_reread", s.Reason)
	}
	if !strings.Contains(s.Detail, "config/settings.json") {
		t.Errorf("Detail missing file path: %s", s.Detail)
	}
}

func TestFileReRead_MixedFiles(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"a.go", "read"}},
		{{"b.go", "read"}},
		{{"a.go", "read"}},
		{{"a.go", "read"}},
		{{"a.go", "read"}},
		{{"b.go", "read"}},
	}, nil)

	signals := detectFileReReads(events, 3)
	foundA := false
	foundB := false
	for _, s := range signals {
		if strings.Contains(s.Detail, "a.go") {
			foundA = true
		}
		if strings.Contains(s.Detail, "b.go") {
			foundB = true
		}
	}
	if !foundA {
		t.Error("expected signal for a.go (4 reads, no cache)")
	}
	if foundB {
		t.Error("b.go read only 2 times, should not trigger")
	}
}

func TestFileReRead_WriteBetweenReads(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"src/handler.go", "read"}},
		{{"src/handler.go", "edit"}},
		{{"src/handler.go", "read"}},
		{{"src/handler.go", "read"}},
	}, nil)

	signals := detectFileReReads(events, 3)
	if len(signals) == 0 {
		t.Fatal("expected signal for re-reads after write, cache=0")
	}
}

func TestFileReRead_MultipleSessions(t *testing.T) {
	events := makeRereadEvents("clean", []fileOpsSpec{
		{{"x.go", "read"}},
		{{"y.go", "read"}},
	}, nil)
	events = append(events, makeRereadEvents("waste", []fileOpsSpec{
		{{"data.json", "read"}},
		{{"data.json", "read"}},
		{{"data.json", "read"}},
		{{"data.json", "read"}},
	}, nil)...)

	signals := detectFileReReads(events, 3)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SessionID != "waste" {
		t.Errorf("SessionID = %s, want waste", signals[0].SessionID)
	}
}

func TestFileReRead_EventRangeCache(t *testing.T) {
	events := makeRereadEvents("ses", []fileOpsSpec{
		{{"a.go", "read"}},
		{{"b.go", "write"}},
		{{"a.go", "read"}},
		{{"a.go", "read"}},
	}, []int64{0, 5000, 0, 0})

	signals := detectFileReReads(events, 3)
	if len(signals) != 0 {
		t.Errorf("cache hit on event 2 between first and last read of a.go should prevent flag, got %d signals", len(signals))
	}
}
