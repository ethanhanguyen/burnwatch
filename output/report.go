package output

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
<link href="https://fonts.googleapis.com/css2?family=Cinzel:wght@400;600;700&family=Spectral:ital,wght@0,400;0,500;0,600;1,400&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<style>
:root {
  --gold: #CA8A04;
  --gold-light: #DAA520;
  --gold-dim: #B8780A;
  --red: #991B1B;
  --red-light: #F87171;
  --purple: #581C87;
  --purple-light: #7C3AED;
  --green: #22C55E;
  --bg: #1A0F0A;
  --surface: #2C1A10;
  --surface-elevated: #3D2517;
  --border: #5C3D2E;
  --border-dim: #3D2517;
  --text: #F5E6D3;
  --text-dim: #BFA98A;
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --spacing-lg: 24px;
  --spacing-xl: 32px;
  --spacing-2xl: 48px;
  --radius: 4px;
  --radius-lg: 8px;
}

*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

body {
  background: var(--bg);
  color: var(--text);
  font-family: 'Spectral', Georgia, serif;
  font-size: 16px;
  line-height: 1.7;
  min-height: 100vh;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: var(--spacing-xl) var(--spacing-lg);
}

/* ── Header ── */
.report-header {
  text-align: center;
  padding: var(--spacing-2xl) 0 var(--spacing-xl);
  border-bottom: 2px solid var(--border);
  margin-bottom: var(--spacing-2xl);
}
.report-header h1 {
  font-family: 'Cinzel', 'Times New Roman', serif;
  font-size: 40px;
  font-weight: 700;
  line-height: 1.15;
  color: var(--gold);
  letter-spacing: 2px;
  text-transform: uppercase;
  text-shadow: 0 2px 20px rgba(202,138,4,0.3);
}
.report-header .subtitle {
  font-family: 'Cinzel', serif;
  font-size: 14px;
  color: var(--text-dim);
  letter-spacing: 2px;
  text-transform: uppercase;
  margin-top: var(--spacing-sm);
}

/* ── Summary Cards ── */
.summary-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-2xl);
}
.summary-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--spacing-lg);
  text-align: center;
  box-shadow: 1px 2px 8px rgba(202,138,4,0.08);
}
.summary-card .label {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: var(--text-dim);
  margin-bottom: var(--spacing-sm);
}
.summary-card .value {
  font-family: 'Cinzel', serif;
  font-size: 28px;
  font-weight: 700;
  color: var(--gold);
}
.summary-card .value.danger { color: var(--red); }
.summary-card .value.success { color: var(--green); }

/* ── Sections ── */
section {
  margin-bottom: var(--spacing-2xl);
}
section h2 {
  font-family: 'Cinzel', serif;
  font-size: 24px;
  font-weight: 600;
  color: var(--gold);
  line-height: 1.25;
  margin-bottom: var(--spacing-lg);
  padding-bottom: var(--spacing-sm);
  border-bottom: 1px solid var(--border);
  letter-spacing: 1px;
}
.chart-container {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--spacing-lg);
  box-shadow: 1px 2px 8px rgba(202,138,4,0.08);
  position: relative;
}
canvas { width: 100% !important; max-height: 400px; }

/* ── Treemap ── */
#treemap { width: 100%; height: 400px; }
#treemap canvas { width: 100% !important; height: 100% !important; }

/* ── Donut Legend ── */
.donut-wrapper { display: flex; align-items: center; gap: var(--spacing-xl); flex-wrap: wrap; }
.donut-wrapper canvas { max-width: 300px; max-height: 300px; }
.donut-legend { flex: 1; }
.donut-legend-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) 0;
  border-bottom: 1px solid var(--border-dim);
  font-size: 14px;
}
.donut-legend-item:last-child { border-bottom: none; }
.donut-legend-swatch {
  width: 14px; height: 14px; border-radius: 2px; flex-shrink: 0;
}
.donut-legend-name { flex: 1; }
.donut-legend-cost { font-family: 'Fira Code', monospace; color: var(--gold); }
.donut-legend-pct { color: var(--text-dim); font-size: 12px; margin-left: var(--spacing-sm); }

