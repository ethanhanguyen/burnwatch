# PR16: Unsupervised Anomaly Detection — Isolation Forest

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Complement threshold-based heuristics with an Isolation Forest that flags multi-dimensional outliers — sessions anomalous in the joint feature space that no single heuristic would catch.

## Success Criteria

- [ ] Isolation forest implementation (pure Go, no external ML libraries, <300 lines)
- [ ] Builds 100 trees with random feature splits on random subsamples (256 sessions per tree)
- [ ] Feature vector per session: `[inputSum, outputSum, ratio, cacheRate, cost, subagentOverheadPct, sessionCountToday, modelPriceTier, isWeekend, inputPerEvent]`
- [ ] Anomaly score ∈ [0, 1] per session (0 = deep in normal cluster, 1 = easy to isolate)
- [ ] Sessions with anomaly score > `anomaly_threshold` (default 0.6) get flagged as MEDIUM
- [ ] Sessions already flagged by another heuristic get `anomaly_score` added to existing WasteSignal (no duplicate signal)
- [ ] Config toggle: `signals.anomaly = false` (disabled by default)
- [ ] Config threshold: `thresholds.anomaly_threshold = 0.6`
- [ ] Deterministic output (fixed random seed in tests, live seed = session data hash)
- [ ] Reasonably fast: <200ms for 1000 sessions

## Dependencies

- **Must merge first:** PR14 (config-wired thresholds — need anomaly config fields), PR15 (calibration — need distribution understanding to validate anomaly scores)
- **External dependencies:** None (pure Go)
- **Can be parallel with:** PR17 (different files, PR17 uses anomaly scores but can start after PR16)
- **Breaking changes / Migrations needed:** New config fields. New waste signal reason type.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr16-anomaly-detection`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/anomaly.go` | IsolationForest struct, Train, Predict, feature extraction | New file |
| `analyze/anomaly_test.go` | Algorithm correctness, edge cases, known outliers | New file |
| `analyze/waste.go` | Integration: run anomaly detection, add AnomalyScore to WasteSignal, new checkAnomaly | Modify |
| `config/config.go` | Add `signals.anomaly`, `thresholds.anomaly_threshold` | Modify |
| `config/config_test.go` | Test new defaults | Modify |
| `cmd/root.go` | Add `--no-anomaly` flag | Modify |
| `output/text.go` | Display anomaly signals | Modify |
| `output/json.go` | Serialize anomaly_score | Modify |

---

## Implementation

### `analyze/anomaly.go` — Isolation Forest

```
Type: FeatureVector = [10]float64
  Index 0: inputSum         (normalized to 0-1 via log1p scale)
  Index 1: outputSum        (normalized to 0-1 via log1p scale)
  Index 2: ratio            (already 0-1ish, clamp outliers to 10x max)
  Index 3: cacheRate        (already 0-1)
  Index 4: cost             (normalized via log1p)
  Index 5: subagentPct      (0-100, or 0 if no subagents)
  Index 6: sessionsToday    (count, log1p scaled)
  Index 7: modelTier        (1=free, 2=cheap, 3=medium, 4=expensive, 5=premium)
  Index 8: isWeekend        (0 or 1)
  Index 9: inputPerEvent    (inputSum / eventCount, log1p scaled)

Type: ITree struct {
    SplitFeature int
    SplitValue   float64
    Left, Right  *ITree
    Size         int       // leaf size (0 for internal nodes)
}

Type: IsolationForest struct {
    Trees      []*ITree
    SampleSize int         // 256
    MaxDepth   int         // ceil(log2(sampleSize)) = 8
}

Functions:
  NewIsolationForest(numTrees int, sampleSize int, seed int64) *IsolationForest
    → numTrees=100, sampleSize=256, seed for reproducibility

  (f *IsolationForest) Fit(vectors []FeatureVector)
    → For each tree:
        Subsample 256 vectors (without replacement if len < 256, with replacement otherwise)
        Build tree recursively (split random feature on random value between min/max)
        Stop when: depth > maxDepth OR node size <= 1 OR min==max for all features

  (f *IsolationForest) Predict(vector FeatureVector) float64
    → Average path length across all trees
    → Normalize: score = 2^(-E(h(x)) / c(N)) where c(N) is average path length of unsuccessful search in BST
    → c(N) = 2*H(N-1) - 2*(N-1)/N  where H(i) ≈ ln(i) + 0.5772156649 (Euler constant)
    → Returns 0.0 to 1.0 (1.0 = most anomalous)

  (f *IsolationForest) PredictAll(vectors []FeatureVector) []float64

  extractFeatures(agg *sessionAgg) FeatureVector
    → Build feature vector from sessionAgg fields
    → Normalize: for features 0,1,4,6,9 use log1p(float64(value)) to handle wide range
    → Normalization happens per-feature across ALL session vectors before training

  normalizeColumns(vectors []FeatureVector) []FeatureVector
    → For each column, compute min/max, scale to [0,1]
    → This is crucial — without normalization, large token counts dominate splits
```

