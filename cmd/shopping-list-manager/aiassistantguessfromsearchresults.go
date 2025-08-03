package main

// Uses an AI assistant to extract product name from search result page titles

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/function61/gokit/os/osutil"
	"github.com/joonas-fi/shopping-list-manager/pkg/openai"
)

func useAIAssistantToGuessProductNameFromSearchResults(ctx context.Context, searchResults []string, logger *log.Logger) (string, error) {
	withErr := func(err error) (string, error) {
		return "", fmt.Errorf("useAIAssistantToGuessProductNameFromSearchResults: %w", err)
	}

	openaiAPIKey, err := osutil.GetenvRequired("OPENAI_API_KEY")
	if err != nil {
		return withErr(err)
	}

	input := strings.Join(searchResults, "\n")

	prompt := fmt.Sprintf("I have list of web search result page titles (one per line), try to guess what is the product name:\n```\n%s\n``` If search results are in multiple languages, prefer Finnish and then English. Respond with only the product name. If you don't have a guess, start you answer with the string `ERROR:`.", input)

	res, err := openai.ChatCompletion(ctx, openai.SimpleChatCompletionReq(prompt), openaiAPIKey)
	if err != nil {
		return withErr(err)
	}

	switch len(res.Choices) {
	case 0:
		return withErr(errors.New("UNEXPECTED: 0 choices in response"))
	case 1:
		// good
	default:
		logger.Printf("WARN: %d choices; this indicates AI agent is unsure of its response", len(res.Choices))
	}

	bestChoice := res.Choices[0].Message // just assumption

	if bestChoice.Refusal != nil {
		return withErr(fmt.Errorf("AI agent refused, reason: %s", *bestChoice.Refusal))
	}

	if strings.HasPrefix(bestChoice.Content, "ERROR:") {
		return withErr(fmt.Errorf("AI agent responded with error: %s", bestChoice.Content))
	}

	return bestChoice.Content, nil
}
