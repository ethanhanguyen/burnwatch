# PR19: LLM Verification — AI-Powered Waste Signal Review

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Optionally verify the top-N waste signals using an LLM (via OpenRouter API). The LLM reviews session statistics and classifies each signal as genuine waste or false positive, with a diagnosis. This reduces false positives on the most impactful signals and builds labeled data for PR20.

## Success Criteria

- [ ] `--llm-verify --llm-key <key>` enables LLM verification for top-N signals (default 20, max 50)
- [ ] `--llm-verify --llm-key <key> --llm-confirm` prints estimated cost and requires confirmation before calling
- [ ] LLM call uses OpenRouter API (`https://openrouter.ai/api/v1/chat/completions`) with `anthropic/claude-haiku-4.5`
- [ ] Prompt includes session stats: input/output tokens, model, cost, subagent count, overhead %, project, date, sessions that day
- [ ] LLM response parsed for `WASTE|NOT_WASTE|<reason>` pattern. Fallback: `UNKNOWN|parse_error` if response can't be parsed
- [ ] Verification result added to WasteSignal: `LlmVerdict`, `LlmReason` fields
- [ ] Text output shows `[LLM: WASTE — <reason>]` or `[LLM: NOT_WASTE]`
- [ ] JSON output includes `llm_verification` object
- [ ] Network errors are handled gracefully (retry once, skip on failure)
- [ ] Rate limited: max 1 request/second (OpenRouter free tier)
- [ ] No API key stored in env/files — must be passed via `--llm-key` flag each time
- [ ] Zero-cost when `--llm-verify` is not used (no network calls)

## Dependencies

- **Must merge first:** PR18 (anomaly detection — anomaly scores help prioritize which signals to verify)
- **External dependencies:** None (stdlib `net/http` only)
- **Can be parallel with:** None (uses PR18 output ordering to select top-N)
- **Breaking changes / Migrations needed:** New WasteSignal fields. New CLI flags.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr19-llm-verification`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/llm_verify.go` | LLM client, prompt builder, response parser | New file |
| `analyze/llm_verify_test.go` | Prompt construction, response parsing, error handling | New file |
| `analyze/waste.go` | Add LlmVerdict/LlmReason to WasteSignal | Modify |
| `cmd/root.go` | Add `--llm-verify`, `--llm-key`, `--llm-confirm`, `--llm-max-verifications`, `--llm-model` flags | Modify |
| `config/config.go` | Add llm config section | Modify |
| `config/config_test.go` | Test defaults | Modify |
| `output/text.go` | Display LLM verdict in signal output | Modify |
| `output/json.go` | Serialize llm_verification | Modify |
| `output/text_test.go` | Golden file updates (or skip — LLM output varies) | Modify |

---

## Implementation

### `analyze/llm_verify.go` — LLM client

```
Type: LlmConfig
Fields:
  APIKey      string
  Model       string    // default: "anthropic/claude-haiku-4.5"
  APIURL      string    // default: "https://openrouter.ai/api/v1/chat/completions"
  MaxSignals  int       // default: 20
  HTTPClient  *http.Client

Type: LlmVerdict
Fields:
  SessionID string `json:"session_id"`
  Verdict   string `json:"verdict"`    // "WASTE", "NOT_WASTE", "UNKNOWN"
  Reason    string `json:"reason"`
  CostUSD   float64 `json:"cost_usd"`  // API call cost
  Error     string `json:"error,omitempty"`

Functions:
  VerifySignals(signals []WasteSignal, config LlmConfig) ([]LlmVerdict, error)
    → Sort signals by potential savings desc
    → Take top config.MaxSignals
    → For each signal: buildPrompt → callLLM → parseResponse
    → Rate limit: time.Sleep(1 * time.Second) between calls
    → Return []LlmVerdict

  buildPrompt(s WasteSignal) string
    → Construct prompt from signal fields

  callLLM(prompt string, config LlmConfig) (string, error)
    → POST to OpenRouter API
    → Parse JSON response
    → Return content string

  parseResponse(response string) (verdict string, reason string)
    → Match "WASTE|reason" or "NOT_WASTE|reason" or "UNKNOWN|reason"
    → Fallback: if no match, verdict="UNKNOWN", reason="parse_error: <first 100 chars>"

  estimateCost(signals []WasteSignal, model string) float64
    → Rough estimate: len(signals) * (500 input tokens * $0.0000008 + 50 output tokens * $0.000004)
    → ~$0.02 per signal with haiku
```