/* ── Signal Table ── */
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
th {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 1px;
  text-transform: uppercase;
  text-align: left;
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border);
  color: var(--text-dim);
  cursor: pointer;
  user-select: none;
}
th:hover { color: var(--gold); }
th .sort-arrow { margin-left: 4px; font-size: 10px; }
td {
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border-dim);
}
tr:hover td { background: var(--surface-elevated); }
.severity-badge {
  font-family: 'Cinzel', serif;
  font-size: 11px;
  letter-spacing: 1px;
  text-transform: uppercase;
  padding: 4px 12px;
  border-radius: 2px;
  display: inline-block;
}
.severity-badge.high { background: rgba(153,27,27,0.2); color: var(--red-light); }
.severity-badge.medium { background: rgba(202,138,4,0.2); color: var(--gold); }
.severity-badge.low { background: rgba(88,28,135,0.2); color: var(--gold); }
.expand-btn {
  background: none;
  border: 1px solid var(--border);
  color: var(--gold);
  font-family: 'Cinzel', serif;
  font-size: 14px;
  cursor: pointer;
  padding: 2px 8px;
  border-radius: 2px;
  transition: all 0.2s;
}
.expand-btn:hover { background: var(--surface-elevated); border-color: var(--gold); }
.expand-row { display: none; }
.expand-row.open { display: table-row; }
.expand-cell {
  padding: 0 !important;
  background: var(--surface-elevated);
  border-bottom: 1px solid rgba(202,138,4,0.15);
}
.expand-inner {
  padding: var(--spacing-md);
  font-size: 13px;
}
.expand-inner h4 {
  font-family: 'Cinzel', serif;
  color: var(--gold);
  font-size: 16px;
  margin-bottom: var(--spacing-sm);
}
.timeline-event {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: 4px 0;
  font-family: 'Fira Code', monospace;
  font-size: 12px;
  border-bottom: 1px solid var(--border-dim);
}
.timeline-event .idx { color: var(--text-dim); min-width: 40px; }
.timeline-event .content { flex: 1; }
.timeline-event .annotation { color: var(--red-light); font-style: italic; }
.timeline-event.subagent { padding-left: var(--spacing-lg); }
.timeline-event .cost { color: var(--gold); text-align: right; min-width: 60px; }

/* ── Chart placeholder ── */
.chart-placeholder {
  display: none;
  text-align: center;
  padding: var(--spacing-2xl);
  color: var(--text-dim);
  font-family: 'Cinzel', serif;
  font-size: 14px;
  letter-spacing: 1px;
}
.no-charts .chart-placeholder { display: block; }
.no-charts canvas { display: none !important; }
.no-charts #treemap { display: none; }

/* ── Footer ── */
footer {
  text-align: center;
  padding: var(--spacing-xl);
  border-top: 1px solid var(--border);
  color: var(--text-dim);
  font-size: 12px;
  font-family: 'Cinzel', serif;
  letter-spacing: 1px;
  margin-top: var(--spacing-2xl);
}

@media (max-width: 768px) {
  .container { padding: var(--spacing-md); }
  .summary-row { grid-template-columns: repeat(2, 1fr); }
  .donut-wrapper { flex-direction: column; }
}
</style>
</head>
<body>
<div class="container">
`)
	b.WriteString(renderHeader())
	b.WriteString(renderSummary(data.Summary))
	b.WriteString(`<section id="cost-over-time">
<h2>Cost Over Time</h2>
<div class="chart-container">
<div class="chart-placeholder">Chart Library Unavailable</div>
<canvas id="costChart"></canvas>
</div>
</section>
<section id="waste-by-type">
<h2>Waste by Signal Type</h2>
<div class="chart-container">
<div class="chart-placeholder">Chart Library Unavailable</div>
<canvas id="wasteTypeChart"></canvas>
</div>
</section>
<section id="top-files">
<h2>Top Wasted Files</h2>
<div class="chart-container">
<div class="chart-placeholder">Chart Library Unavailable</div>
<canvas id="topFilesChart"></canvas>
</div>
</section>
<section id="subagent-tree">
<h2>Subagent Cost Tree</h2>
<div class="chart-container">
<div class="chart-placeholder">Chart Library Unavailable</div>
<div id="treemap"><canvas id="treemapCanvas"></canvas></div>
</div>
</section>
<section id="model-breakdown">
<h2>Model Cost Breakdown</h2>
<div class="chart-container">
<div class="chart-placeholder">Chart Library Unavailable</div>
<div class="donut-wrapper">
<canvas id="modelChart"></canvas>
<div id="modelLegend" class="donut-legend"></div>
</div>
</div>
</section>
<section id="signal-table">
<h2>Waste Signals</h2>
<div class="chart-container" style="overflow-x:auto">
<table id="signals">
<thead><tr>
<th data-sort="severity">Sev <span class="sort-arrow"></span></th>
<th data-sort="session">Session <span class="sort-arrow"></span></th>
<th data-sort="project">Project <span class="sort-arrow"></span></th>
<th data-sort="reason">Reason <span class="sort-arrow"></span></th>
<th data-sort="cost">Cost <span class="sort-arrow"></span></th>
<th></th>
</tr></thead>
<tbody></tbody>
</table>
</div>
</section>
`)
	b.WriteString(renderFooter(data.Version, data.Generated))
	b.WriteString(`
