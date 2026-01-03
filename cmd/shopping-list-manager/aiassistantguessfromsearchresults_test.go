package main

import (
	_ "embed"
	"testing"

	"github.com/function61/gokit/testing/assert"
)

//go:embed testdata/expected_prompt.txt
var expectedPrompt string

func TestMakePrompt(t *testing.T) {
	prompt, _ := makePrompt([]string{"Tacokastike", "Tacokastike 100 g"})

	assert.Equal(t, prompt, expectedPrompt)
}
