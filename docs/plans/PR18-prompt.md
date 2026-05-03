# PR18: ML Pipeline — Supervised Waste Classification (Experimental)

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Build a supervised learning pipeline that learns from user-labeled sessions which patterns represent genuine waste. Closes the feedback loop: heuristics flag → LLM verifies → user labels → classifier learns → future runs improve.

## Success Criteria

- [ ] `burnwatch --label ses_abc123 waste [--reason "context bloat"]` stores a label
- [ ] `burnwatch --label ses_def456 not_waste` stores a label
- [ ] `burnwatch --labels` lists all labeled sessions with verdicts
- [ ] `burnwatch --labels --clear` clears all labels
- [ ] Labels stored in `~/.cache/burnwatch/labels.jsonl` (JSON lines, one label per line)
- [ ] `burnwatch --train` trains a logistic regression classifier on labeled data
- [ ] Classifier saved to `~/.cache/burnwatch/classifier.json`
- [ ] When classifier exists and ≥20 labels, `P(waste)` score computed for each session
- [ ] Sessions with `P(waste) > 0.7` and not already flagged get a new signal (severity: LOW, reason: "ml_predicted")
- [ ] `--train` prints precision, recall, F1 score on hold-out set
- [ ] Config toggle: `ml.enabled = false` (disabled by default, experimental)
- [ ] No external ML dependencies (pure Go logistic regression)
- [ ] Existing heuristics unchanged — ML is supplementary, not replacement

## Dependencies

- **Must merge first:** PR17 (LLM verification — generates initial labeled data via LLM verdicts)
- **External dependencies:** None
- **Can be parallel with:** None (serial after PR17)
- **Breaking changes / Migrations needed:** New CLI commands. New config section. New cache files.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr18-ml-pipeline`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/ml_label.go` | Label struct, label storage/retrieval, CLI integration | New file |
| `analyze/ml_classifier.go` | LogisticRegression struct, Train, Predict, FeatureVector | New file |
| `analyze/ml_label_test.go` | Label CRUD tests | New file |
| `analyze/ml_classifier_test.go` | Training, prediction, accuracy tests | New file |
| `analyze/waste.go` | Add ML prediction signal type | Modify |
| `cmd/root.go` | `--label`, `--labels`, `--clear-labels`, `--train` flags | Modify |
| `config/config.go` | Add `ml` config section | Modify |

---

## Implementation

### `analyze/ml_label.go` — Label storage

```
Type: Label
Fields:
  SessionID  string    `json:"session_id"`
  Verdict    string    `json:"verdict"`     // "waste" or "not_waste"
  Reason     string    `json:"reason"`      // user-provided or LLM-provided
  Source     string    `json:"source"`      // "manual", "llm_verify"
  CreatedAt  time.Time `json:"created_at"`

Functions:
  LabelsPath() string
    → os.UserCacheDir() + "/burnwatch/labels.jsonl"

  LoadLabels(path string) ([]Label, error)
    → Read JSONL file, one label per line

  SaveLabel(path string, label Label) error
    → Append to JSONL file

  DeleteAllLabels(path string) error
    → Remove file

  ListLabels(path string) (string, error)
    → Format labels as text table

  ImportFromLLMVerdicts(verdicts []LlmVerdict, path string) error
    → Convert LlmVerdict to Label (source="llm_verify")
    → Skip if label already exists for session_id
    → Append to labels file
```

### CLI commands

```bash
# Label a session
burnwatch --label ses_abc123 waste
burnwatch --label ses_def456 not_waste --reason "legitimate code review"

# List all labels
burnwatch --labels

# Clear all labels
burnwatch --labels --clear

# Train classifier
burnwatch --train

# Normal run with ML enabled
burnwatch  # uses classifier if trained and >=20 labels
```

### `analyze/ml_classifier.go` — Logistic regression

