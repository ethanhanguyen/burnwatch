package analyze

import "github.com/ethanhanguyen/burnwatch/source"

type SubagentTree struct {
	SessionID    string
	Subagents    []SubagentNode
	TotalCost    float64
	SubagentCost float64
	OverheadPct  float64
}

type SubagentNode struct {
	SessionID string
	AgentType string
	Cost      float64
	Children  []SubagentNode
}

type sessionInfo struct {
	cost        float64
	agentType   string
	isSubagent  bool
	parentID    string
	hasToplevel bool
}

func BuildSubagentTree(events []source.TokenEvent) []SubagentTree {
	if len(events) == 0 {
		return nil
	}

	sessions := make(map[string]*sessionInfo)
	childMap := make(map[string][]string)

	for _, e := range events {
		info, ok := sessions[e.SessionID]
		if !ok {
			info = &sessionInfo{}
			sessions[e.SessionID] = info
		}
		info.cost += e.CostUSD
		if !e.IsSubagent {
			info.hasToplevel = true
		}
		if e.IsSubagent {
			info.isSubagent = true
			info.parentID = e.ParentSessionID
			info.agentType = e.AgentType
		}
	}

	for sid, info := range sessions {
		if info.isSubagent && info.parentID != "" {
			childMap[info.parentID] = append(childMap[info.parentID], sid)
		}
	}

	var trees []SubagentTree
	for sid, info := range sessions {
		if info.hasToplevel {
			children := buildChildNodes(sid, childMap, sessions)
			totalCost := info.cost
			subCost := sumSubagentCosts(children)
			totalCost += subCost

			var overhead float64
			if totalCost > 0 {
				overhead = subCost / totalCost * 100
			}

			trees = append(trees, SubagentTree{
				SessionID:    sid,
				Subagents:    children,
				TotalCost:    totalCost,
				SubagentCost: subCost,
				OverheadPct:  overhead,
			})
		}
	}

	for sid, info := range sessions {
		if info.isSubagent && !info.hasToplevel && info.parentID != "" {
			_, parentInSessions := sessions[info.parentID]
			if !parentInSessions {
				children := buildChildNodes(sid, childMap, sessions)
				totalCost := info.cost
				subCost := sumSubagentCosts(children)
				totalCost += subCost

				var overhead float64
				if totalCost > 0 {
					overhead = subCost / totalCost * 100
				}

				trees = append(trees, SubagentTree{
					SessionID:    sid,
					Subagents:    children,
					TotalCost:    totalCost,
					SubagentCost: subCost,
					OverheadPct:  overhead,
				})
			}
		}
	}

	return trees
}

func buildChildNodes(sessionID string, childMap map[string][]string, sessions map[string]*sessionInfo) []SubagentNode {
	return buildChildNodesVisited(sessionID, childMap, sessions, nil)
}

func buildChildNodesVisited(sessionID string, childMap map[string][]string, sessions map[string]*sessionInfo, visited map[string]bool) []SubagentNode {
	if visited == nil {
		visited = make(map[string]bool)
	}
	if visited[sessionID] {
		return nil
	}
	visited[sessionID] = true

	childIDs := childMap[sessionID]
	if len(childIDs) == 0 {
		return nil
	}

	var nodes []SubagentNode
	for _, cid := range childIDs {
		info := sessions[cid]
		if info == nil {
			continue
		}
		children := buildChildNodesVisited(cid, childMap, sessions, visited)
		totalCost := info.cost + sumSubagentCosts(children)

		nodes = append(nodes, SubagentNode{
			SessionID: cid,
			AgentType: info.agentType,
			Cost:      totalCost,
			Children:  children,
		})
	}
	return nodes
}

func sumSubagentCosts(nodes []SubagentNode) float64 {
	var sum float64
	for _, n := range nodes {
		sum += n.Cost
	}
	return sum
}
