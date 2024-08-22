// OpenAI.com barebones client
package openai

import (
	"context"

	"github.com/function61/gokit/net/http/ezhttp"
)

const (
	ModelGPT4o = "gpt-4o"
)

type ChatMessage struct {
	Role    string  `json:"role"`
	Content string  `json:"content"`
	Refusal *string `json:"refusal,omitempty"`
}

func SimpleChatCompletionReq(prompt string) ChatCompletionReq {
	return ChatCompletionReq{
		Model: ModelGPT4o,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
}

type ChatCompletionReq struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatCompletionRes struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

func ChatCompletion(ctx context.Context, req ChatCompletionReq, apiKey string) (*ChatCompletionRes, error) {
	res := &ChatCompletionRes{}
	_, err := ezhttp.Post(ctx, "https://api.openai.com/v1/chat/completions", ezhttp.AuthBearer(apiKey), ezhttp.SendJSON(req), ezhttp.RespondsJSONAllowUnknownFields(res))
	return res, err
}
