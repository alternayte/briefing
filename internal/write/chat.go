package write

import (
	"context"
	"errors"

	"github.com/alternayte/briefing/internal/platform"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// errCut reports an answer that stopped at the token limit.
var errCut = errors.New("the answer was cut at the token limit")

// chat sends one chat completion and returns the text of the answer. With
// jsonOut the model must answer with a JSON object.
func chat(ctx context.Context, or *platform.OpenRouter, model, stage string, messages []message, maxTokens int, jsonOut bool) (string, error) {
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
	if err := or.Post(ctx, "/api/v1/chat/completions", stage, model, req, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("the model returned no choice")
	}
	content := resp.Choices[0].Message.Content
	if resp.Choices[0].FinishReason == "length" {
		return content, errCut
	}
	return content, nil
}
