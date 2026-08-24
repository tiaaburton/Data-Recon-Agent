package a2ui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// A2UIMimeType is the official MIME type for A2UI / A2A UI protocol streaming.
const A2UIMimeType = "application/json+a2ui"

var (
	rawJSONCodeBlockRegex = regexp.MustCompile(`(?s)` + "```" + `(?:json)?\s*[\{\[].*?(?:validated_a2ui_json|beginRendering|surfaceUpdate|"version":|"status":).*?[\}\]]\s*` + "```")
	a2aTagRegex           = regexp.MustCompile(`(?s)<a2a_datapart_json>.*?</a2a_datapart_json>`)
	surfaceSuffixRegex    = regexp.MustCompile(`-[0-9a-f]{8}$`)

	pendingMu  sync.Mutex
	pendingMap = make(map[string][]any)
)

func generateRandomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "a1b2c3d4"
	}
	return hex.EncodeToString(bytes)
}

// UniquifySurfaceIDs ensures every A2UI payload has a unique surfaceId to allow multiple interactive cards in a session.
func UniquifySurfaceIDs(messages []any) []any {
	if len(messages) == 0 {
		return messages
	}
	suffix := "-" + generateRandomHex(4)
	mapping := make(map[string]string)

	var collect func(v any)
	collect = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if sID, ok := t["surfaceId"].(string); ok && sID != "" {
				if !surfaceSuffixRegex.MatchString(sID) && mapping[sID] == "" {
					mapping[sID] = sID + suffix
				}
			}
			for _, val := range t {
				collect(val)
			}
		case []any:
			for _, val := range t {
				collect(val)
			}
		}
	}

	var replace func(v any)
	replace = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if sID, ok := t["surfaceId"].(string); ok && sID != "" {
				if newID, exists := mapping[sID]; exists {
					t["surfaceId"] = newID
				}
			}
			for _, val := range t {
				replace(val)
			}
		case []any:
			for _, val := range t {
				replace(val)
			}
		}
	}

	for _, m := range messages {
		collect(m)
		replace(m)
	}
	return messages
}

// BuildA2UIDataPart creates a native A2A DataPart map for Gemini Enterprise / Discovery Engine.
func BuildA2UIDataPart(a2uiMessage any) map[string]any {
	return map[string]any{
		"kind":      "data",
		"mimeType":  A2UIMimeType,
		"mime_type": A2UIMimeType,
		"metadata": map[string]any{
			"mimeType":  A2UIMimeType,
			"mime_type": A2UIMimeType,
		},
		"data": a2uiMessage,
	}
}

// WrapA2UIDataPartText wraps an A2UI message inside the official A2A DataPart envelope and sentinel tags.
func WrapA2UIDataPartText(a2uiMessage any) string {
	dataPartEnvelope := BuildA2UIDataPart(a2uiMessage)
	jsonBytes, err := json.Marshal(dataPartEnvelope)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("<a2a_datapart_json>%s</a2a_datapart_json>", string(jsonBytes))
}

// CleanRawJSONText removes raw A2UI JSON code blocks and tool dump strings from conversational text,
// while strictly preserving valid <a2a_datapart_json> wire protocol blocks.
func CleanRawJSONText(text string) string {
	if text == "" {
		return ""
	}

	trimmed := strings.TrimSpace(text)
	if (strings.HasPrefix(trimmed, `{"status":`) || strings.HasPrefix(trimmed, `{"result":`) ||
		strings.HasPrefix(trimmed, `{"data":`) || strings.HasPrefix(trimmed, `{"is_sensitive":`)) &&
		!strings.Contains(text, "<a2a_datapart_json>") {
		return ""
	}

	// 1. Extract and protect <a2a_datapart_json> blocks with placeholders
	var datapartBlocks []string
	protected := a2aTagRegex.ReplaceAllStringFunc(text, func(m string) string {
		datapartBlocks = append(datapartBlocks, m)
		return fmt.Sprintf("__A2A_DATAPART_BLOCK_%d__", len(datapartBlocks)-1)
	})

	// 2. Strip code blocks containing raw JSON or A2UI keywords
	cleaned := rawJSONCodeBlockRegex.ReplaceAllString(protected, "")

	// 3. Restore protected <a2a_datapart_json> blocks
	for i, block := range datapartBlocks {
		cleaned = strings.ReplaceAll(cleaned, fmt.Sprintf("__A2A_DATAPART_BLOCK_%d__", i), block)
	}

	return strings.TrimSpace(cleaned)
}