### Prompt template

```
You are an AI agent waste analyzer. Review this coding session for wasteful behavior.

Session: {sessionID}
Cost: ${cost}
Model: {model}
Input tokens: {inputTokens} ({formatTokens(inputTokens)})
Output tokens: {outputTokens} ({formatTokens(outputTokens)})
Output/input ratio: {ratio}
Subagents: {subagentCount} ({overheadPct}% overhead)
Project: {project}
Date: {date}
Sessions that day: {sessionCountToday}
Signal severity: {severity}
Signal reason: {reason}
Signal detail: {detail}

Common waste patterns to check:
- Context bloat (reading too many files unnecessarily)
- Tool-call loops (repeated calls returning same/similar results)
- Over-delegation (subagents for trivial tasks)
- Model mismatch (expensive model for simple task)
- Runaway generation (output growing over time, self-reinforcing)

Reply EXACTLY in this format:
WASTE|<reason>   (if this session represents wasted tokens/money)
NOT_WASTE|<reason>   (if this is legitimate usage)

Examples:
WASTE|context bloat from repeated file reads
NOT_WASTE|code review session, inherently high-input/low-output
WASTE|subagent over-delegation: 5 subagents for 10 lines of code
```

### API call

```go
type openRouterRequest struct {
    Model    string          `json:"model"`
    Messages []chatMessage   `json:"messages"`
    MaxTokens int            `json:"max_tokens"`
}

type chatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type openRouterResponse struct {
    Choices []struct {
        Message struct {
            Content string `json:"content"`
        } `json:"message"`
    } `json:"choices"`
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
    } `json:"usage"`
}

func callLLM(prompt string, config LlmConfig) (string, error) {
    body := openRouterRequest{
        Model: config.Model,
        Messages: []chatMessage{
            {Role: "user", Content: prompt},
        },
        MaxTokens: 200,
    }
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", config.APIURL, bytes.NewReader(jsonBody))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+config.APIKey)
    req.Header.Set("HTTP-Referer", "https://github.com/ethanhanguyen/burnwatch")
    req.Header.Set("X-Title", "Burnwatch")

    resp, err := config.HTTPClient.Do(req)
    // ... handle errors, parse response ...
}
```

### `analyze/waste.go` — WasteSignal extension

```go
type WasteSignal struct {
    // ... existing ...
    LlmVerdict string `json:"llm_verdict,omitempty"`   // "WASTE", "NOT_WASTE", "UNKNOWN"
    LlmReason  string `json:"llm_reason,omitempty"`    // diagnosis from LLM
}
```

### `output/text.go` — Display verdict

```
  HIGH ses_abc123 (project): $94510.79 — 33.2x project baseline
    Model: claude-sonnet-4-6, 30.4K in / 306.2K out
    [LLM: WASTE — repeated file reads causing context bloat; agent read the same file 12 times]
    → Investigate session for unnecessary loops or re-prompts. Potential savings: $91666.30
```

### cmd/root.go — New flags

