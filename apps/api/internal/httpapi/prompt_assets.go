package httpapi

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed prompts/*.md
var embeddedPrompts embed.FS

func mustReadEmbeddedPrompt(path string) string {
	content, err := embeddedPrompts.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(content))
}

func mustFormatEmbeddedPrompt(path string, args ...any) string {
	return strings.TrimSpace(fmt.Sprintf(mustReadEmbeddedPrompt(path), args...))
}