**c(N) function:**

```go
func c(n int) float64 {
    if n <= 1 {
        return 0
    }
    if n == 2 {
        return 1
    }
    h := math.Log(float64(n-1)) + 0.5772156649 // harmonic number approx
    return 2.0*h - (2.0*float64(n-1))/float64(n)
}
```

**Tree building:**

```go
func buildTree(vectors []FeatureVector, depth int, maxDepth int, rng *rand.Rand) *ITree {
    n := len(vectors)
    if depth >= maxDepth || n <= 1 {
        return &ITree{Size: n}
    }
    // Check if all features have zero range
    allSame := true
    for f := 0; f < numFeatures; f++ {
        minVal, maxVal := rangeForFeature(vectors, f)
        if maxVal > minVal {
            allSame = false
            break
        }
    }
    if allSame {
        return &ITree{Size: n}
    }
    // Pick random feature with non-zero range
    feature := rng.Intn(numFeatures)
    minVal, maxVal := rangeForFeature(vectors, feature)
    for maxVal <= minVal {
        feature = rng.Intn(numFeatures)
        minVal, maxVal = rangeForFeature(vectors, feature)
    }
    split := minVal + rng.Float64()*(maxVal-minVal)
    var left, right []FeatureVector
    for _, v := range vectors {
        if v[feature] < split {
            left = append(left, v)
        } else {
            right = append(right, v)
        }
    }
    // Handle edge: all vectors on one side
    if len(left) == 0 || len(right) == 0 {
        return &ITree{Size: n}
    }
    return &ITree{
        SplitFeature: feature,
        SplitValue:   split,
        Left:         buildTree(left, depth+1, maxDepth, rng),
        Right:        buildTree(right, depth+1, maxDepth, rng),
    }
}
```

**Path length:**

```go
func pathLength(tree *ITree, vector FeatureVector, currentDepth int) float64 {
    if tree.Left == nil || tree.Right == nil {
        // Leaf node
        if tree.Size <= 1 {
            return float64(currentDepth)
        }
        return float64(currentDepth) + c(tree.Size)
    }
    if vector[tree.SplitFeature] < tree.SplitValue {
        return pathLength(tree.Left, vector, currentDepth+1)
    }
    return pathLength(tree.Right, vector, currentDepth+1)
}
```

### Integration into `analyze/waste.go`

```go
// In DetectWaste(), after all per-session checks, before churn:
if toggles.Anomaly {
    anomalyForest, anomalyScores := runAnomalyDetection(agg, cfg)
    for _, a := range agg {
        score := anomalyScores[a.sessionID]
        if score > cfg.Thresholds.AnomalyThreshold {
            // Check if session already has a signal
            if existing := findSignal(signals, a.sessionID); existing != nil {
                existing.AnomalyScore = score  // add to existing signal
            } else {
                signals = append(signals, WasteSignal{
                    SessionID:     a.sessionID,
                    Severity:      "medium",
                    Reason:        "anomaly",
                    Detail:        fmt.Sprintf("anomaly score = %.2f", score),
                    Metric:        score,
                    Threshold:     cfg.Thresholds.AnomalyThreshold,
                    SessionCost:   a.cost,
                    AnomalyScore:  score,
                    // ... model, tokens ...
                })
            }
        }
    }
}
```

Add `AnomalyScore float64` to `WasteSignal`:

```go
type WasteSignal struct {
    // ... existing ...
    AnomalyScore float64  `json:"anomaly_score,omitempty"`  // set when anomaly heuristic fires
}
```

### Model price tier mapping