</div>

<script>
const REPORT = `)
	b.Write(jsonData)
	b.WriteString(`;
</script>
`)
	b.WriteString(renderJS())
	b.WriteString(`
</body>
</html>`)

	return b.String()
}

func renderHeader() string {
	var b strings.Builder
	b.WriteString(`<header class="report-header">
<h1>burnwatch</h1>
<div class="subtitle">Waste Analysis Report</div>
</header>
`)
	return b.String()
}

func renderSummary(s reportSummary) string {
	var b strings.Builder
	b.WriteString(`<div class="summary-row">
`)
	costClass := ""
	if s.TotalCost > 0 {
		costClass = " value"
	}
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Total Cost</div><div class="%s">$%.2f</div></div>
`, costClass, s.TotalCost)
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Sessions</div><div class="value">%d</div></div>
`, s.Sessions)
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Projects</div><div class="value">%d</div></div>
`, s.ProjectCount)
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Waste Signals</div><div class="value%s">%d</div></div>
`, map[bool]string{true: " danger", false: ""}[s.TotalSignals > 0], s.TotalSignals)
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Date Range</div><div class="value" style="font-size:14px">%s</div></div>
`, s.DateFrom)
	fmt.Fprintf(&b, `<div class="summary-card"><div class="label">Days Analyzed</div><div class="value">%d</div></div>
`, s.DayCount)
	b.WriteString(`</div>
`)
	return b.String()
}

func renderFooter(version, generated string) string {
	ts, _ := time.Parse(time.RFC3339, generated)
	tsStr := generated
	if !ts.IsZero() {
		tsStr = ts.Format("2006-01-02 15:04:05 UTC")
	}
	return fmt.Sprintf(`<footer>burnwatch %s &middot; generated %s</footer>
`, version, tsStr)
}

