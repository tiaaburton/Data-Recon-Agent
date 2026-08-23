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
)

// CleanRawJSONText removes raw A2UI JSON code blocks from conversational text parts.
func CleanRawJSONText(text string) string {
	if text == "" {
		return ""
	}
	if !strings.Contains(text, "\"version\": \"v0.9\"") && !strings.Contains(text, "\"createSurface\"") {
		return text
	}

	cleaned := rawJSONCodeBlockRegex.ReplaceAllString(text, "")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

// NewA2UIPartsPlugin returns an ADK plugin that emits native A2A DataParts (<a2a_datapart_json>)
// so that Gemini Enterprise natively renders interactive A2UI component cards in the chat UI.
func NewA2UIPartsPlugin() (*plugin.Plugin, error) {
	return plugin.New(plugin.Config{
		Name: "a2ui_parts_plugin",
		OnEventCallback: func(ctx agent.InvocationContext, event *session.Event) (*session.Event, error) {
			if event == nil || event.Content == nil || len(event.Content.Parts) == 0 {
				return nil, nil
			}

			newParts := make([]*genai.Part, 0, len(event.Content.Parts)+2)
			modified := false

			for _, part := range event.Content.Parts {
				// Clean text parts to avoid showing raw machine JSON in chat bubble
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
					var a2uiJSONStr string
					if payload, exists := respMap["a2ui_json"]; exists {
						if s, isStr := payload.(string); isStr {
							a2uiJSONStr = s
						} else {
							b, _ := json.Marshal(payload)
							a2uiJSONStr = string(b)
						}
					} else if payload, exists := respMap["a2ui_payload"]; exists {
						if s, isStr := payload.(string); isStr {
							a2uiJSONStr = s
						} else {
							b, _ := json.Marshal(payload)
							a2uiJSONStr = string(b)
						}
					}

					if a2uiJSONStr != "" {
						// Emit native A2A DataPart with wire protocol wrapper for Gemini Enterprise
						wrappedData := fmt.Sprintf("<a2a_datapart_json>%s</a2a_datapart_json>", a2uiJSONStr)
						dataPart := &genai.Part{
							InlineData: &genai.Blob{
								MIMEType: A2UIMimeType,
								Data:     []byte(wrappedData),
							},
						}

						newParts = append(newParts, dataPart)
						modified = true

						// Sanitize function response to keep large JSON from polluting conversational context
						sanitizedResp := make(map[string]any)
						for k, v := range respMap {
							if k != "a2ui_json" && k != "a2ui_payload" {
								sanitizedResp[k] = v
							}
						}
						sanitizedResp["a2ui_status"] = "RENDERED_VIA_A2A_DATAPART"
						part.FunctionResponse.Response = sanitizedResp
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
