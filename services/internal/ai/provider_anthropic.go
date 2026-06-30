package ai

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicModel is the default Claude model for the secondary/fallback
// provider. Per Anthropic guidance, default to the latest Opus tier.
const anthropicModel = "claude-opus-4-8"

// anthropicProvider is the Claude (secondary) provider, using the official SDK.
type anthropicProvider struct {
	apiKey string
	client anthropic.Client
}

func newAnthropicProvider(apiKey string) *anthropicProvider {
	p := &anthropicProvider{apiKey: apiKey}
	if apiKey != "" {
		p.client = anthropic.NewClient(option.WithAPIKey(apiKey))
	}
	return p
}

func (a *anthropicProvider) Name() string   { return "claude" }
func (a *anthropicProvider) Available() bool { return a.apiKey != "" }

func (a *anthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(anthropicModel),
		MaxTokens: int64(req.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: req.System}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
	})
	if err != nil {
		return CompletionResponse{}, err
	}

	var text strings.Builder
	for _, block := range msg.Content {
		text.WriteString(block.Text)
	}
	return CompletionResponse{
		Text:      text.String(),
		Provider:  a.Name(),
		TokensIn:  int(msg.Usage.InputTokens),
		TokensOut: int(msg.Usage.OutputTokens),
	}, nil
}