```go
flag.BoolVar(&flags.LlmVerify, "llm-verify", false, "Verify top-N waste signals with LLM")
flag.StringVar(&flags.LlmKey, "llm-key", "", "OpenRouter API key for LLM verification")
flag.BoolVar(&flags.LlmConfirm, "llm-confirm", false, "Confirm LLM verification (bypasses cost prompt)")
flag.IntVar(&flags.LlmMaxVerifications, "llm-max-verifications", 20, "Max signals to verify (1-50)")
flag.StringVar(&flags.LlmModel, "llm-model", "anthropic/claude-haiku-4.5", "Model for verification")

// In Execute(), after DetectWaste:
if flags.LlmVerify {
    if flags.LlmKey == "" {
        fmt.Fprintln(os.Stderr, "Error: --llm-key is required with --llm-verify")
        os.Exit(1)
    }
    if flags.LlmMaxVerifications < 1 || flags.LlmMaxVerifications > 50 {
        fmt.Fprintln(os.Stderr, "Error: --llm-max-verifications must be 1-50")
        os.Exit(1)
    }
    estimatedCost := analyze.EstimateLLMCost(signals, flags.LlmModel, flags.LlmMaxVerifications)
    if !flags.LlmConfirm {
        fmt.Printf("LLM verification will verify %d signals using %s.\n", 
            min(flags.LlmMaxVerifications, len(signals)), flags.LlmModel)
        fmt.Printf("Estimated cost: $%.2f\n", estimatedCost)
        fmt.Print("Continue? (y/N): ")
        var answer string
        fmt.Scanln(&answer)
        if strings.ToLower(answer) != "y" {
            fmt.Println("Aborted.")
            return
        }
    }
    verdicts, err := analyze.VerifySignals(signals, analyze.LlmConfig{...})
    if err != nil {
        fmt.Fprintf(os.Stderr, "LLM verification error: %v\n", err)
    }
    // Merge verdicts into signals
    for i := range signals {
        for _, v := range verdicts {
            if signals[i].SessionID == v.SessionID {
                signals[i].LlmVerdict = v.Verdict
                signals[i].LlmReason = v.Reason
                break
            }
        }
    }
}
```

### Config additions

```go
type LlmConfig struct {
    Model           string `toml:"model"`
    MaxVerifications int   `toml:"max_verifications"`
    APIURL          string `toml:"api_url"`   // hidden: not exposed in TOML, only CLI flag
}

// Defaults:
Llm{
    Model:           "anthropic/claude-haiku-4.5",
    MaxVerifications: 20,
    APIURL:          "https://openrouter.ai/api/v1/chat/completions",
}
```

---

## Test Requirements

1. **`analyze/llm_verify_test.go`**:
   - Prompt contains all required fields (sessionID, cost, tokens, model, etc.)
   - Prompt length <2000 characters (fits in small context window)
   - Parse "WASTE|context bloat" → verdict=WASTE, reason=context bloat
   - Parse "NOT_WASTE|legitimate code review" → verdict=NOT_WASTE
   - Parse "UNKNOWN|unclear" → verdict=UNKNOWN
   - Parse "garbage text without pipe" → verdict=UNKNOWN, reason=parse_error
   - Parse "WASTE|multi|pipe" → verdict=WASTE, reason=multi|pipe (only split on first pipe)
   - Empty response → verdict=UNKNOWN
   - estimateCost: 20 signals → ~$0.40
   - estimateCost: 0 signals → $0.00
   - Sort by potential savings: top signal is the one with highest cost × savingsRatio

2. **`cmd/` — Manual test:**
   - Not easily testable without API key. Use `--llm-verify --llm-key $OPENROUTER_KEY --llm-confirm` manually.
   - Document manual test procedure in PR description.

3. Coverage target: >=90% on new code (all parse/estimate logic; API call excluded via interface mock)

---

## Approach

1. Define `LlmConfig`, `LlmVerdict`, parse/estimate functions
2. Write tests (RED → GREEN)
3. Implement API call function (with interface for testability)
4. Build prompt template
5. Add WasteSignal fields
6. Wire CLI flags + config
7. Add output display
8. Manual integration test with real API key
9. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr19-llm-verification`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: LLM verification for top-N waste signals`
- [ ] Push to branch `pr19-llm-verification`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- No new dependencies. `net/http` + `encoding/json` from stdlib for API calls.
- OpenRouter free tier allows ~20 requests/minute. Rate limit of 1/second stays well within this.
- `--llm-verify` without `--llm-confirm` shows cost estimate and prompts for confirmation. `--llm-confirm` skips the prompt (for scripting).
- API key is never written to disk, env, or config. Must be passed each time.
- `--llm-model` accepts any OpenRouter model ID. Default `anthropic/claude-haiku-4.5` is cheap and good at classification.
- The prompt template explicitly asks for the WASTE|NOT_WASTE format. This is crucial for reliable parsing.
- If the API returns an error, verdict is set to UNKNOWN and the signal is still displayed (just without LLM label).
