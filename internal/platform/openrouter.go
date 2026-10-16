package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/alternayte/drel"
)

// OpenRouterBase is the public OpenRouter API.
const OpenRouterBase = "https://openrouter.ai"

// OpenRouter is the one gateway to every model call. It records the usage
// of each call in llm_usage, which the spend cap reads.
type OpenRouter struct {
	base   string
	key    string
	client *http.Client
	engine *drel.Engine
	now    func() time.Time
}

// NewOpenRouter builds the client. A nil engine records no usage.
func NewOpenRouter(base, key string, client *http.Client, engine *drel.Engine, now func() time.Time) *OpenRouter {
	return &OpenRouter{base: base, key: key, client: client, engine: engine, now: now}
}

// APIError is an error answer of OpenRouter.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("openrouter: %d %s", e.Status, e.Message) }

// Retryable reports whether a later attempt can succeed.
func (e *APIError) Retryable() bool { return e.Status == http.StatusTooManyRequests || e.Status >= 500 }

// usageWire covers the usage shapes of the decisions, embeddings and chat
// endpoints.
type usageWire struct {
	Usage struct {
		InputTokens      int     `json:"input_tokens"`
		OutputTokens     int     `json:"output_tokens"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

// Post sends in as JSON to path, decodes the answer into out, and records the
// call under stage and model.
func (o *OpenRouter) Post(ctx context.Context, path, stage, model string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("openrouter: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+o.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "Briefing")
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("openrouter: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &env)
		return &APIError{Status: resp.StatusCode, Message: env.Error.Message}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("openrouter: decode %s: %w", path, err)
	}
	var u usageWire
	_ = json.Unmarshal(raw, &u)
	in1, out1 := u.Usage.InputTokens+u.Usage.PromptTokens, u.Usage.OutputTokens+u.Usage.CompletionTokens
	if err := o.record(ctx, stage, model, in1, out1, u.Usage.Cost); err != nil {
		// The call succeeded and is paid for; a lost usage row must not lose
		// its result.
		slog.ErrorContext(ctx, "record llm usage", "stage", stage, "error", err)
	}
	return nil
}

func (o *OpenRouter) record(ctx context.Context, stage, model string, in, out int, cost float64) error {
	if o.engine == nil {
		return nil
	}
	day := o.now().UTC().Format(time.DateOnly)
	return o.engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `
			INSERT INTO llm_usage (day, provider, stage, model, calls, input_tokens, output_tokens, cost_usd)
			VALUES ($1, 'openrouter', $2, $3, 1, $4, $5, $6)
			ON CONFLICT (day, provider, stage, model) DO UPDATE SET
				calls = llm_usage.calls + 1,
				input_tokens = llm_usage.input_tokens + EXCLUDED.input_tokens,
				output_tokens = llm_usage.output_tokens + EXCLUDED.output_tokens,
				cost_usd = llm_usage.cost_usd + EXCLUDED.cost_usd`,
			day, stage, model, in, out, cost)
		return err
	})
}

// SpentOn returns the recorded model cost of one UTC day in USD.
func (o *OpenRouter) SpentOn(ctx context.Context, day time.Time) (float64, error) {
	var spent float64
	err := o.engine.WithTx(ctx, func(ctx context.Context) error {
		return drel.MustFromContext(ctx).QueryRow(ctx,
			`SELECT COALESCE(sum(cost_usd), 0)::float8 FROM llm_usage WHERE day = $1`, day.UTC().Format(time.DateOnly)).Scan(&spent)
	})
	return spent, err
}

// ChatMessage is one message of a chat completion.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ErrCut reports a chat answer that stopped at the token limit.
var ErrCut = errors.New("the answer was cut at the token limit")

// Chat sends one chat completion and returns the text of the answer. With
// jsonOut the model must answer with a JSON object. On ErrCut the text is
// the part that was written.
func (o *OpenRouter) Chat(ctx context.Context, model, stage string, messages []ChatMessage, maxTokens int, jsonOut bool) (string, error) {
	req := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.2,
		"max_tokens":  maxTokens,
		// A reasoning model would spend the output budget before the answer.
		"reasoning": map[string]bool{"enabled": false},
		"usage":     map[string]bool{"include": true},
	}
	if jsonOut {
		req["response_format"] = map[string]string{"type": "json_object"}
	}
	var resp struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := o.Post(ctx, "/api/v1/chat/completions", stage, model, req, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("openrouter: the model returned no choice")
	}
	content := resp.Choices[0].Message.Content
	if resp.Choices[0].FinishReason == "length" {
		return content, ErrCut
	}
	return content, nil
}
