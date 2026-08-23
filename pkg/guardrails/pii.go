package guardrails

import (
	"regexp"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/genai"
)

var (
	ssnRegex        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	creditCardRegex = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`)
	emailRegex      = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	phoneRegex      = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	apiKeyRegex     = regexp.MustCompile(`(?i)(Bearer\s+[A-Za-z0-9_\-\.]{20,}|AIza[0-9A-Za-z\-_]{20,}|ghp_[0-9a-zA-Z]{36}|sk-[a-zA-Z0-9]{20,})`)
)

// RedactPII scrubs personally identifiable information (PII) and credentials from text.
func RedactPII(input string) string {
	if input == "" {
		return ""
	}

	result := ssnRegex.ReplaceAllString(input, "[REDACTED_SSN]")
	result = creditCardRegex.ReplaceAllString(result, "[REDACTED_CREDIT_CARD]")
	result = apiKeyRegex.ReplaceAllString(result, "[REDACTED_API_KEY]")
	result = emailRegex.ReplaceAllString(result, "[REDACTED_EMAIL]")
	result = phoneRegex.ReplaceAllString(result, "[REDACTED_PHONE]")

	return result
}

// RedactContent scrubs PII across all parts in a GenAI content object.
func RedactContent(content *genai.Content) (*genai.Content, bool) {
	if content == nil || len(content.Parts) == 0 {
		return content, false
	}

	modified := false
	newParts := make([]*genai.Part, 0, len(content.Parts))

	for _, part := range content.Parts {
		if part.Text != "" {
			cleaned := RedactPII(part.Text)
			if cleaned != part.Text {
				modified = true
				newParts = append(newParts, &genai.Part{Text: cleaned})
				continue
			}
		}
		newParts = append(newParts, part)
	}

	if modified {
		return &genai.Content{
			Role:  content.Role,
			Parts: newParts,
		}, true
	}

	return content, false
}

// NewPIIGuardrailPlugin creates an ADK plugin that enforces real-time DLP/PII redaction.
func NewPIIGuardrailPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "pii_guardrail_plugin",
		OnUserMessageCallback: func(ctx agent.InvocationContext, content *genai.Content) (*genai.Content, error) {
			if content == nil {
				return nil, nil
			}

			cleanedContent, modified := RedactContent(content)
			if modified {
				return cleanedContent, nil
			}
			return nil, nil
		},
	})
}
