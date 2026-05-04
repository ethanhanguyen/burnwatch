package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

func FormatReport(events []source.TokenEvent, baselines map[string]analyze.Baseline, signals []analyze.WasteSignal, recommendations []analyze.Recommendation, trees []analyze.SubagentTree, version string, generated time.Time) string {
	data := computeReportData(events, baselines, signals, trees)
	data.Version = version
	data.Generated = generated.Format(time.RFC3339)

	jsonData, err := json.Marshal(data)
	if err != nil {
		jsonData = []byte("{}")
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString(`<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>burnwatch — Waste Report</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Cinzel:wght@400;500;600;700&family=Cormorant+Garamond:ital,wght@0,400;0,500;0,600;0,700;1,400;1,500&family=Spectral:ital,wght@0,300;0,400;0,500;0,600;1,400&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<style>
`)
	b.WriteString(renderCSS())
	b.WriteString(`</style>
</head>
<body>
<div class="shell">
`)
	b.WriteString(renderSidebar(version, generated, data.Summary))
	b.WriteString(`<main>
`)
	b.WriteString(renderBanner(data.Summary))
	b.WriteString(renderHero(data.Summary))
	b.WriteString(renderFlowSection())
	b.WriteString(renderWasteSection())
	b.WriteString(renderTreeSection())
	b.WriteString(renderSignalsSection(data.Summary))
	b.WriteString(renderFooter(data.Version, data.Generated))
	b.WriteString(`</main>
</div>
<script>
const REPORT = `)
	b.Write(jsonData)
	b.WriteString(`;
`)
	b.WriteString(renderJS())
	b.WriteString(`</script>
</body>
</html>`)

	return b.String()
}

func renderCSS() string {
	return `:root {
  --ink:           #1a0f0a;
  --ink-deep:      #120906;
  --parchment:     #f5e6d3;
  --parchment-dim: #bfa98a;
  --parchment-faint: #8a7659;
  --surface:       #2c1a10;
  --surface-2:     #3a2316;
  --surface-3:     #4a2d1c;
  --surface-glow:  #5c3d2e;
  --rule:          rgba(202,138,4,0.28);
  --rule-strong:   rgba(202,138,4,0.55);
  --rule-faint:    rgba(202,138,4,0.10);
  --gold:          #ca8a04;
  --gold-bright:   #eab308;
  --gold-deep:     #92670a;
  --copper:        #b8470a;
  --ember:         #d04a1a;
  --crimson:       #b91c1c;
  --crimson-glow:  #f87171;
  --indigo:        #6d28d9;
  --indigo-glow:   #a78bfa;
  --moss:          #4d7c2a;
  --moss-glow:     #84cc16;
  --r-sm: 2px;
  --r:    4px;
  --r-lg: 6px;
  --s1: 4px; --s2: 8px; --s3: 12px; --s4: 16px; --s5: 24px; --s6: 32px; --s7: 48px; --s8: 64px;
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html { background: var(--ink-deep); }
body {
  background:
    radial-gradient(ellipse at 20% 0%, rgba(202,138,4,0.05), transparent 50%),
    radial-gradient(ellipse at 80% 30%, rgba(184,71,10,0.04), transparent 50%),
    var(--ink);
  color: var(--parchment);
  font-family: 'Spectral', Georgia, serif;
  font-size: 15px;
  font-weight: 400;
  line-height: 1.65;
  min-height: 100vh;
  overflow-x: hidden;
}
body::before {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  opacity: 0.035;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='200' height='200'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' seed='3'/%3E%3CfeColorMatrix values='0 0 0 0 0.95 0 0 0 0 0.85 0 0 0 0 0.7 0 0 0 1 0'/%3E%3C/filter%3E%3Crect width='200' height='200' filter='url(%23n)'/%3E%3C/svg%3E");
}
.shell {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  max-width: 1440px;
  margin: 0 auto;
  position: relative;
  z-index: 2;
}
.rail {
  position: sticky;
  top: 0;
  height: 100vh;
  padding: var(--s7) var(--s5) var(--s5);
  border-right: 1px solid var(--rule);
  display: flex;
  flex-direction: column;
  gap: var(--s5);
  background: linear-gradient(180deg, rgba(26,15,10,0.4), transparent);
}
.rail-mark {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 0.25em;
  color: var(--gold);
  text-transform: uppercase;
  padding-bottom: var(--s3);
  border-bottom: 1px solid var(--rule);
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.rail-mark .glyph { font-size: 16px; color: var(--copper); }
.rail-nav {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.rail-nav li a {
  display: flex;
  align-items: center;
  gap: var(--s3);
  padding: var(--s2) 0;
  color: var(--parchment-dim);
  text-decoration: none;
  font-family: 'Cormorant Garamond', serif;
  font-size: 16px;
  letter-spacing: 0.02em;
  transition: color 0.2s, padding-left 0.2s;
  border-left: 2px solid transparent;
  padding-left: var(--s3);
  margin-left: -2px;
}
.rail-nav li a:hover { color: var(--gold); padding-left: var(--s4); }
.rail-nav li a.active { color: var(--gold); border-left-color: var(--gold); }
.rail-nav .roman {
  font-family: 'Cinzel', serif;
  font-size: 10px;
  color: var(--parchment-faint);
  letter-spacing: 0.15em;
  min-width: 22px;
}
.rail-nav li a.active .roman { color: var(--copper); }
.rail-foot {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--parchment-faint);
  line-height: 1.6;
  padding-top: var(--s4);
  border-top: 1px solid var(--rule-faint);
}
.rail-foot .key { color: var(--parchment-dim); display: block; }
.rail-foot .val { color: var(--gold); }
main {
  padding: var(--s7) var(--s7) var(--s8);
  min-width: 0;
}
.banner {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: end;
  gap: var(--s5);
  padding-bottom: var(--s4);
  border-bottom: 1px solid var(--rule);
  margin-bottom: var(--s7);
}
.banner h1 {
  font-family: 'Cinzel', serif;
  font-size: clamp(36px, 5vw, 56px);
  font-weight: 700;
  letter-spacing: 0.18em;
  color: var(--gold);
  text-transform: uppercase;
  text-shadow: 0 1px 0 rgba(0,0,0,0.4), 0 0 32px rgba(202,138,4,0.18);
  line-height: 1;
}
.banner .h1-tail {
  display: block;
  font-family: 'Cormorant Garamond', serif;
  font-style: italic;
  font-weight: 400;
  font-size: 18px;
  color: var(--parchment-dim);
  letter-spacing: 0.04em;
  text-transform: none;
  margin-top: var(--s2);
  text-shadow: none;
}
.banner-meta {
  text-align: right;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: var(--parchment-faint);
  line-height: 1.7;
  letter-spacing: 0.05em;
}
.banner-meta .stamp {
  font-family: 'Cinzel', serif;
  letter-spacing: 0.2em;
  color: var(--copper);
  font-size: 10px;
  text-transform: uppercase;
  display: block;
  margin-bottom: var(--s2);
}
.hero {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: 1px;
  background: var(--rule);
  border: 1px solid var(--rule-strong);
  border-radius: var(--r-lg);
  margin-bottom: var(--s7);
  overflow: hidden;
  box-shadow: 0 12px 40px rgba(0,0,0,0.45), inset 0 1px 0 rgba(202,138,4,0.12);
}
.hero-main {
  background:
    radial-gradient(ellipse at top right, rgba(208,74,26,0.1), transparent 60%),
    var(--surface);
  padding: var(--s6) var(--s7);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: var(--s5);
  position: relative;
  min-height: 240px;
}
.hero-main::after {
  content: '';
  position: absolute;
  top: var(--s5); right: var(--s5);
  width: 72px; height: 72px;
  border: 1px solid var(--rule);
  border-radius: 50%;
  background: radial-gradient(circle at 35% 30%, rgba(234,179,8,0.18), transparent 60%);
}
.hero-main::before {
  content: '\2609';
  position: absolute;
  top: var(--s5); right: var(--s5);
  width: 72px; height: 72px;
  display: grid;
  place-items: center;
  font-size: 32px;
  color: var(--copper);
  z-index: 1;
  text-shadow: 0 0 12px rgba(208,74,26,0.4);
}
.hero-stamp {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 0.3em;
  color: var(--parchment-dim);
  text-transform: uppercase;
}
.hero-figure {
  font-family: 'Cinzel', serif;
  font-size: clamp(56px, 8vw, 96px);
  font-weight: 700;
  line-height: 0.9;
  color: var(--gold-bright);
  letter-spacing: -0.01em;
  text-shadow: 0 2px 24px rgba(234,179,8,0.18);
  font-feature-settings: "tnum";
}
.hero-figure .currency {
  font-size: 0.5em;
  color: var(--copper);
  vertical-align: 0.5em;
  margin-right: 0.08em;
  font-weight: 500;
}
.hero-figure .cents {
  font-size: 0.55em;
  color: var(--gold-deep);
  vertical-align: 0.18em;
}
.hero-caption {
  font-family: 'Cormorant Garamond', serif;
  font-style: italic;
  font-size: 17px;
  color: var(--parchment-dim);
  max-width: 36ch;
  line-height: 1.5;
}
.hero-caption strong {
  color: var(--ember);
  font-weight: 600;
  font-style: normal;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  padding: 0 4px;
}
.hero-side {
  background: var(--surface-2);
  display: grid;
  grid-template-rows: repeat(3, 1fr);
}
.hero-stat {
  padding: var(--s4) var(--s5);
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: var(--s3);
  border-bottom: 1px solid var(--rule-faint);
  position: relative;
}
.hero-stat:last-child { border-bottom: none; }
.hero-stat::before {
  content: '';
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--rule);
}
.hero-stat.danger::before { background: var(--ember); box-shadow: 0 0 12px rgba(208,74,26,0.4); }
.hero-stat.warn::before { background: var(--gold); }
.hero-stat .stat-label {
  font-family: 'Cinzel', serif;
  font-size: 10px;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--parchment-dim);
  display: block;
  margin-bottom: 4px;
}
.hero-stat .stat-sub {
  font-family: 'Cormorant Garamond', serif;
  font-style: italic;
  font-size: 13px;
  color: var(--parchment-faint);
}
.hero-stat .stat-value {
  font-family: 'Cinzel', serif;
  font-size: 30px;
  font-weight: 600;
  color: var(--gold);
  line-height: 1;
  font-feature-settings: "tnum";
}
.hero-stat.danger .stat-value { color: var(--crimson-glow); }
section {
  margin-bottom: var(--s7);
  scroll-margin-top: var(--s5);
}
.sec-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--s4);
  padding-bottom: var(--s3);
  margin-bottom: var(--s5);
  border-bottom: 1px solid var(--rule);
  position: relative;
}
.sec-head::after {
  content: '';
  position: absolute;
  bottom: -3px;
  left: 0;
  width: 60px;
  height: 1px;
  background: var(--gold);
}
.sec-head .roman {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 0.3em;
  color: var(--copper);
  display: block;
  margin-bottom: var(--s1);
  text-transform: uppercase;
}
.sec-head h2 {
  font-family: 'Cinzel', serif;
  font-size: 22px;
  font-weight: 600;
  color: var(--parchment);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  line-height: 1;
}
.sec-head .sec-tail {
  font-family: 'Cormorant Garamond', serif;
  font-style: italic;
  font-size: 14px;
  color: var(--parchment-faint);
  text-align: right;
  max-width: 40ch;
}
.panel {
  background:
    linear-gradient(180deg, rgba(202,138,4,0.04), transparent 30%),
    var(--surface);
  border: 1px solid var(--rule);
  border-radius: var(--r-lg);
  padding: var(--s5);
  position: relative;
  box-shadow: 0 4px 18px rgba(0,0,0,0.35);
}
.panel.flush { padding: 0; overflow: hidden; }
.panel-title {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 0.22em;
  color: var(--parchment-dim);
  text-transform: uppercase;
  margin-bottom: var(--s4);
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
.panel-title .panel-meta {
  color: var(--parchment-faint);
  font-family: 'JetBrains Mono', monospace;
  letter-spacing: 0.05em;
  font-size: 10px;
}
.bento-flow {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: var(--s4);
}
.chart-host { position: relative; height: 320px; }
.chart-host.tall { height: 360px; }
.chart-host.donut { height: 240px; display: grid; place-items: center; }
canvas { width: 100% !important; height: 100% !important; }
.donut-legend {
  margin-top: var(--s4);
  padding-top: var(--s4);
  border-top: 1px dashed var(--rule-faint);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
}
.donut-row {
  display: grid;
  grid-template-columns: 14px 1fr auto auto;
  align-items: center;
  gap: var(--s3);
  font-size: 13px;
  padding: 4px 0;
}
.donut-row .swatch { width: 10px; height: 10px; border-radius: 1px; transform: rotate(45deg); margin: 2px; }
.donut-row .name { color: var(--parchment); font-family: 'JetBrains Mono', monospace; font-size: 11px; }
.donut-row .cost { font-family: 'JetBrains Mono', monospace; color: var(--gold); font-size: 12px; }
.donut-row .pct { font-family: 'Cinzel', serif; font-size: 10px; color: var(--parchment-faint); letter-spacing: 0.1em; min-width: 36px; text-align: right; }
.bento-pair {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: var(--s4);
}
.leaderboard { display: flex; flex-direction: column; gap: var(--s2); }
.leaderboard-scroll { max-height: 380px; overflow-y: auto; }
.file-filter { display: flex; gap: 6px; margin-bottom: var(--s3); }
.file-filter button {
  background: var(--surface-2); border: 1px solid var(--rule);
  color: var(--parchment-dim); font-family: 'Cinzel', serif;
  font-size: 10px; padding: 4px 12px; border-radius: var(--r-sm);
  cursor: pointer; letter-spacing: 0.1em; transition: all 0.15s;
}
.file-filter button.active,
.file-filter button:hover { background: var(--surface-3); border-color: var(--gold); color: var(--gold); }
.lb-row {
  display: grid;
  grid-template-columns: 32px 1fr auto;
  align-items: center;
  gap: var(--s3);
  padding: var(--s3) var(--s2);
  border-bottom: 1px dashed var(--rule-faint);
  position: relative;
  transition: background 0.2s;
}
.lb-row:hover { background: rgba(202,138,4,0.04); }
.lb-row::before {
  content: '';
  position: absolute;
  left: 0; bottom: 0;
  height: 2px;
  background: linear-gradient(90deg, var(--gold-deep), var(--gold), transparent);
  width: var(--bar, 0%);
  opacity: 0.5;
}
.lb-rank {
  font-family: 'Cinzel', serif;
  font-size: 14px;
  color: var(--copper);
  letter-spacing: 0.05em;
  text-align: center;
}
.lb-rank.top { color: var(--gold-bright); font-size: 16px; text-shadow: 0 0 10px rgba(234,179,8,0.4); }
.lb-name {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: var(--parchment);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lb-name .dim { color: var(--parchment-faint); }
.lb-meta {
  text-align: right;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
}
.lb-meta .reads { color: var(--ember); font-weight: 600; }
.lb-meta .cost { color: var(--gold); display: block; font-size: 10px; opacity: 0.85; }
.tree-host { height: 380px; position: relative; }
.tree-placeholder {
  display: none;
  height: 100%;
  justify-content: center;
  align-items: center;
  color: var(--parchment-faint);
  font-family: 'Cinzel', serif;
  font-size: 14px;
  letter-spacing: 0.12em;
}
.no-tree .tree-placeholder { display: flex; }
.no-tree canvas { display: none !important; }
.ledger-table {
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: var(--r-lg);
  overflow: hidden;
}
.ledger-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--s4);
  padding: var(--s4) var(--s5);
  border-bottom: 1px solid var(--rule);
  background: linear-gradient(180deg, rgba(202,138,4,0.05), transparent);
}
.ledger-tally {
  display: flex;
  gap: var(--s5);
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}
.ledger-tally span { color: var(--parchment-dim); }
.ledger-tally span b {
  color: var(--gold);
  font-family: 'JetBrains Mono', monospace;
  font-weight: 500;
  margin-left: 6px;
}
.ledger-tally .high b { color: var(--crimson-glow); }
.ledger-tally .med b { color: var(--gold); }
.ledger-tally .low b { color: var(--indigo-glow); }
.ledger-search {
  background: var(--surface-2);
  border: 1px solid var(--rule);
  color: var(--parchment);
  font-family: 'Spectral', serif;
  font-size: 13px;
  padding: 6px 10px;
  border-radius: var(--r);
  width: 220px;
  outline: none;
}
.ledger-search::placeholder { color: var(--parchment-faint); font-style: italic; }
.ledger-search:focus { border-color: var(--gold); box-shadow: 0 0 0 2px rgba(202,138,4,0.15); }
table.ledger {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
table.ledger th {
  font-family: 'Cinzel', serif;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  text-align: left;
  padding: var(--s3) var(--s4);
  color: var(--parchment-dim);
  background: var(--surface-2);
  border-bottom: 1px solid var(--rule);
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
}
table.ledger th:hover { color: var(--gold); }
table.ledger th .arrow { color: var(--copper); margin-left: 4px; opacity: 0.6; font-size: 9px; }
table.ledger td {
  padding: var(--s3) var(--s4);
  border-bottom: 1px solid var(--rule-faint);
  vertical-align: middle;
}
table.ledger tbody tr:last-child td { border-bottom: none; }
table.ledger tbody tr:hover td { background: rgba(202,138,4,0.04); }
.sev {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: 'Cinzel', serif;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  padding: 4px 10px;
  border-radius: var(--r-sm);
}
.sev::before { content: ''; width: 6px; height: 6px; border-radius: 50%; }
.sev.high   { background: rgba(185,28,28,0.15);  color: var(--crimson-glow); }
.sev.high::before   { background: var(--crimson-glow); box-shadow: 0 0 8px rgba(248,113,113,0.5); }
.sev.medium { background: rgba(202,138,4,0.15);  color: var(--gold-bright); }
.sev.medium::before { background: var(--gold-bright); }
.sev.low    { background: rgba(109,40,217,0.15); color: var(--indigo-glow); }
.sev.low::before    { background: var(--indigo-glow); }
.sid { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--parchment-dim); }
.sid .pre { color: var(--copper); }
.proj { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--parchment); }
.reason {
  font-family: 'Cormorant Garamond', serif;
  font-size: 14px;
  color: var(--parchment);
  letter-spacing: 0.02em;
}
.reason small {
  display: block;
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--parchment-faint);
  letter-spacing: 0.05em;
  margin-top: 2px;
  text-transform: uppercase;
}
.cost-cell {
  font-family: 'Cinzel', serif;
  font-size: 14px;
  color: var(--gold);
  text-align: right;
  font-feature-settings: "tnum";
}
.cost-cell .cur { font-size: 11px; color: var(--copper); margin-right: 2px; }
.expand-btn {
  background: transparent;
  border: 1px solid var(--rule);
  color: var(--gold);
  font-family: 'Cinzel', serif;
  font-size: 11px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: var(--r-sm);
  letter-spacing: 0.12em;
  transition: all 0.15s;
}
.expand-btn:hover { background: var(--surface-2); border-color: var(--gold); color: var(--gold-bright); }
.expand-row { display: none; }
.expand-row.open { display: table-row; }
.expand-cell {
  padding: 0 !important;
  background: var(--surface-2);
  border-bottom: 1px solid rgba(202,138,4,0.15);
}
.expand-inner {
  padding: var(--s4);
  font-size: 13px;
}
.expand-inner h4 {
  font-family: 'Cinzel', serif;
  color: var(--gold);
  font-size: 16px;
  margin-bottom: var(--s3);
}
.timeline-event {
  display: flex;
  align-items: flex-start;
  gap: var(--s2);
  padding: 4px 0;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  border-bottom: 1px solid var(--rule-faint);
  color: var(--parchment);
}
.timeline-event .idx { color: var(--parchment-faint); min-width: 40px; }
.timeline-event .content { flex: 1; }
.timeline-event .annotation { color: var(--crimson-glow); font-style: italic; }
.timeline-event.subagent { padding-left: var(--s5); }
.timeline-event .cost { color: var(--gold); text-align: right; min-width: 60px; }
footer {
  margin-top: var(--s7);
  padding: var(--s5) 0;
  border-top: 1px solid var(--rule);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-family: 'Cinzel', serif;
  font-size: 10px;
  letter-spacing: 0.25em;
  text-transform: uppercase;
  color: var(--parchment-faint);
}
footer .sigil { color: var(--copper); font-size: 14px; }
@media (max-width: 1100px) {
  .shell { grid-template-columns: 1fr; }
  .rail { display: none; }
  main { padding: var(--s5); }
  .hero { grid-template-columns: 1fr; }
  .bento-flow, .bento-pair { grid-template-columns: 1fr; }
}`
}

func renderSidebar(version string, generated time.Time, s reportSummary) string {
	var b strings.Builder
	b.WriteString(`<aside class="rail">
<div class="rail-mark">
<span>☉ Burnwatch</span>
<span class="glyph">⚝</span>
</div>
<ul class="rail-nav">
<li><a href="#hero" class="active"><span class="roman">I</span>Ledger</a></li>
<li><a href="#flow"><span class="roman">II</span>Flow of Coin</a></li>
<li><a href="#waste"><span class="roman">III</span>Waste by Form</a></li>
<li><a href="#tree"><span class="roman">IV</span>Subagent Tree</a></li>
<li><a href="#signals"><span class="roman">V</span>Signals</a></li>
</ul>
<div class="rail-foot">
`)
	fmt.Fprintf(&b, `<span class="key">version</span><span class="val">%s</span><br>
`, version)
	fmt.Fprintf(&b, `<span class="key">generated</span><span class="val">%s</span><br>
`, generated.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&b, `<span class="key">scope</span><span class="val">all harnesses · %dd</span>
`, s.DayCount)
	b.WriteString(`</div>
</aside>
`)
	return b.String()
}

func renderBanner(s reportSummary) string {
	var b strings.Builder
	b.WriteString(`<header class="banner">
<div>
<h1>burnwatch</h1>
<span class="h1-tail">An accounting of token spent in the work of agents.</span>
</div>
<div class="banner-meta">
<span class="stamp">Folio · MMXXVI</span>
`)
	fmt.Fprintf(&b, "%s → %s<br>\n", s.DateFrom, s.DateTo)
	b.WriteString(`ledger sealed at the close of day
</div>
</header>
`)
	return b.String()
}

func renderHero(s reportSummary) string {
	costStr := fmt.Sprintf("%.2f", s.TotalCost)
	parts := strings.SplitN(costStr, ".", 2)
	dollars := parts[0]
	cents := "." + parts[1]

	tracedStr := fmt.Sprintf("%.2f", s.TracedCost)

	avgDaily := 0.0
	if s.DayCount > 0 {
		avgDaily = s.TotalCost / float64(s.DayCount)
	}

	var b strings.Builder
	b.WriteString(`<section id="hero">
<div class="hero">
<div class="hero-main">
<span class="hero-stamp">Total Burned · `)
	fmt.Fprintf(&b, "%d", s.DayCount)
	b.WriteString(` Days</span>
<div class="hero-figure">
<span class="currency">$</span>`)
	b.WriteString(dollars)
	b.WriteString(`<span class="cents">`)
	b.WriteString(cents)
	b.WriteString(`</span>
</div>
<p class="hero-caption">
Of that sum, <strong>$`)
	b.WriteString(tracedStr)
	b.WriteString(`</strong> traced to identifiable waste —
tool loops, file re-reads, and idle subagents recorded across <strong>`)
	fmt.Fprintf(&b, "%d", s.ProjectCount)
	b.WriteString(`</strong> projects.
</p>
</div>
<div class="hero-side">
<div class="hero-stat danger">
<div>
<span class="stat-label">Waste Signals</span>
<span class="stat-sub">`)
	fmt.Fprintf(&b, "%d high &middot; %d medium &middot; %d low", s.HighSignals, s.MediumSignals, s.LowSignals)
	b.WriteString(`</span>
</div>
<div class="stat-value">`)
	fmt.Fprintf(&b, "%d", s.TotalSignals)
	b.WriteString(`</div>
</div>
<div class="hero-stat warn">
<div>
<span class="stat-label">Sessions</span>
<span class="stat-sub">across `)
	fmt.Fprintf(&b, "%d", s.ProjectCount)
	b.WriteString(` projects</span>
</div>
<div class="stat-value">`)
	fmt.Fprintf(&b, "%d", s.Sessions)
	b.WriteString(`</div>
</div>
<div class="hero-stat">
<div>
<span class="stat-label">Avg. Daily Burn</span>
<span class="stat-sub">$`)
	fmt.Fprintf(&b, "%.2f", avgDaily)
	fmt.Fprintf(&b, " over %d", s.DayCount)
	b.WriteString(` days</span>
</div>
<div class="stat-value">$`)
	fmt.Fprintf(&b, "%.0f", avgDaily)
	b.WriteString(`</div>
</div>
</div>
</div>
</section>
`)
	return b.String()
}

func renderFlowSection() string {
	return `<section id="flow">
<div class="sec-head">
<div>
<span class="roman">II ·</span>
<h2>Flow of Coin</h2>
</div>
<p class="sec-tail">Daily expenditure with a seven-day moving average, alongside the model that drew the heaviest purse.</p>
</div>
<div class="bento-flow">
<div class="panel">
<div class="panel-title">Cost Over Time <span class="panel-meta">USD · daily</span></div>
<div class="chart-host"><canvas id="cFlow"></canvas></div>
</div>
<div class="panel">
<div class="panel-title">Model Breakdown <span class="panel-meta">share of spend</span></div>
<div class="chart-host donut"><canvas id="cDonut"></canvas></div>
<div class="donut-legend" id="donutLegend"></div>
</div>
</div>
</section>
`
}

func renderWasteSection() string {
	return `<section id="waste">
<div class="sec-head">
<div>
<span class="roman">III ·</span>
<h2>Waste by Form</h2>
</div>
<p class="sec-tail">Stacked spend per project against the most re-traversed files in the catalogue.</p>
</div>
<div class="bento-pair">
<div class="panel">
<div class="panel-title">Waste by Signal Type <span class="panel-meta">stacked · per project</span></div>
<div class="chart-host"><canvas id="cWaste"></canvas></div>
</div>
<div class="panel">
<div class="panel-title">Most Re-Read Files <span class="panel-meta">top files</span></div>
<div class="file-filter">
  <button data-n="3">Top 3</button>
  <button data-n="10" class="active">Top 10</button>
  <button data-n="15">All</button>
</div>
<div class="leaderboard-scroll">
  <div class="leaderboard" id="leaderboard"></div>
</div>
</div>
</div>
</section>
`
}

func renderTreeSection() string {
	return `<section id="tree">
<div class="sec-head">
<div>
<span class="roman">IV ·</span>
<h2>Subagent Tree</h2>
</div>
<p class="sec-tail">Costs of delegated work, arranged by the parent that summoned them.</p>
</div>
<div class="panel">
<div class="panel-title">Subagent Cost Tree <span class="panel-meta">treemap · slice &amp; dice</span></div>
<div class="tree-host" id="treeHost">
<div class="tree-placeholder">No subagents detected</div>
<canvas id="cTree"></canvas>
</div>
</div>
</section>
`
}

func renderSignalsSection(s reportSummary) string {
	var b strings.Builder
	b.WriteString(`<section id="signals">
<div class="sec-head">
<div>
<span class="roman">V ·</span>
<h2>Signals Ledger</h2>
</div>
<p class="sec-tail">Every flagged session, ranked by severity then by cost. Click a row to read the annotated timeline.</p>
</div>
<div class="ledger-table">
<div class="ledger-controls">
<div class="ledger-tally">
<span class="high">High <b>`)
	fmt.Fprintf(&b, "%d", s.HighSignals)
	b.WriteString(`</b></span>
<span class="med">Medium <b>`)
	fmt.Fprintf(&b, "%d", s.MediumSignals)
	b.WriteString(`</b></span>
<span class="low">Low <b>`)
	fmt.Fprintf(&b, "%d", s.LowSignals)
	b.WriteString(`</b></span>
<span>Total <b>`)
	fmt.Fprintf(&b, "%d", s.TotalSignals)
	b.WriteString(`</b></span>
</div>
<input class="ledger-search" placeholder="filter by project or session…">
</div>
<table class="ledger">
<thead>
<tr>
<th data-sort="severity">Severity <span class="arrow">▾</span></th>
<th data-sort="session">Session</th>
<th data-sort="project">Project</th>
<th data-sort="reason">Reason</th>
<th data-sort="cost" style="text-align:right">Cost <span class="arrow">▾</span></th>
<th></th>
</tr>
</thead>
<tbody></tbody>
</table>
</div>
</section>
`)
	return b.String()
}

func renderFooter(version, generated string) string {
	ts, _ := time.Parse(time.RFC3339, generated)
	tsStr := generated
	if !ts.IsZero() {
		tsStr = ts.Format("2006-01-02")
	}
	ver := strings.TrimPrefix(version, "v")
	var b strings.Builder
	b.WriteString(`<footer>
`)
	fmt.Fprintf(&b, `<span>burnwatch v%s · sealed %s</span>
<span class="sigil">☉ ✶ ☽</span>
<span>Folium MMXXVI</span>
`, ver, tsStr)
	b.WriteString(`</footer>
`)
	return b.String()
}

func renderJS() string {
	return `
(function(){
  if (typeof Chart === 'undefined') { buildTable(); setupRailTracking(); return; }

  const gold       = '#ca8a04';
  const goldBright = '#eab308';
  const copper     = '#b8470a';
  const ember      = '#d04a1a';
  const crimson    = '#b91c1c';
  const indigo     = '#6d28d9';
  const moss       = '#4d7c2a';
  const parchment  = '#f5e6d3';
  const dim        = '#bfa98a';
  const faint      = '#8a7659';
  const rule       = 'rgba(202,138,4,0.18)';
  const surface    = '#2c1a10';
  const palette    = [gold, copper, indigo, moss, ember, '#92670a', '#a78bfa', '#84cc16'];

  Chart.defaults.color = dim;
  Chart.defaults.borderColor = rule;
  Chart.defaults.font.family = 'JetBrains Mono, monospace';
  Chart.defaults.font.size = 10;

  buildCostOverTime();
  buildModelDonut();
  buildWasteByType();
  buildLeaderboard();
  buildTreemap();
  buildTable();
  setupRailTracking();
  setupSearchFilter();

  window.addEventListener('resize', function(){
    Object.values(Chart.instances).forEach(function(c){ c.resize(); });
    drawTree();
  });

  function buildCostOverTime() {
    var data = REPORT.costOverTime || [];
    if (!data.length) return;
    var ctx = document.getElementById('cFlow').getContext('2d');
    new Chart(ctx, {
      type: 'line',
      data: {
        labels: data.map(function(d){ return d.date; }),
        datasets: [
          {
            label: 'Daily',
            data: data.map(function(d){ return d.cost; }),
            borderColor: gold,
            backgroundColor: 'rgba(202,138,4,0.10)',
            fill: true, tension: 0.32,
            pointRadius: 3, pointHoverRadius: 5,
            pointBackgroundColor: goldBright,
            borderWidth: 2
          },
          {
            label: '7-day Avg',
            data: data.map(function(d){ return d.movingAvg; }),
            borderColor: copper,
            borderDash: [4, 4], borderWidth: 1.5,
            pointRadius: 3, fill: false, tension: 0.32
          }
        ]
      },
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: { labels: { usePointStyle: true, padding: 16, font: { family: 'Cinzel', size: 10 }, color: dim } },
          tooltip: {
            backgroundColor: surface, borderColor: rule, borderWidth: 1,
            titleFont: { family: 'Cinzel', size: 11 }, bodyFont: { family: 'JetBrains Mono', size: 11 },
            callbacks: { label: function(c){ return '  $' + c.raw.toFixed(2); } }
          }
        },
        scales: {
          x: { grid: { color: rule, drawTicks: false }, ticks: { maxTicksLimit: 10, color: faint } },
          y: { grid: { color: rule }, ticks: { callback: function(v){ return '$'+v; }, color: faint } }
        }
      }
    });
  }

  function buildModelDonut() {
    var data = REPORT.modelBreakdown || [];
    if (!data.length) return;
    var ctx = document.getElementById('cDonut').getContext('2d');
    var legend = document.getElementById('donutLegend');

    new Chart(ctx, {
      type: 'doughnut',
      data: {
        labels: data.map(function(d){ return d.model; }),
        datasets: [{
          data: data.map(function(d){ return d.cost; }),
          backgroundColor: palette,
          borderColor: surface, borderWidth: 3
        }]
      },
      options: {
        responsive: true, maintainAspectRatio: false,
        cutout: '64%',
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: surface, borderColor: rule, borderWidth: 1,
            titleFont: { family: 'Cinzel', size: 11 }, bodyFont: { family: 'JetBrains Mono', size: 11 },
            callbacks: { label: function(c){ return '  $' + c.raw.toFixed(2); } }
          }
        }
      }
    });

    data.forEach(function(m, i){
      var row = document.createElement('div');
      row.className = 'donut-row';
      row.innerHTML = '<span class="swatch" style="background:' + palette[i % palette.length] + '"></span>' +
        '<span class="name">' + escHtml(m.model) + '</span>' +
        '<span class="cost">$' + m.cost.toFixed(2) + '</span>' +
        '<span class="pct">' + m.percentage.toFixed(1) + '%</span>';
      legend.appendChild(row);
    });
  }

  function buildWasteByType() {
    var data = REPORT.wasteByType || [];
    if (!data.length) return;
    var ctx = document.getElementById('cWaste').getContext('2d');

    var allReasons = {};
    data.forEach(function(p){
      Object.keys(p.reasons || {}).forEach(function(r){ allReasons[r] = true; });
    });
    var reasonKeys = Object.keys(allReasons).sort();

    var reasonColors = {
      tool_call_loop: crimson,
      file_reread: gold,
      subagent_overlap: indigo,
      session_restart: moss,
      cache_underutilized: copper,
      subagent_overhead: '#a78bfa',
      cost_outlier: ember,
      fragmentation_index: '#ef4444',
      input_overconsumption: '#ff6b35',
      output_explosion: '#e84d8a',
      low_token_efficiency: '#36a2eb',
      low_signal: dim
    };

    var projects = data.map(function(d){ return d.project; });
    var datasets = reasonKeys.map(function(reason, i){
      return {
        label: reason.replace(/_/g, ' '),
        data: data.map(function(p){ return (p.reasons && p.reasons[reason]) || 0; }),
        backgroundColor: reasonColors[reason] || palette[i % palette.length],
        borderWidth: 0
      };
    });

    new Chart(ctx, {
      type: 'bar',
      data: { labels: projects, datasets: datasets },
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: { labels: { usePointStyle: true, padding: 12, font: { family: 'Cinzel', size: 10 }, color: dim } },
          tooltip: {
            backgroundColor: surface, borderColor: rule, borderWidth: 1,
            titleFont: { family: 'Cinzel', size: 11 }, bodyFont: { family: 'JetBrains Mono', size: 11 },
            callbacks: { label: function(c){ return '  ' + c.dataset.label + ': $' + c.raw.toFixed(2); } }
          }
        },
        scales: {
          x: { stacked: true, grid: { display: false }, ticks: { color: dim, font: { family: 'Cinzel', size: 10, weight: '500' } } },
          y: { stacked: true, grid: { color: rule }, ticks: { callback: function(v){ return '$'+v; }, color: faint } }
        }
      }
    });
  }

  function buildLeaderboard() {
    var data = REPORT.topFiles || [];
    if (!data.length) return;
    var currentN = 10;

    function render(n) {
      var container = document.getElementById('leaderboard');
      container.innerHTML = '';
      var maxReads = data[0] ? data[0].readCount : 1;
      var roman = ['I','II','III','IV','V','VI','VII','VIII','IX','X','XI','XII','XIII','XIV','XV'];
      var limit = Math.min(n, data.length, roman.length);
      for (var i = 0; i < limit; i++) {
        var d = data[i];
        var barPct = Math.round((d.readCount / maxReads) * 100);
        var row = document.createElement('div');
        row.className = 'lb-row';
        row.style.setProperty('--bar', barPct + '%');
        var rankCls = i === 0 ? 'lb-rank top' : 'lb-rank';
        var label = formatFileLabel(d.path);
        row.innerHTML = '<span class="' + rankCls + '">' + roman[i] + '</span>' +
          '<span class="lb-name">' + label + '</span>' +
          '<span class="lb-meta"><span class="reads">' + d.readCount + '\u00d7</span>' +
          '<span class="cost">$' + d.cost.toFixed(2) + '</span></span>';
        container.appendChild(row);
      }
    }

    render(currentN);

    var buttons = document.querySelectorAll('.file-filter button');
    for (var b = 0; b < buttons.length; b++) {
      buttons[b].addEventListener('click', function(e){
        var n = parseInt(this.getAttribute('data-n'));
        currentN = n;
        render(n);
        buttons.forEach(function(bt){ bt.classList.remove('active'); });
        this.classList.add('active');
      });
    }
  }

  function formatFileLabel(path) {
    var parts = path.split('/');
    if (parts.length <= 1) return escHtml(path);
    if (parts.length === 2) return escHtml(parts[0]) + '/<span class="dim">' + escHtml(parts[1]) + '</span>';
    var first = escHtml(parts[0]) + '/';
    var middle = parts.slice(1, -1).map(escHtml).join('/') + '/';
    var last = escHtml(parts[parts.length - 1]);
    return first + '<span class="dim">' + middle + '</span>' + last;
  }

  function buildTreemap() {
    var treeData = REPORT.subagentTree;
    if (!treeData || !treeData.children || !treeData.children.length) {
      document.getElementById('treeHost').classList.add('no-tree');
      return;
    }
    drawTree();
  }

  function drawTree() {
    var canv = document.getElementById('cTree');
    var host = document.getElementById('treeHost');
    var W = host.clientWidth;
    var H = host.clientHeight || 380;
    var ratio = window.devicePixelRatio || 1;
    canv.width = W * ratio; canv.height = H * ratio;
    var ctx = canv.getContext('2d');
    ctx.scale(ratio, ratio);

    var items = [];
    function flatten(n, parentName) {
      if (!n) return;
      if (!n.children || !n.children.length) {
        if (n.value > 0) items.push({ name: n.name, v: n.value, parent: parentName || n.name });
      } else {
        n.children.forEach(function(c){ flatten(c, n.name); });
      }
    }
    flatten(REPORT.subagentTree, '');
    if (!items.length) {
      document.getElementById('treeHost').classList.add('no-tree');
      return;
    }
    document.getElementById('treeHost').classList.remove('no-tree');

    items.sort(function(a,b){ return b.v - a.v; });
    var total = items.reduce(function(s,n){ return s + n.v; }, 0);
    if (!total) return;

    var usedParents = [];
    items.forEach(function(n){
      if (usedParents.indexOf(n.parent) < 0) usedParents.push(n.parent);
    });

    function squarify(arr, x, y, w, h) {
      if (!arr.length) return;
      var sum = arr.reduce(function(s,i){ return s + i.v; }, 0);
      if (w >= h) {
        var cx = x;
        arr.forEach(function(i){
          var iw = (i.v / sum) * w;
          drawCell(i, cx, y, iw, h);
          cx += iw;
        });
      } else {
        var cy = y;
        arr.forEach(function(i){
          var ih = (i.v / sum) * h;
          drawCell(i, x, cy, w, ih);
          cy += ih;
        });
      }
    }

    function drawCell(n, x, y, w, h) {
      var colorIdx = usedParents.indexOf(n.parent);
      var col = palette[colorIdx % palette.length];
      var grad = ctx.createLinearGradient(x, y, x+w, y+h);
      grad.addColorStop(0, col);
      grad.addColorStop(1, shade(col, -0.25));
      ctx.fillStyle = grad;
      ctx.fillRect(x+1, y+1, Math.max(0,w-2), Math.max(0,h-2));
      ctx.strokeStyle = 'rgba(26,15,10,0.5)';
      ctx.lineWidth = 1;
      ctx.strokeRect(x+1, y+1, Math.max(0,w-2), Math.max(0,h-2));
      if (w > 70 && h > 32) {
        ctx.fillStyle = parchment;
        ctx.font = '500 11px "JetBrains Mono", monospace';
        ctx.fillText(truncate(n.name, Math.max(4, Math.floor(w / 7))), x + 8, y + 18);
        ctx.fillStyle = 'rgba(245,230,211,0.65)';
        ctx.font = '10px "Cinzel", serif';
        ctx.fillText('$' + n.v.toFixed(2), x + 8, y + 32);
        if (h > 50) {
          ctx.fillStyle = 'rgba(245,230,211,0.4)';
          ctx.font = 'italic 10px "Cormorant Garamond", serif';
          ctx.fillText(n.parent, x + 8, y + 46);
        }
      }
    }

    function shade(hex, pct) {
      var r = parseInt(hex.slice(1,3),16), g = parseInt(hex.slice(3,5),16), b = parseInt(hex.slice(5,7),16);
      var f = pct < 0 ? (1+pct) : (1-pct);
      return 'rgb(' + (pct<0?Math.round(r*f):Math.round(r*f+255*pct)) + ',' +
        (pct<0?Math.round(g*f):Math.round(g*f+255*pct)) + ',' +
        (pct<0?Math.round(b*f):Math.round(b*f+255*pct)) + ')';
    }

    function truncate(s, max) {
      if (s.length <= max) return s;
      return s.slice(0, max-1) + '\u2026';
    }

    squarify(items, 0, 0, W, H);
  }

  function buildTable() {
    var signals = REPORT.signals || [];
    var timelines = REPORT.signalTimelines || {};
    var tbody = document.querySelector('table.ledger tbody');
    if (!tbody) return;

    signals.forEach(function(s, i){
      var reasonLabel = s.reason.replace(/_/g, ' ');
      var reasonCap = reasonLabel.charAt(0).toUpperCase() + reasonLabel.slice(1);
      var detailText = escHtml(s.detail);

      var row = document.createElement('tr');
      row.innerHTML =
        '<td><span class="sev ' + escHtml(s.severity) + '">' + escHtml(s.severity) + '</span></td>' +
        '<td><span class="sid">' + escHtml(s.sessionId) + '</span></td>' +
        '<td><span class="proj">' + escHtml(s.project) + '</span></td>' +
        '<td><span class="reason">' + reasonCap + ' <small>' + detailText + '</small></span></td>' +
        '<td class="cost-cell"><span class="cur">$</span>' + s.cost.toFixed(2) + '</td>' +
        '<td><button class="expand-btn">View ▾</button></td>';
      tbody.appendChild(row);

      var tl = timelines[s.sessionId];
      if (tl) {
        var exRow = document.createElement('tr');
        exRow.className = 'expand-row';
        exRow.innerHTML = '<td colspan="6" class="expand-cell"><div class="expand-inner">' +
          '<h4>Session ' + escHtml(s.sessionId) + '</h4>' +
          buildTimelineHTML(tl) +
          '</div></td>';
        tbody.appendChild(exRow);
      }
    });

    document.querySelectorAll('table.ledger th[data-sort]').forEach(function(th){
      th.addEventListener('click', function(){ sortTable(this.dataset.sort); });
    });

    document.querySelectorAll('.expand-btn').forEach(function(btn){
      btn.addEventListener('click', function(){
        var row = this.closest('tr');
        var next = row.nextElementSibling;
        while (next && next.classList.contains('expand-row')) {
          next.classList.toggle('open');
          this.textContent = next.classList.contains('open') ? '▴' : 'View ▾';
          break;
        }
      }.bind(btn));
    });
  }

  function buildTimelineHTML(tl) {
    var html = '';
    tl.forEach(function(e){
      var cls = e.isSubagent ? ' timeline-event subagent' : 'timeline-event';
      html += '<div class="' + cls + '">';
      html += '<span class="idx">#' + e.index + '</span>';
      html += '<span class="content">';
      if (e.toolCalls && e.toolCalls.length) {
        e.toolCalls.forEach(function(tc){ html += '<div>' + escHtml(tc) + '</div>'; });
      }
      if (e.fileOps && e.fileOps.length) {
        e.fileOps.forEach(function(fo){ html += '<div style="color:' + dim + '">' + escHtml(fo) + '</div>'; });
      }
      if (e.annotations && e.annotations.length) {
        e.annotations.forEach(function(a){ html += '<div class="annotation">' + escHtml(a) + '</div>'; });
      }
      html += '</span>';
      html += '<span class="cost">$' + e.cost.toFixed(2) + '</span>';
      html += '</div>';
    });
    return html;
  }

  function sortTable(key) {
    var tbody = document.querySelector('table.ledger tbody');
    var rows = Array.from(tbody.querySelectorAll('tr:not(.expand-row)'));
    var dataRows = [];
    rows.forEach(function(r){
      if (r.classList.contains('expand-row')) return;
      dataRows.push(r);
    });

    var currentDir = tbody.dataset.sortDir === 'desc' ? 'desc' : 'asc';
    var nextDir = currentDir === 'asc' ? 'desc' : 'asc';
    tbody.dataset.sortDir = nextDir;
    var dir = currentDir === 'asc' ? 1 : -1;

    dataRows.sort(function(a, b){
      var aval, bval;
      if (key === 'severity') {
        aval = ['high','medium','low'].indexOf(a.cells[0].textContent.trim().toLowerCase());
        bval = ['high','medium','low'].indexOf(b.cells[0].textContent.trim().toLowerCase());
      } else if (key === 'cost') {
        aval = parseFloat(a.cells[4].textContent.replace('$',''));
        bval = parseFloat(b.cells[4].textContent.replace('$',''));
      } else {
        var colMap = { session:1, project:2, reason:3 };
        aval = a.cells[colMap[key] || 0].textContent;
        bval = b.cells[colMap[key] || 0].textContent;
        if (typeof aval === 'string') return dir * aval.localeCompare(bval);
      }
      return dir * (aval > bval ? 1 : aval < bval ? -1 : 0);
    });

    dataRows.forEach(function(r){
      var next = r.nextElementSibling;
      tbody.appendChild(r);
      if (next && next.classList.contains('expand-row')) {
        tbody.appendChild(next);
      }
    });

    document.querySelectorAll('table.ledger th .arrow').forEach(function(s){ s.textContent = ''; });
    var th = document.querySelector('table.ledger th[data-sort="' + key + '"] .arrow');
    if (th) th.textContent = nextDir === 'desc' ? '▾' : '▴';
  }

  function setupSearchFilter() {
    var input = document.querySelector('.ledger-search');
    if (!input) return;
    input.addEventListener('input', function(e){
      var q = e.target.value.toLowerCase();
      var rows = document.querySelectorAll('table.ledger tbody tr');
      rows.forEach(function(r){
        if (r.classList.contains('expand-row')) return;
        var text = r.textContent.toLowerCase();
        var shouldShow = !q || text.indexOf(q) >= 0;
        r.style.display = shouldShow ? '' : 'none';
        var next = r.nextElementSibling;
        if (next && next.classList.contains('expand-row')) {
          next.style.display = shouldShow && next.classList.contains('open') ? '' : 'none';
        }
      });
    });
  }

  function setupRailTracking() {
    var links = document.querySelectorAll('.rail-nav a');
    var sections = Array.from(links).map(function(a){ return document.querySelector(a.getAttribute('href')); });
    function onScroll() {
      var y = window.scrollY + 120;
      var active = 0;
      sections.forEach(function(s, i){ if (s && s.offsetTop <= y) active = i; });
      links.forEach(function(l, i){ l.classList.toggle('active', i === active); });
    }
    window.addEventListener('scroll', onScroll, { passive: true });
  }

  function escHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }
})();
`
}

func FormatReportJSON(events []source.TokenEvent, baselines map[string]analyze.Baseline, signals []analyze.WasteSignal, trees []analyze.SubagentTree, version string, generated time.Time) ([]byte, error) {
	data := computeReportData(events, baselines, signals, trees)
	data.Version = version
	data.Generated = generated.Format(time.RFC3339)
	return json.Marshal(data)
}