// StorePendingA2UIMessages stores A2UI messages for a session.
func StorePendingA2UIMessages(sessKey string, messages []any) {
	if len(messages) == 0 {
		return
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingMap[sessKey] = messages
	if sessKey != "default" {
		pendingMap["default"] = messages
	}
}

// PopPendingA2UIMessages retrieves and clears pending A2UI messages for a session.
func PopPendingA2UIMessages(sessKey string) []any {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if msgs, ok := pendingMap[sessKey]; ok && len(msgs) > 0 {
		delete(pendingMap, sessKey)
		delete(pendingMap, "default")
		return msgs
	}
	if msgs, ok := pendingMap["default"]; ok && len(msgs) > 0 {
		delete(pendingMap, "default")
		return msgs
	}
	return nil
}

func getSessionKey(ctx agent.InvocationContext) string {
	if ctx != nil && ctx.Session() != nil {
		return ctx.Session().ID()
	}
	return "default"
}

// NewA2UIPartsPlugin returns an ADK plugin that captures A2UI tool envelopes,
// creates native A2A DataParts, and cleans conversational text parts so Gemini Enterprise renders interactive UI cards without raw JSON leaks.
func NewA2UIPartsPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "a2ui_parts_plugin",
		OnEventCallback: func(ctx agent.InvocationContext, event *session.Event) (*session.Event, error) {
			if event == nil {
				return nil, nil
			}

			sessKey := getSessionKey(ctx)
			modified := false

			// 1. Intercept FunctionResponse to capture A2UI envelopes and sanitize history
			if event.Content != nil && len(event.Content.Parts) > 0 {
				newParts := make([]*genai.Part, 0, len(event.Content.Parts))
				for _, part := range event.Content.Parts {
					if part.FunctionResponse != nil && part.FunctionResponse.Response != nil {
						respMap := part.FunctionResponse.Response
						var rawPayload any
						for k, v := range respMap {
							lowerKey := strings.ToLower(k)
							if lowerKey == "validated_a2ui_json" || lowerKey == "a2ui_payload" || lowerKey == "a2ui_json" {
								rawPayload = v
								break
							}
						}

						if rawPayload != nil {
							var messages []any
							if s, isStr := rawPayload.(string); isStr {
								var parsed any
								if err := json.Unmarshal([]byte(s), &parsed); err == nil {
									if list, isList := parsed.([]any); isList {
										messages = list
									} else if m, isMap := parsed.(map[string]any); isMap {
										messages = []any{m}
									}
								}
							} else if list, isList := rawPayload.([]any); isList {
								messages = list
							} else if listMap, isListMap := rawPayload.([]map[string]any); isListMap {
								for _, m := range listMap {
									messages = append(messages, m)
								}
							} else if m, isMap := rawPayload.(map[string]any); isMap {
								messages = []any{m}
							}

							if len(messages) > 0 {
								messages = UniquifySurfaceIDs(messages)
								// Store messages for delivery on the upcoming model turn
								StorePendingA2UIMessages(sessKey, messages)
							}

							sanitizedResp := make(map[string]any)
							for k, v := range respMap {
								if k != "a2ui_json" && k != "a2ui_payload" && k != "validated_a2ui_json" {
									sanitizedResp[k] = v
								}
							}
							sanitizedResp["status"] = "success"
							sanitizedResp["a2ui_status"] = "A2UI_SURFACE_SYNTHESIZED"
							part.FunctionResponse.Response = sanitizedResp
							modified = true
						}
					}
					newParts = append(newParts, part)
				}
				if modified {
					event.Content.Parts = newParts
				}
			}

			// 2. Attach pending A2UI DataParts to the Model turn event so Gemini Enterprise renders them
			isModelTurn := (event.LLMResponse.Content != nil && len(event.LLMResponse.Content.Parts) > 0) ||
				(event.Content != nil && event.Content.Role == "model" && len(event.Content.Parts) > 0)

			if isModelTurn {
				var targetParts []*genai.Part
				if event.LLMResponse.Content != nil {
					targetParts = event.LLMResponse.Content.Parts
				} else if event.Content != nil {
					targetParts = event.Content.Parts
				}

				// Clean text parts of raw JSON dumps
				cleanedParts := make([]*genai.Part, 0, len(targetParts))
				for _, part := range targetParts {
					if part.Text != "" {
						cleaned := CleanRawJSONText(part.Text)
						if cleaned != part.Text {
							modified = true
							if cleaned != "" {
								part.Text = cleaned
								cleanedParts = append(cleanedParts, part)
							}
							continue
						}
					}
					cleanedParts = append(cleanedParts, part)
				}

				// Attach A2A DataPart envelopes to the Model event
				if pending := PopPendingA2UIMessages(sessKey); len(pending) > 0 {
					for _, msg := range pending {
						dataPartText := WrapA2UIDataPartText(msg)
						if dataPartText != "" {
							cleanedParts = append(cleanedParts, &genai.Part{Text: dataPartText})
							modified = true
						}
					}
				}

				if event.LLMResponse.Content != nil {
					event.LLMResponse.Content.Parts = cleanedParts
				}
				if event.Content != nil {
					event.Content.Parts = cleanedParts
				}
			}

			if modified {
				return event, nil
			}

			return nil, nil
		},
	})
}