func renderJS() string {
	return `
(function(){
  if (typeof Chart === 'undefined') {
    document.body.classList.add('no-charts');
    buildTable();
    return;
  }

  var gold       = '#CA8A04';
  var goldAlpha  = 'rgba(202,138,4,0.15)';
  var red        = '#991B1B';
  var purple     = '#581C87';
  var green      = '#22C55E';
  var text       = '#F5E6D3';
  var textDim    = '#BFA98A';
  var surface    = '#2C1A10';
  var border     = '#5C3D2E';
  var redLight   = '#F87171';
  var colors     = [gold, red, purple, green, '#DAA520', '#B91C1C', '#7C3AED', '#4ADE80', '#B8780A', '#EF4444'];

  Chart.defaults.color = textDim;
  Chart.defaults.borderColor = border;

  buildCostOverTime();
  buildWasteByType();
  buildTopFiles();
  buildTreemap();
  buildModelDonut();
  buildTable();

  window.addEventListener('resize', function(){
    Object.values(Chart.instances).forEach(function(c){ c.resize(); });
  });

  // ── Cost Over Time ──
  function buildCostOverTime() {
    var data = REPORT.costOverTime || [];
    if (!data.length) return;
    var ctx = document.getElementById('costChart').getContext('2d');
    new Chart(ctx, {
      type: 'line',
      data: {
        labels: data.map(function(d){ return d.date; }),
        datasets: [
          {
            label: 'Daily Cost',
            data: data.map(function(d){ return d.cost; }),
            borderColor: gold,
            backgroundColor: goldAlpha,
            fill: true,
            tension: 0.3,
            pointRadius: 2,
            pointHoverRadius: 5,
            borderWidth: 2
          },
          {
            label: '7-Day Avg',
            data: data.map(function(d){ return d.movingAvg; }),
            borderColor: purple,
            borderDash: [5, 5],
            borderWidth: 2,
            pointRadius: 0,
            fill: false,
            tension: 0.3
          }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { usePointStyle: true, padding: 20, font: { family: 'Cinzel', size: 11 } } },
          tooltip: { callbacks: { label: function(c){ return '$' + c.raw.toFixed(2); } } }
        },
        scales: {
          x: { grid: { color: border }, ticks: { font: { family: 'Spectral', size: 11 }, maxTicksLimit: 15 } },
          y: { grid: { color: border }, ticks: { font: { family: 'Fira Code', size: 11 }, callback: function(v){ return '$' + v; } } }
        }
      }
    });
  }

  // ── Waste by Type ──
  function buildWasteByType() {
    var data = REPORT.wasteByType || [];
    if (!data.length) return;
    var ctx = document.getElementById('wasteTypeChart').getContext('2d');

    var reasonColors = {
      tool_call_loop: red,
      file_reread: gold,
      subagent_overlap: purple,
      session_restart: '#DAA520',
      cost_outlier: '#B91C1C',
      low_signal: textDim,
      subagent_overhead: '#7C3AED',
      cache_underutilized: '#B8780A',
      fragmentation_index: '#EF4444',
      input_overconsumption: '#FF6B35',
      output_explosion: '#E84D8A',
      low_token_efficiency: '#36A2EB'
    };

    var allReasons = {};
    data.forEach(function(p){
      Object.keys(p.reasons || {}).forEach(function(r){ allReasons[r] = true; });
    });
    var reasonKeys = Object.keys(allReasons).sort();

    var datasets = reasonKeys.map(function(reason, i){
      return {
        label: reason.replace(/_/g, ' '),
        data: data.map(function(p){ return (p.reasons && p.reasons[reason]) || 0; }),
        backgroundColor: reasonColors[reason] || colors[i % colors.length],
        borderWidth: 0
      };
    });

    new Chart(ctx, {
      type: 'bar',
      data: {
        labels: data.map(function(d){ return d.project; }),
        datasets: datasets
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { usePointStyle: true, padding: 15, font: { family: 'Cinzel', size: 10 } } },
          tooltip: { callbacks: { label: function(c){ return c.dataset.label + ': $' + c.raw.toFixed(2); } } }
        },
        scales: {
          x: { stacked: true, grid: { color: border }, ticks: { font: { family: 'Spectral', size: 11 } } },
          y: { stacked: true, grid: { color: border }, ticks: { font: { family: 'Fira Code', size: 11 }, callback: function(v){ return '$' + v; } } }
        }
      }
    });
  }

  // ── Top Files ──
  function buildTopFiles() {
    var data = REPORT.topFiles || [];
    if (!data.length) return;
    var ctx = document.getElementById('topFilesChart').getContext('2d');
    new Chart(ctx, {
      type: 'bar',
      data: {
        labels: data.map(function(d){ return d.path.split('/').pop() || d.path; }),
        datasets: [{
          label: 'Read Count',
          data: data.map(function(d){ return d.readCount; }),
          backgroundColor: data.map(function(_, i){ return colors[i % colors.length]; }),
          borderWidth: 0,
          borderRadius: 2
        }]
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              title: function(items){ var i = items[0].dataIndex; return data[i].path; },
              label: function(c){ return c.raw + ' reads, $' + data[c.dataIndex].cost.toFixed(2); }
            }
          }
        },
        scales: {
          x: { grid: { color: border }, ticks: { font: { family: 'Fira Code', size: 11 } } },
          y: { grid: { display: false }, ticks: { font: { family: 'Spectral', size: 11 } } }
        }
      }
    });
  }

  // ── Treemap ──
  function buildTreemap() {
    var treeData = REPORT.subagentTree;
    if (!treeData || !treeData.children || !treeData.children.length) {
      var div = document.getElementById('treemap');
      div.innerHTML = '<div class="chart-placeholder" style="display:block;padding:60px">No subagents detected</div>';
      return;
    }
    var canv = document.getElementById('treemapCanvas');
    var parent = document.getElementById('treemap');
    var W = parent.clientWidth;
    var H = parent.clientHeight || 400;
    canv.width = W;
    canv.height = H;
    var ctx = canv.getContext('2d');

    var nodes = [];
    function flatten(n, depth) {
      if (!n) return;
      nodes.push({ name: n.name || 'unknown', value: n.value || n.cost || 0, depth: depth || 0, children: n.children });
      if (n.children) {
        n.children.forEach(function(c){ flatten(c, (depth||0)+1); });
      }
    }
    flatten(treeData, 0);

    var leaves = nodes.filter(function(n){ return !n.children || !n.children.length; });
    if (!leaves.length) return;

    var total = 0;
    leaves.forEach(function(l){ total += l.value; });
    if (total <= 0) return;

    var x = 0, y = 0;
    leaves.sort(function(a,b){ return b.value - a.value; });

    // simple slice-and-dice treemap
    layoutSlice(leaves, 0, 0, W, H);

    leaves.forEach(function(n){
      var r = n._r || {x:0,y:0,w:0,h:0};
      ctx.fillStyle = colors[n.depth % colors.length];
      ctx.fillRect(r.x+1, r.y+1, Math.max(0, r.w-2), Math.max(0, r.h-2));

      ctx.fillStyle = canvasContrast(colors[n.depth % colors.length]);
      ctx.font = '10px "Fira Code", monospace';
      var label = n.name;
      if (r.w > 60 && r.h > 18) {
        ctx.fillText(label, r.x + 4, r.y + 16);
        var vtext = '$' + n.value.toFixed(2);
        if (r.h > 34) {
          ctx.fillStyle = textDim;
          ctx.font = '9px "Cinzel", serif';
          ctx.fillText(vtext, r.x + 4, r.y + 30);
        }
      }
    });
  }

  function layoutSlice(items, x, y, w, h) {
    if (!items.length) return;
    if (w <= 0 || h <= 0) return;
    var total = 0;
    items.forEach(function(i){ total += i.value; });
    if (total <= 0) return;

    if (w >= h) {
      var cx = x;
      items.forEach(function(i){
        var iw = (i.value / total) * w;
        i._r = { x: cx, y: y, w: iw, h: h };
        cx += iw;
      });
    } else {
      var cy = y;
      items.forEach(function(i){
        var ih = (i.value / total) * h;
        i._r = { x: x, y: cy, w: w, h: ih };
        cy += ih;
      });
    }
  }

  // ── Model Donut ──
  function buildModelDonut() {
    var data = REPORT.modelBreakdown || [];
    if (!data.length) return;
    var ctx = document.getElementById('modelChart').getContext('2d');
    var legend = document.getElementById('modelLegend');

    data.forEach(function(d, i){
      var item = document.createElement('div');
      item.className = 'donut-legend-item';
      item.innerHTML = '<span class="donut-legend-swatch" style="background:'+colors[i%colors.length]+'"></span>' +
        '<span class="donut-legend-name">'+escHtml(d.model)+'</span>' +
        '<span class="donut-legend-cost">$'+d.cost.toFixed(2)+'</span>' +
        '<span class="donut-legend-pct">'+d.percentage.toFixed(1)+'%</span>';
      legend.appendChild(item);
    });

    new Chart(ctx, {
      type: 'doughnut',
      data: {
        labels: data.map(function(d){ return d.model; }),
        datasets: [{
          data: data.map(function(d){ return d.cost; }),
          backgroundColor: data.map(function(_, i){ return colors[i % colors.length]; }),
          borderColor: surface,
          borderWidth: 2
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        cutout: '60%',
        plugins: {
          legend: { display: false },
          tooltip: { callbacks: { label: function(c){ return c.label + ': $' + c.raw.toFixed(2) + ' (' + data[c.dataIndex].percentage.toFixed(1) + '%)'; } } }
        }
      }
    });
  }

  // ── Signal Table ──
  function buildTable() {
    var signals = REPORT.signals || [];
    var timelines = REPORT.signalTimelines || {};
    var tbody = document.querySelector('#signals tbody');

    signals.forEach(function(s, i){
      var row = document.createElement('tr');
      row.innerHTML =
        '<td><span class="severity-badge '+s.severity+'">'+escHtml(s.severity)+'</span></td>' +
        '<td style="font-family: \'Fira Code\', monospace; font-size: 12px">'+escHtml(s.sessionId)+'</td>' +
        '<td>'+escHtml(s.project)+'</td>' +
        '<td style="font-size:13px; color:'+textDim+'">'+escHtml(s.reason.replace(/_/g, ' '))+'</td>' +
        '<td style="font-family: \'Fira Code\', monospace; color:'+gold+'">$'+s.cost.toFixed(2)+'</td>' +
        '<td><button class="expand-btn" data-idx="'+i+'">+</button></td>';
      tbody.appendChild(row);

      var tl = timelines[s.sessionId];
      if (tl) {
        var exRow = document.createElement('tr');
        exRow.className = 'expand-row';
        exRow.innerHTML = '<td colspan="6" class="expand-cell"><div class="expand-inner">' +
          '<h4>Session '+s.sessionId+'</h4>' +
          buildTimelineHTML(tl) +
          '</div></td>';
        tbody.appendChild(exRow);
      }
    });

    // Sorting
    document.querySelectorAll('#signals th[data-sort]').forEach(function(th){
      th.addEventListener('click', function(){
        sortTable(this.dataset.sort);
      });
    });

    // Expand/collapse
    document.querySelectorAll('.expand-btn').forEach(function(btn){
      btn.addEventListener('click', function(){
        var row = this.closest('tr').nextElementSibling;
        if (row && row.classList.contains('expand-row')) {
          row.classList.toggle('open');
          this.textContent = row.classList.contains('open') ? '-' : '+';
        }
      });
    });
  }

  function buildTimelineHTML(tl) {
    var html = '';
    tl.forEach(function(e){
      var cls = e.isSubagent ? ' timeline-event subagent' : 'timeline-event';
      html += '<div class="'+cls+'">';
      html += '<span class="idx">#'+e.index+'</span>';
      html += '<span class="content">';
      if (e.toolCalls && e.toolCalls.length) {
        e.toolCalls.forEach(function(tc){ html += '<div>'+escHtml(tc)+'</div>'; });
      }
      if (e.fileOps && e.fileOps.length) {
        e.fileOps.forEach(function(fo){ html += '<div style="color:'+textDim+'">'+escHtml(fo)+'</div>'; });
      }
      if (e.annotations && e.annotations.length) {
        e.annotations.forEach(function(a){ html += '<div class="annotation">'+escHtml(a)+'</div>'; });
      }
      html += '</span>';
      html += '<span class="cost">$'+e.cost.toFixed(2)+'</span>';
      html += '</div>';
    });
    return html;
  }

  function sortTable(key) {
    var tbody = document.querySelector('#signals tbody');
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
      var aval;
      var bval;
      if (key === 'severity') {
        aval = ['high','medium','low'].indexOf(a.cells[0].textContent.trim().toLowerCase());
        bval = ['high','medium','low'].indexOf(b.cells[0].textContent.trim().toLowerCase());
      } else if (key === 'cost') {
        aval = parseFloat(a.cells[4].textContent.replace('$',''));
        bval = parseFloat(b.cells[4].textContent.replace('$',''));
      } else {
        aval = a.cells[{session:1, project:2, reason:3}[key] || 0].textContent;
        bval = b.cells[{session:1, project:2, reason:3}[key] || 0].textContent;
        if (typeof aval === 'string') {
          return dir * aval.localeCompare(bval);
        }
      }
      return dir * (aval > bval ? 1 : aval < bval ? -1 : 0);
    });

    // rebuild tbody preserving expand rows
    dataRows.forEach(function(r){
      var next = r.nextElementSibling;
      tbody.appendChild(r);
      if (next && next.classList.contains('expand-row')) {
        tbody.appendChild(next);
      }
    });

    document.querySelectorAll('#signals th .sort-arrow').forEach(function(s){ s.textContent = ''; });
    var th = document.querySelector('#signals th[data-sort="'+key+'"] .sort-arrow');
    if (th) th.textContent = nextDir === 'desc' ? '▼' : '▲';
  }

  // ── Helpers ──

  function canvasContrast(hex) {
    var r = parseInt(hex.slice(1,3), 16);
    var g = parseInt(hex.slice(3,5), 16);
    var b = parseInt(hex.slice(5,7), 16);
    return (r*0.299 + g*0.587 + b*0.114) > 128 ? '#1A0F0A' : '#F5E6D3';
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