```
Type: FeatureVector = [10]float64  (same features as PR16 anomaly detection)
  Index 0: inputSum (log1p normalized)
  Index 1: outputSum (log1p normalized)
  Index 2: ratio
  Index 3: cacheRate
  Index 4: cost (log1p normalized)
  Index 5: subagentPct (0-100)
  Index 6: sessionsToday (log1p normalized)
  Index 7: modelTier (1-5)
  Index 8: isWeekend (0/1)
  Index 9: inputPerEvent (log1p normalized)

Type: LogisticRegression struct {
    Weights    [10]float64 `json:"weights"`
    Bias       float64     `json:"bias"`
    TrainedAt  time.Time   `json:"trained_at"`
    NumSamples int         `json:"num_samples"`
    Accuracy   float64     `json:"accuracy"`
    Precision  float64     `json:"precision"`   // on hold-out set
    Recall     float64     `json:"recall"`
    F1         float64     `json:"f1"`
}

Functions:
  TrainLogisticRegression(features []FeatureVector, labels []float64, learningRate float64, epochs int) LogisticRegression
    → labels: 1.0 for waste, 0.0 for not_waste
    → Stochastic gradient descent: for each epoch, shuffle samples, update weights
    → Sigmoid(z) = 1 / (1 + exp(-z))  where z = dot(weights, features) + bias
    → Gradient: (sigmoid(z) - y) * features[i] for each weight
    → Default: learningRate=0.01, epochs=1000

  (lr LogisticRegression) Predict(features []FeatureVector) []float64
    → Return P(waste) for each vector: sigmoid(z)

  (lr LogisticRegression) PredictSingle(fv FeatureVector) float64
    → Return P(waste) for single vector

  (lr LogisticRegression) classFromProb(prob float64) string
    → prob >= 0.5 → "waste", else "not_waste"

  SaveModel(path string, lr LogisticRegression) error
    → os.UserCacheDir() + "/burnwatch/classifier.json"

  LoadModel(path string) (LogisticRegression, error)

  trainTestSplit(features []FeatureVector, labels []float64, testRatio float64) (trainF, testF, trainL, testL)
    → 80/20 split, stratified (preserve waste/not_waste ratio in both sets)
```

**SGD implementation:**

```go
func TrainLogisticRegression(features []FeatureVector, labels []float64, lr float64, epochs int) LogisticRegression {
    n := len(features)
    model := LogisticRegression{TrainedAt: time.Now(), NumSamples: n}

    // Initialize weights to small random values
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    for i := range model.Weights {
        model.Weights[i] = (rng.Float64() - 0.5) * 0.01
    }

    for epoch := 0; epoch < epochs; epoch++ {
        // Shuffle indices
        indices := rand.Perm(n)
        for _, idx := range indices {
            z := model.Bias
            for i, w := range model.Weights {
                z += w * features[idx][i]
            }
            pred := sigmoid(z)
            err := pred - labels[idx]

            for i := range model.Weights {
                model.Weights[i] -= lr * err * features[idx][i]
            }
            model.Bias -= lr * err
        }
    }

    return model
}

func sigmoid(z float64) float64 {
    if z > 20 {
        return 1.0
    }
    if z < -20 {
        return 0.0
    }
    return 1.0 / (1.0 + math.Exp(-z))
}
```

### Integration into DetectWaste

```go
// In DetectWaste(), after anomaly detection, before return:
if cfg.ML.Enabled {
    model, err := LoadModel(ClassifierPath())
    if err == nil && model.NumSamples >= cfg.ML.MinLabels {
        var vectors []FeatureVector
        var sessionIDs []string
        for _, a := range agg {
            vectors = append(vectors, extractMLFeatures(a))
            sessionIDs = append(sessionIDs, a.sessionID)
        }
        probs := model.Predict(vectors)
        for i, p := range probs {
            if p > 0.7 {
                sid := sessionIDs[i]
                a := agg[sid]
                existing := findSignal(signals, sid)
                if existing != nil {
                    existing.MLWasteProb = p
                    continue
                }
                signals = append(signals, WasteSignal{
                    SessionID:    sid,
                    Severity:     "low",
                    Reason:       "ml_predicted",
                    Detail:       fmt.Sprintf("P(waste) = %.2f", p),
                    Metric:       p,
                    Threshold:    0.7,
                    SessionCost:  a.cost,
                    MLWasteProb:  p,
                    // ... model, tokens ...
                })
            }
        }
    }
}
```

