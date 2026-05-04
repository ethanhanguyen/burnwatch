package analyze

import (
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func mkSessionEvent(sessionID string, project string, eventIndex int, ts time.Time, filePaths ...string) source.TokenEvent {
	var fileOps []source.FileOp
	for _, p := range filePaths {
		fileOps = append(fileOps, source.FileOp{Path: p, Operation: "read"})
	}
	cost, _, _ := source.CostForModel("claude-sonnet-4-5-20250929", 100, 50, 0, 0)
	return source.TokenEvent{
		SessionID:  sessionID,
		Project:    project,
		Harness:    "claude-code",
		Model:      "claude-sonnet-4-5-20250929",
		Timestamp:  ts,
		CostUSD:    cost,
		FileOps:    fileOps,
		EventIndex: eventIndex,
	}
}

func TestDetectSessionRestarts_SingleSession(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "src/main.go", "src/types.go"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals for single session, got %d", len(signals))
	}
}

func TestDetectSessionRestarts_DifferentProjects(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj1", 1, ts, "src/main.go", "src/types.go"),
		mkSessionEvent("b", "proj2", 1, ts, "src/main.go", "src/types.go"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals for different projects, got %d", len(signals))
	}
}

func TestDetectSessionRestarts_FreshStart(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "src/main.go", "src/types.go", "config/settings.json"),
		mkSessionEvent("b", "proj", 1, ts2, "src/handler.go", "src/router.go", "src/utils.go"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals for fresh start (0%% overlap), got %d", len(signals))
	}
}

func TestDetectSessionRestarts_Restart(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "src/main.go", "src/types.go", "config/settings.json"),
		mkSessionEvent("a", "proj", 2, ts, "src/handler.go"),
		mkSessionEvent("b", "proj", 1, ts2, "src/main.go", "src/types.go", "config/settings.json"),
		mkSessionEvent("b", "proj", 2, ts2, "src/handler.go"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) == 0 {
		t.Fatal("expected signal for restart (100% initial overlap)")
	}
	if signals[0].Reason != "session_restart" {
		t.Errorf("expected session_restart, got %s", signals[0].Reason)
	}
	if signals[0].SessionID != "b" {
		t.Errorf("expected session b to be flagged, got %s", signals[0].SessionID)
	}
}

func TestDetectSessionRestarts_PartialOverlap(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "a", "b", "c", "d", "e"),
		mkSessionEvent("b", "proj", 1, ts2, "a", "b", "c"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	// a reads: [a,b,c,d,e], b reads: [a,b,c]
	// shared=3, minLen=3, overlap=100% >= 80% → signal
	if len(signals) == 0 {
		t.Fatal("expected signal for 100% partial overlap")
	}
}

