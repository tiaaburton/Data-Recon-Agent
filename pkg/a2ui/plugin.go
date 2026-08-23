package a2ui

import (
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
	rawJSONCodeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*\\{\\s*\"version\":\\s*\"v0\\.9\".*?\\}\\s*```")
	a2aTagRegex           = regexp.MustCompile("(?s)<a2a_datapart_json>.*?</a2a_datapart_json>")

	pendingMu  sync.Mutex
	pendingMap = make(map[string][]any)
)

// BuildA2UIDataPart creates a native A2A DataPart map for Gemini Enterprise / Discovery Engine.
func BuildA2UIDataPart(a2uiMessage any) map[string]any {
	return map[string]any{
		"kind": "data",
		"metadata": map[string]any{
			"mimeType": A2UIMimeType,
		},
		"data": a2uiMessage,
	}
}

// WrapA2UIDataPartText wraps an A2UI message inside the official A2A DataPart envelope and sentinel tags
// for legacy text-fallback rendering.
func WrapA2UIDataPartText(a2uiMessage any) string {
	dataPartEnvelope := BuildA2UIDataPart(a2uiMessage)
	jsonBytes, err := json.Marshal(dataPartEnvelope)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("<a2a_datapart_json>%s</a2a_datapart_json>", string(jsonBytes))
}

// CleanRawJSONText removes raw A2UI JSON code blocks and legacy <a2a_datapart_json> tags from conversational text parts.
func CleanRawJSONText(text string) string {
	if text == "" {
		return ""
	}
	cleaned := a2aTagRegex.ReplaceAllString(text, "")
	cleaned = rawJSONCodeBlockRegex.ReplaceAllString(cleaned, "")
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

// NewA2UIPartsPlugin returns an ADK plugin that captures A2UI tool envelopes
// and cleans conversational text parts so Gemini Enterprise renders interactive UI cards without raw JSON leaks.
func NewA2UIPartsPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "a2ui_parts_plugin",
		OnEventCallback: func(ctx agent.InvocationContext, event *session.Event) (*session.Event, error) {
			if event == nil || event.Content == nil || len(event.Content.Parts) == 0 {
				return nil, nil
			}

			sessKey := getSessionKey(ctx)
			newParts := make([]*genai.Part, 0, len(event.Content.Parts))
			modified := false

			for _, part := range event.Content.Parts {
				// Clean text parts to avoid showing raw machine JSON or datapart tags in chat bubble
				if part.Text != "" {
					cleaned := CleanRawJSONText(part.Text)
					if cleaned != part.Text {
						modified = true
						if cleaned != "" {
							newParts = append(newParts, &genai.Part{Text: cleaned})
						}
						continue
					}
				}

				// Check function responses for A2UI payload envelopes
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

						// Store messages for delivery on the model turn in SSE streaming
						if len(messages) > 0 {
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
				return event, nil
			}

			return nil, nil
		},
	})
}