Add `MLWasteProb float64` to `WasteSignal`.

### Config additions

```go
type ML struct {
    Enabled   bool `toml:"enabled"`    // default: false
    MinLabels int  `toml:"min_labels"` // default: 20
}

// In Config:
ML ML
```

### Training flow

```bash
$ burnwatch --train
Loading labels... 45 labels found (32 waste, 13 not_waste)
Extracting features... done.
Training logistic regression... (45 samples, 1000 epochs) done.
Accuracy on hold-out (9 samples): 77.8%
Precision: 0.83  Recall: 0.71  F1: 0.77
Model saved to ~/.cache/burnwatch/classifier.json
```

---

## Test Requirements

1. **`analyze/ml_label_test.go`**:
   - Save label → load labels → label appears
   - Save two labels → load returns both
   - Label with reason → reason preserved
   - ImportFromLLMVerdicts: 5 verdicts → 5 labels created
   - ImportFromLLMVerdicts: duplicate session → skipped
   - DeleteAllLabels → file removed
   - LoadLabels on non-existent file → empty slice, no error

2. **`analyze/ml_classifier_test.go`**:
   - Train on linearly separable data → 100% accuracy
   - Train on random data → ~50% accuracy (can't learn noise)
   - Sigmoid(0) = 0.5
   - Sigmoid(10) ≈ 1.0
   - Sigmoid(-10) ≈ 0.0
   - Predict on known waste pattern → P(waste) > 0.9 after training
   - Train on 30 waste + 30 not_waste → all predictions correct
   - trainTestSplit: 100 samples, 0.2 ratio → 80 train, 20 test
   - trainTestSplit: preserves class ratio
   - SaveModel → LoadModel → weights match
   - Feature vector extraction: inputSum=100000 → log1p normalized correctly
   - Empty dataset → returns untrained model (no panic)

3. Coverage target: >=90% on new code

---

## Approach

1. Implement label storage (ml_label.go)
2. Write label tests (RED → GREEN)
3. Implement logistic regression (ml_classifier.go)
4. Write classifier tests (RED → GREEN)
5. Add CLI commands for labeling
6. Add `--train` command
7. Integrate ML prediction into DetectWaste
8. Add config section
9. Manual test: label 30 sessions, train, run — verify ML predictions appear
10. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr18-ml-pipeline`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: supervised ML pipeline for waste classification`
- [ ] Push to branch `pr18-ml-pipeline`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- Logistic regression is the simplest classifier that gives calibrated probabilities. More complex models (XGBoost, random forest) would need external dependencies — rejected for v2.
- STRATIFIED train/test split is important: if 80% of labels are "waste" and we random-split, the test set might have 100% waste and accuracy trivially 100%.
- Features are identical to PR16 (Isolation Forest). Shared feature extraction function in `analyze/anomaly.go` — refactor to `analyze/features.go` that both use.
- The ML classifier supplements heuristics, doesn't replace them. Both run side by side.
- `ml.enabled = false` by default. Users must explicitly enable it and train the model.
- Minimum 20 labels (configurable) before training runs — less than that and logistic regression can't learn meaningful weights.
- Model file is human-readable JSON. Users can inspect weights to understand what features the classifier cares about.
- FUTURE: If a session is flagged as "ml_predicted" AND the user labels it manually, we retrain. But auto-retraining on every label is too aggressive for v2. Use `--train` explicitly.
