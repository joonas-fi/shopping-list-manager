// OpenAI.com barebones client
package openai

import (
	"context"

	"github.com/function61/gokit/net/http/ezhttp"
)

const (
	ModelGPT4o = "gpt-4o"
)

// NOTE: I don't recommend giving money to OpenAI: https://bsky.app/profile/joonas.fi/post/3lxwfysweu22u
func New(apiKey string) Client {
	return Client{
		apiKey:  apiKey,
		baseurl: "https://api.openai.com/v1/",
	}
}

func NewGoogle(apiKey string) Client {
	return Client{
		apiKey:  apiKey,
		baseurl: "https://generativelanguage.googleapis.com/v1beta/openai/",
	}
}

type ChatMessage struct {
	Role    string  `json:"role"`
	Content string  `json:"content"`
	Refusal *string `json:"refusal,omitempty"`
}

func SimpleChatCompletionReq(prompt string, model string) ChatCompletionReq {
	return ChatCompletionReq{
		Model: model,
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

type Client struct {
	baseurl string
	apiKey  string
}

func (c Client) ChatCompletion(ctx context.Context, req ChatCompletionReq) (*ChatCompletionRes, error) {
	res := &ChatCompletionRes{}
	_, err := ezhttp.Post(ctx, c.baseurl+"chat/completions", ezhttp.AuthBearer(c.apiKey), ezhttp.SendJSON(req), ezhttp.RespondsJSONAllowUnknownFields(res))
	return res, err
}
