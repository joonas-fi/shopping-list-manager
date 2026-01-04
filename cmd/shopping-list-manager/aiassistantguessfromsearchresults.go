package main

// Uses an AI assistant to extract product name from search result page titles

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	. "github.com/function61/gokit/builtin"
	"github.com/joonas-fi/shopping-list-manager/pkg/openai"
)

func useAIAssistantToGuessProductDetailsFromSearchResults(ctx context.Context, searchResults []string, link string, logger *slog.Logger) (*productDetails, error) {
	withErr := func(err error) (*productDetails, error) {
		return nil, fmt.Errorf("useAIAssistantToGuessProductDetailsFromSearchResults: %w", err)
	}

	aiProviderAPIKey := cmp.Or(os.Getenv("AI_PROVIDER_API_KEY"), os.Getenv("OPENAI_API_KEY"))
	if err := ErrorIfUnset(aiProviderAPIKey == "", "aiProviderAPIKey"); err != nil {
		return withErr(err)
	}

	prompt, answerMatchers := makePrompt(searchResults)

	res, err := openai.NewGoogle(aiProviderAPIKey).ChatCompletion(ctx, openai.SimpleChatCompletionReq(prompt, "gemini-2.5-flash"))
	if err != nil {
		return withErr(err)
	}

	switch len(res.Choices) {
	case 0:
		return withErr(errors.New("UNEXPECTED: 0 choices in response"))
	case 1:
		// good
	default:
		logger.Warn("AI agent returned multiple choices; indicates uncertainty of its response", "numChoices", len(res.Choices))
	}

	bestChoice := res.Choices[0].Message // just assumption

	if bestChoice.Refusal != nil {
		return withErr(fmt.Errorf("AI agent refused, reason: %s", *bestChoice.Refusal))
	}

	answer := bestChoice.Content

	resolve := func(re *regexp.Regexp) string {
		match := re.FindStringSubmatch(answer)
		if match == nil {
			return ""
		}
		return match[1]
	}

	now := time.Now().UTC()

	details := productDetails{
		Name:            resolve(answerMatchers.ProductName),
		ProductType:     resolve(answerMatchers.ProductType),
		ProductCategory: resolve(answerMatchers.ProductCategory),
		Notes:           resolve(answerMatchers.Notes),
		Link:            link,
		FirstScanned:    &now,
		LastScanned:     &now,
	}

	if details.Name == "" {
		return nil, fmt.Errorf("AI agent failed to resolve product name. notes: %s", resolve(answerMatchers.Notes))
	}

	logger.Debug("useAIAssistantToGuessProductDetailsFromSearchResults", "Name", details.Name, "ProductType", details.ProductType, "ProductCategory", details.ProductCategory)

	return &details, nil
}

type promptAnswerMatchers struct {
	ProductName     *regexp.Regexp
	ProductType     *regexp.Regexp
	ProductCategory *regexp.Regexp
	Notes           *regexp.Regexp
}

func makePrompt(searchResults []string) (string, promptAnswerMatchers) {
	promptTemplate := `I have list of web search result page titles (one per line), try to guess what is the product name (usually it's a grocery store item, but not always):

CODEFENCE
%s
CODEFENCE

If search results are in multiple languages, prefer Finnish and then English.

This will be variable *ProductName*. It may or may not be in Finnish.

I want also to resolve product type (*ProductType*) and product category (*ProductCategory*) for the product name. Product type example is just "Milk" and product category is one of these rigid options:

- %s

Please respond succinctly in this format:

CODEFENCE
Product name: <ProductName>
Product type: <ProductType>
Product category: <ProductCategory>
Notes: <notes if you have any additional notes, for example if you're unsure of some detail>
CODEFENCE

For the category if you're unsure choose "Other" and include in notes why you're unsure.
`

	return fmt.Sprintf(strings.ReplaceAll(promptTemplate, "CODEFENCE", "```"),
			strings.Join(searchResults, "\n"),
			strings.Join(productCategoriesLabelsOnly, "\n- "),
		), promptAnswerMatchers{
			ProductName:     regexp.MustCompile(`Product name: ([^\n]+)`),
			ProductType:     regexp.MustCompile(`Product type: ([^\n]+)`),
			ProductCategory: regexp.MustCompile(`Product category: ([^\n]+)`),
			Notes:           regexp.MustCompile(`Notes: ([^\n]+)`),
		}
}
