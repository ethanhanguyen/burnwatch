package analyze

import (
	"math"
	"testing"

	"github.com/ethanhanguyen/burnwatch/source"
)

func TestBuildSubagentTreeParentWithChildren(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "parent-1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false},
		{SessionID: "child-1", ParentSessionID: "parent-1", AgentType: "build", CostUSD: 2.0, IsSubagent: true},
		{SessionID: "child-2", ParentSessionID: "parent-1", AgentType: "explore", CostUSD: 3.0, IsSubagent: true},
	}
	trees := BuildSubagentTree(events)

	if len(trees) != 1 {
		t.Fatalf("expected 1 tree, got %d", len(trees))
	}
	tree := trees[0]
	if tree.SessionID != "parent-1" {
		t.Errorf("SessionID = %q, want %q", tree.SessionID, "parent-1")
	}

	expectedTotal := 1.0 + 2.0 + 3.0
	if math.Abs(tree.TotalCost-expectedTotal) > delta {
		t.Errorf("TotalCost = %f, want %f", tree.TotalCost, expectedTotal)
	}

	expectedSub := 2.0 + 3.0
	if math.Abs(tree.SubagentCost-expectedSub) > delta {
		t.Errorf("SubagentCost = %f, want %f", tree.SubagentCost, expectedSub)
	}

	expectedOverhead := expectedSub / expectedTotal * 100
	if math.Abs(tree.OverheadPct-expectedOverhead) > delta {
		t.Errorf("OverheadPct = %f, want %f", tree.OverheadPct, expectedOverhead)
	}

	if len(tree.Subagents) != 2 {
		t.Errorf("expected 2 subagent nodes, got %d", len(tree.Subagents))
	}
}

func TestBuildSubagentTreeNoSubagents(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false},
	}
	trees := BuildSubagentTree(events)

	if len(trees) != 1 {
		t.Fatalf("expected 1 tree, got %d", len(trees))
	}
	tree := trees[0]
	if tree.TotalCost != 1.0 {
		t.Errorf("TotalCost = %f, want 1.0", tree.TotalCost)
	}
	if tree.SubagentCost != 0 {
		t.Errorf("SubagentCost = %f, want 0", tree.SubagentCost)
	}
	if tree.OverheadPct != 0 {
		t.Errorf("OverheadPct = %f, want 0", tree.OverheadPct)
	}
	if len(tree.Subagents) != 0 {
		t.Errorf("expected 0 subagents, got %d", len(tree.Subagents))
	}
}

func TestBuildSubagentTreeNested(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "grandparent", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false},
		{SessionID: "parent-1", ParentSessionID: "grandparent", AgentType: "general", CostUSD: 2.0, IsSubagent: true},
		{SessionID: "child-1", ParentSessionID: "parent-1", AgentType: "build", CostUSD: 3.0, IsSubagent: true},
	}
	trees := BuildSubagentTree(events)

	if len(trees) != 1 {
		t.Fatalf("expected 1 tree, got %d", len(trees))
	}
	tree := trees[0]

	expectedTotal := 1.0 + 2.0 + 3.0
	if math.Abs(tree.TotalCost-expectedTotal) > delta {
		t.Errorf("TotalCost = %f, want %f", tree.TotalCost, expectedTotal)
	}

	if len(tree.Subagents) != 1 {
		t.Fatalf("expected 1 subagent node, got %d", len(tree.Subagents))
	}
	sa := tree.Subagents[0]
	if sa.SessionID != "parent-1" {
		t.Errorf("subagent SessionID = %q, want %q", sa.SessionID, "parent-1")
	}
	if len(sa.Children) != 1 {
		t.Fatalf("expected 1 grandchild, got %d", len(sa.Children))
	}
	gc := sa.Children[0]
	if gc.SessionID != "child-1" {
		t.Errorf("grandchild SessionID = %q, want %q", gc.SessionID, "child-1")
	}
}

func TestBuildSubagentTreeUnknownParent(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "orphan", ParentSessionID: "nonexistent", AgentType: "build", CostUSD: 2.0, IsSubagent: true},
	}
	trees := BuildSubagentTree(events)

	if len(trees) != 1 {
		t.Fatalf("expected 1 tree (toplevel with orphan), got %d", len(trees))
	}
	tree := trees[0]
	if tree.SessionID != "orphan" {
		t.Errorf("SessionID = %q, want %q", tree.SessionID, "orphan")
	}
	if tree.TotalCost != 2.0 {
		t.Errorf("TotalCost = %f, want 2.0", tree.TotalCost)
	}
}

func TestBuildSubagentTreeEmptyInput(t *testing.T) {
	trees := BuildSubagentTree(nil)
	if len(trees) != 0 {
		t.Errorf("expected 0 trees for nil, got %d", len(trees))
	}

	trees = BuildSubagentTree([]source.TokenEvent{})
	if len(trees) != 0 {
		t.Errorf("expected 0 trees for empty, got %d", len(trees))
	}
}

func TestBuildSubagentTreeMultipleSessions(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false},
		{SessionID: "child-a", ParentSessionID: "s1", AgentType: "build", CostUSD: 0.5, IsSubagent: true},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 2.0, IsSubagent: false},
	}
	trees := BuildSubagentTree(events)

	if len(trees) != 2 {
		t.Fatalf("expected 2 trees, got %d", len(trees))
	}

	s1 := findTree(trees, "s1")
	if s1 == nil {
		t.Fatal("tree for s1 not found")
	}
	if math.Abs(s1.TotalCost-1.5) > delta {
		t.Errorf("s1 TotalCost = %f, want 1.5", s1.TotalCost)
	}

	s2 := findTree(trees, "s2")
	if s2 == nil {
		t.Fatal("tree for s2 not found")
	}
	if s2.TotalCost != 2.0 {
		t.Errorf("s2 TotalCost = %f, want 2.0", s2.TotalCost)
	}
}

func findTree(trees []SubagentTree, sessionID string) *SubagentTree {
	for i := range trees {
		if trees[i].SessionID == sessionID {
			return &trees[i]
		}
	}
	return nil
}
