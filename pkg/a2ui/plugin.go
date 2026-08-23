package a2ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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
)

// CleanRawJSONText removes raw A2UI JSON code blocks and leaked datapart tags from conversational text parts.
func CleanRawJSONText(text string) string {
	if text == "" {
		return ""
	}
	cleaned := a2aTagRegex.ReplaceAllString(text, "")
	cleaned = rawJSONCodeBlockRegex.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

// NewA2UIPartsPlugin returns an ADK plugin that emits native A2A DataParts
// so that Gemini Enterprise natively renders interactive A2UI component cards in the chat UI.
func NewA2UIPartsPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "a2ui_parts_plugin",
		OnEventCallback: func(ctx agent.InvocationContext, event *session.Event) (*session.Event, error) {
			if event == nil || event.Content == nil || len(event.Content.Parts) == 0 {
				return nil, nil
			}

			newParts := make([]*genai.Part, 0, len(event.Content.Parts)+4)
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

						// Create native A2A DataPart GenAI parts with <a2a_datapart_json> wrapper
						for _, msg := range messages {
							msgBytes, err := json.Marshal(msg)
							if err == nil && len(msgBytes) > 0 {
								wrappedData := fmt.Sprintf("<a2a_datapart_json>%s</a2a_datapart_json>", string(msgBytes))
								newParts = append(newParts, &genai.Part{
									InlineData: &genai.Blob{
										MIMEType: A2UIMimeType,
										Data:     []byte(wrappedData),
									},
								})
							}
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