```go
func modelPriceTier(model string) int {
    lower := strings.ToLower(model)
    if strings.Contains(lower, "free") || strings.Contains(lower, "deepseek-v4-flash") {
        return 1  // free/tiny
    }
    if strings.Contains(lower, "haiku") || strings.Contains(lower, "flash") ||
       strings.Contains(lower, "mini") || strings.Contains(lower, "nano") ||
       strings.Contains(lower, "qwen-turbo") || strings.Contains(lower, "minimax-m2.5") {
        return 2  // cheap
    }
    if strings.Contains(lower, "sonnet") || strings.Contains(lower, "gemini-3-pro") ||
       strings.Contains(lower, "deepseek-v4-pro") || strings.Contains(lower, "kimi-k2.6") ||
       strings.Contains(lower, "qwen3.6") || strings.Contains(lower, "gpt-5.4") {
        return 3  // medium
    }
    if strings.Contains(lower, "opus") || strings.Contains(lower, "gemini-2.5-pro") ||
       strings.Contains(lower, "gpt-5.4-pro") || strings.Contains(lower, "gpt-5.5") {
        return 4  // expensive
    }
    return 5  // premium (unknown, conservative)
}
```

### `output/text.go` — Anomaly display

```
  MEDIUM ses_abc123 (project): $2.34 — anomaly score = 0.72 (threshold = 0.60)
    → Session is anomalous in joint feature space. Review manually.
```

### Config additions

```go
// In Signals:
Anomaly  bool  `toml:"anomaly"`   // default: false

// In Thresholds:
AnomalyThreshold float64 `toml:"anomaly_threshold"`  // default: 0.6
```

### Seed determinism

```go
// For reproducibility: hash session IDs to create seed
func sessionSeed(agg map[string]*sessionAgg) int64 {
    var ids []string
    for id := range agg {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    h := fnv.New64a()
    for _, id := range ids {
        h.Write([]byte(id))
    }
    return int64(h.Sum64())
}
```

Use `seed` for `rand.NewSource(seed)` — ensures same data → same anomaly scores. Tests override seed.

---

## Test Requirements

1. **`analyze/anomaly_test.go`**:
   - Isolation forest: train on 100 random points, all anomaly scores <0.5 (no outliers)
   - Known outlier: train on [0,0,0,...] × 95 + [1,1,1,...] × 5 → 5 outliers get score >0.6
   - Single outlier far from cluster → anomaly score >0.8
   - All identical vectors → all scores ≈0.5 (no way to isolate)
   - Normalization: verify columns scaled to [0,1] range
   - c(N) function: c(1)=0, c(2)=1.0, c(10)≈3.0 (spot check)
   - Path length: leaf node of size 1 at depth 5 → depth=5
   - Path length: leaf node of size 3 at depth 4 → depth=4 + c(3)
   - Deterministic: same seed + same data → same scores
   - Performance: 1000 sessions × 100 trees <200ms (benchmark test)
   - modelPriceTier: known models mapped correctly

2. **`analyze/waste_test.go`**:
   - Anomaly detection finds outlier in homogeneous dataset
   - Anomaly detection does NOT flag normal sessions
   - AnomalyScore added to existing WasteSignal (not duplicated)
   - anomaly toggle disabled → no anomaly signals

3. Coverage target: >=90% on new code

---

## Approach

1. Implement `analyze/anomaly.go` with pure-Go isolation forest
2. Write comprehensive tests (RED → GREEN)
3. Implement feature extraction and normalization
4. Add anomaly fields to config
5. Integrate into DetectWaste
6. Wire CLI flag + config toggle
7. Add output display
8. Performance benchmark
9. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr16-anomaly-detection`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: isolation forest anomaly detection on session features`
- [ ] Push to branch `pr16-anomaly-detection`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- Isolation forest algorithm from: Liu, Ting, Zhou (2008) "Isolation Forest." ICDM.
- Build 100 trees with sampleSize=256. For 1000 sessions: 100 × 256 comparisons = 25.6K operations per predict. With 10 features, ~256K float comparisons total. Well under 200ms.
- `modelPriceTier` function is a simplification. For accuracy, could use actual $/MTok from pricing but that adds coupling to PR11 pricing. Tier buckets are good enough for anomaly detection.
- `isWeekend` = 1 if session day is Saturday or Sunday. Weekend sessions have different patterns (batch processing, longer runs).
- `inputPerEvent` = inputSum / eventCount. Requires tracking event count in sessionAgg. Add `eventCount int` to sessionAgg.
- The algorithm normalizes features to [0,1] internally. Raw values in vectors are log1p-scaled before normalization.
- Anomaly detection is disabled by default. Users enable it via config or `--anomaly` flag (if we add one, or just config).