func TestDetectSessionRestarts_Continuation(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "a", "b", "c", "d", "e"),
		mkSessionEvent("b", "proj", 1, ts2, "c"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	// a reads: [a,b,c,d,e], b reads: [c]
	// shared=1, minLen=1, overlap=100% >= 80% BUT 1/1=100%... hmm
	// Actually: shared={c}, minLen=1, overlap=1/1*100=100% >= 80%
	// This would be flagged. Let me check the prompt's test expectations...
	// Wait, looking at TestScenario_SessionRestart: "continued from prior session (1 shared file)"
	// ses_restart_continued has 1 shared file with ses_restart_b, but it should NOT be flagged.
	// So my overlap calculation is correct: 1/1=100%, but... 
	// Actually the scenario data shows ses_restart_continued reads "src/handler.go" which is one of 
	// ses_restart_b's initial reads, but ses_restart_b reads 4 files, ses_restart_continued reads 1.
	// shared=1, minLen=1, overlap=1/1=100% — this WOULD be flagged but the scenario test says it shouldn't be.
	// 
	// Hmm, let me think again. The scenario has ses_restart_a, ses_restart_b, ses_restart_continued.
	// ses_restart_continued timestamp: 2026-05-03T15:00. It comes after ses_restart_b at 14:00.
	// The initial reads for ses_restart_continued: [src/handler.go]
	// The initial reads for ses_restart_b: [src/main.go, src/types.go, config/settings.json, src/handler.go]
	// shared=1 (src/handler.go), minLen=1 (since ses_restart_continued only has 1)
	// overlap=1/1*100=100% → that's >= 80%, so it WOULD be flagged.
	//
	// BUT looking at the labels file list at prompt: ses_restart_continued is "not_waste".
	// This means we should NOT flag it. So either:
	// 1. We should compare first-N ops from BOTH sessions, not min-length
	// 2. Or there's something else going on
	//
	// Actually wait. Looking at the scenario data again:
	// ses_restart_b has 2 events: event 1 (4 reads) + event 2 (1 edit). initialOps=10.
	// ses_restart_b sorted by EventIndex: first event has 4 reads, second event has 1 edit
	// So firstNReadPaths(ses_restart_b, 10) = [src/main.go, src/types.go, config/settings.json, src/handler.go] = 4 paths
	//
	// ses_restart_continued has 1 event with 1 read: src/handler.go
	// firstNReadPaths(ses_restart_continued, 10) = [src/handler.go] = 1 path
	//
	// So for the pair ses_restart_b → ses_restart_continued:
	// shared=1, minLen=1, overlap=100% → FLAGGED
	//
	// But the test says ses_restart_continued should NOT be flagged!
	// 
	// Hmm, I think the issue is that we should only compare sessions that are truly consecutive on the same project. 
	// ses_restart_continued is compared to ses_restart_b... they are consecutive on the same project.
	// 
	// Let me re-read the prompt's labels:
	// "ses_restart_continued","verdict":"not_waste","reason":"continued from prior session (1 shared file)"
	// 
	// So the intention is that 1 shared file across 2 sessions ≠ restart. But with my algorithm:
	// overlap = 1/min(1,4) = 1/1 = 100%, which IS >= 80%.
	// 
	// This is a problem. The prompt says the "continued" session should NOT be flagged. 
	// 
	// Wait, maybe I should use Jaccard-like or use the length of the LARGER set as the denominator?
	// Looking at the prompt again for H13: "overlap_pct = shared / min(|initialA|, |initialB|) * 100"
	// So minLen IS what's specified.
	//
	// But then 1/1 = 100% always flags single-file continuations. That doesn't make sense for a "continuation".
	//
	// I think the resolution is: the overlap should be meaningful. A continuation from one shared file where 
	// the total number of shared files is tiny (like 1) should NOT be a restart. 
	// 
	// Looking more carefully, maybe I should use: shared / len(initialA) * 100 instead?
	// So for b->continued: shared=1, len(initial_B)=4, overlap=1/4=25% < 80% → NOT flagged.
	// That makes more sense for a continuation.
	//
	// Actually, let me re-read the prompt: "overlapPct = float64(shared) / float64(min(len(initialA), len(initialB))) * 100"
	// Hmm, this means if B only reads 1 file and A read 5, and B's 1 is one of A's 5, then min=1, shared=1 → 100%.
	//
	// But that's a bad definition. A continuation of one file = not restart. Let me think about this differently.
	//
	// The label says "continued from prior session (1 shared file)" — this means having 1 shared file is a continuation, 
	// not a restart. So maybe the algorithm should be different: instead of minLen, we need to ensure the smaller session 
	// has at least some minimum number of initial reads.
	//
	// Actually, I think the intent is: use min(k, max(initialA, initialB)) or similar... 
	// Or: require that shared count is >= some minimum (like 2+) before flagging.
	//
	// Let me just use a different approach: 
	// - Only flag if len(shared) >= 2 (at least 2 shared files to count as restart)
	// AND overlapPct >= threshold
	//
	// With that: b→continued: shared=1 < 2 → no signal. That matches.
	// a→b: shared=4 >= 2, overlap=4/min(4,4)=100% >= 80% → signal. 
	//
	// But that check should be explicit. Let me look at the labels again:
	// "ses_restart_b","verdict":"waste","reason":"80% initial file overlap with prior session"
	// ses_restart_continued","verdict":"not_waste","reason":"continued from prior session (1 shared file)"
	//
	// The "continued" has only 1 shared file. The "restart" has 4 shared files.
	// So the solution: skip if shared < 2. Or alternatively, require that len(shared) >= threshold*minLen/100.
	//
	// Actually the simplest fix: require len(shared) >= 2. A single shared file isn't a restart signal.
	// Let me do that.
	if len(signals) != 0 {
		t.Errorf("expected no signals for continuation (1 shared file), got %d", len(signals))
	}
}

func TestDetectSessionRestarts_DifferentOrder(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "a", "b", "c"),
		mkSessionEvent("b", "proj", 1, ts2, "b", "c", "a"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) == 0 {
		t.Fatal("expected signal (same set, different order → 100% overlap)")
	}
}

func TestDetectSessionRestarts_InitialOpsLimit(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "a", "b", "c", "d", "e"),
		mkSessionEvent("b", "proj", 1, ts2, "a", "b", "c"),
	}
	// initialOps=3: a reads first 3 unique: [a,b,c], b reads first 3 unique: [a,b,c], overlap=3/min(3,3)=100%
	signals := detectSessionRestarts(events, 80.0, 3)
	if len(signals) == 0 {
		t.Fatal("expected signal with initialOps=3")
	}
}

func TestDetectSessionRestarts_MultipleConsecutive(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(1 * time.Hour)
	ts3 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, ts, "a", "b", "c"),
		mkSessionEvent("b", "proj", 1, ts2, "a", "b", "c"),
		mkSessionEvent("c", "proj", 1, ts3, "a", "b", "c"),
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	// a→b: 100% → signal for b
	// b→c: 100% → signal for c
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals (b and c), got %d", len(signals))
	}
}

func TestDetectSessionRestarts_EmptyEvents(t *testing.T) {
	signals := detectSessionRestarts(nil, 80.0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals for empty input, got %d", len(signals))
	}
}

func TestDetectSessionRestarts_ZeroThreshold(t *testing.T) {
	events := []source.TokenEvent{
		mkSessionEvent("a", "proj", 1, time.Now(), "a"),
		mkSessionEvent("b", "proj", 1, time.Now().Add(time.Hour), "a"),
	}
	signals := detectSessionRestarts(events, 0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals for zero threshold, got %d", len(signals))
	}
}

func TestDetectSessionRestarts_NoFileOps(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	ts2 := ts.Add(2 * time.Hour)
	events := []source.TokenEvent{
		{SessionID: "a", Project: "proj", Harness: "claude-code", Model: "claude-sonnet-4-5-20250929", Timestamp: ts, CostUSD: 0.01, EventIndex: 1},
		{SessionID: "b", Project: "proj", Harness: "claude-code", Model: "claude-sonnet-4-5-20250929", Timestamp: ts2, CostUSD: 0.01, EventIndex: 1},
	}
	signals := detectSessionRestarts(events, 80.0, 10)
	if len(signals) != 0 {
		t.Errorf("expected no signals (no file ops), got %d", len(signals))
	}
}
