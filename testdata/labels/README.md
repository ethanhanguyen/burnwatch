# Labels

Labeled sessions for signal quality benchmarking. Each session has a ground-truth verdict.

## Format

One JSON object per line in `labels.jsonl`:

```json
{"session_id":"ses_xxx","verdict":"waste|not_waste","reason":"...","source":"scenario|manual|llm_verify","created_at":"ISO8601"}
```

## Adding labels

```bash
# Mark a session as waste
burnwatch --label ses_abc123 waste --reason "tool-call loop"

# Mark as not waste
burnwatch --label ses_def456 not_waste --reason "legitimate code review"
```

## Source

- `scenario` — synthetic labels from crafted scenario data (bootstrapped)
- `manual` — user-labeled via CLI
- `llm_verify` — imported from LLM verification results (PR17+)

## Benchmarking

Labels are used by `BenchmarkSignalQuality` to compute precision, recall, and F1 score:

```
go test ./output -bench=SignalQuality -benchmem
```
