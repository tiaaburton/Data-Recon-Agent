// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package method

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"iter"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/model"
	"github.com/tiaaburton/Data-Recon-Agent/internal/agentengine/internal/models"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"google.golang.org/adk/v2/session"
)

type agentSpaceStreamResponse struct {
	Events    []simpleEvent `json:"events"`
	SessionID string        `json:"session_id"`
}

type streamAwareLLM struct{}

func (streamAwareLLM) Name() string {
	return "stream-aware-llm"
}

func (streamAwareLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if stream {
			if !yield(&model.LLMResponse{
				Content: genai.NewContentFromText("partial response", genai.RoleModel),
				Partial: true,
			}, nil) {
				return
			}
		}
		yield(&model.LLMResponse{
			Content: genai.NewContentFromText("final response", genai.RoleModel),
		}, nil)
	}
}

func TestDecodeStreamingAgentRunWithEventsRequest(t *testing.T) {
	payload := []byte(`{
		"class_method": "streaming_agent_run_with_events",
		"input": {
			"request_json": "{\"message\":{\"role\":\"user\",\"parts\":[{\"text\":\"Hi\"}]},\"session_id\":\"projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890\",\"user_id\":\"test-user@example.com\"}"
		}
	}`)
	var req models.StreamingAgentRunWithEventsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	got, requestedSessionID, err := decodeStreamingAgentRunWithEventsRequest(&req)
	if err != nil {
		t.Fatalf("decodeStreamingAgentRunWithEventsRequest() failed: %v", err)
	}

	want := &models.StreamingAgentRunWithEventsRunRequest{
		UserID:    "test-user@example.com",
		SessionID: "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890",
		Message: genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Hi"}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("decodeStreamingAgentRunWithEventsRequest() mismatch (-want +got):\n%s", diff)
	}
	if requestedSessionID != want.SessionID {
		t.Errorf("requestedSessionID = %q, want %q", requestedSessionID, want.SessionID)
	}
}

func TestEnsureBackendSession_CreateBackendSession(t *testing.T) {
	sessionService := session.InMemoryService()
	handler := NewStreamingAgentRunWithEventsHandler(&launcher.Config{SessionService: sessionService}, "app", "streaming_agent_run_with_events", "async_stream")
	req := &models.StreamingAgentRunWithEventsRunRequest{
		UserID:    "test-user@example.com",
		SessionID: "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890",
	}
	requestedSessionID := req.SessionID

	if err := handler.ensureBackendSession(t.Context(), req, requestedSessionID); err != nil {
		t.Fatalf("ensureBackendSession() failed: %v", err)
	}
	if req.SessionID == "" || req.SessionID == requestedSessionID {
		t.Fatalf("SessionID = %q, want generated backend session ID", req.SessionID)
	}

	got, err := sessionService.Get(t.Context(), &session.GetRequest{
		AppName:   "app",
		UserID:    "test-user@example.com",
		SessionID: req.SessionID,
	})
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Session.ID() != req.SessionID {
		t.Errorf("stored SessionID = %q, want %q", got.Session.ID(), req.SessionID)
	}
}

func TestEnsureBackendSession_ReuseReturnedBackendSession(t *testing.T) {
	sessionService := session.InMemoryService()
	created, err := sessionService.Create(t.Context(), &session.CreateRequest{
		AppName: "app",
		UserID:  "jan@example.com",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	handler := NewStreamingAgentRunWithEventsHandler(&launcher.Config{SessionService: sessionService}, "app", "streaming_agent_run_with_events", "async_stream")
	req := &models.StreamingAgentRunWithEventsRunRequest{
		UserID:    "jan@example.com",
		SessionID: created.Session.ID(),
	}

	if err := handler.ensureBackendSession(t.Context(), req, req.SessionID); err != nil {
		t.Fatalf("ensureBackendSession() failed: %v", err)
	}
	if req.SessionID != created.Session.ID() {
		t.Errorf("SessionID = %q, want existing backend session %q", req.SessionID, created.Session.ID())
	}
}

func TestStreamJSONL_AgentSpaceResponseEnvelope(t *testing.T) {
	const (
		appName           = "app"
		userID            = "test-user@example.com"
		externalSessionID = "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890"
	)

	a, err := llmagent.New(llmagent.Config{
		Name: "Echo",
		BeforeAgentCallbacks: []agent.BeforeAgentCallback{
			func(cc agent.Context) (*genai.Content, error) {
				return cc.UserContent(), nil
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamingAgentRunWithEventsHandler(config, appName, "streaming_agent_run_with_events", "async_stream")

	requestJSON := `{"message":{"role":"user","parts":[{"text":"Please"}]},"session_id":"` + externalSessionID + `","user_id":"` + userID + `"}`
	payload, err := json.Marshal(models.StreamingAgentRunWithEventsRequest{
		ClassMethod: "streaming_agent_run_with_events",
		Input: models.StreamingAgentRunWithEventsInput{
			RequestJSON: requestJSON,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := newStringWriter()
	if err := h.streamJSONL(t.Context(), w, payload); err != nil {
		t.Fatalf("streamJSONL() failed: %v", err)
	}

	var got agentSpaceStreamResponse
	if err := json.Unmarshal([]byte(w.sb.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}
	if got.SessionID == "" || got.SessionID == externalSessionID {
		t.Fatalf("SessionID = %q, want generated backend session ID", got.SessionID)
	}
	if len(got.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(got.Events))
	}

	wantContent := genai.NewContentFromText("Please", genai.RoleUser)
	if diff := cmp.Diff(wantContent, got.Events[0].Content); diff != "" {
		t.Errorf("event content mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamJSONL_AgentSpaceAcceptsReturnedBackendSessionID(t *testing.T) {
	const (
		appName           = "app"
		userID            = "test-user@example.com"
		externalSessionID = "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890"
	)

	a, err := llmagent.New(llmagent.Config{
		Name: "Echo",
		BeforeAgentCallbacks: []agent.BeforeAgentCallback{
			func(cc agent.Context) (*genai.Content, error) {
				return cc.UserContent(), nil
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamingAgentRunWithEventsHandler(config, appName, "streaming_agent_run_with_events", "async_stream")

	run := func(message, sessionID string) agentSpaceStreamResponse {
		t.Helper()
		requestJSON := `{"message":{"role":"user","parts":[{"text":"` + message + `"}]},"session_id":"` + sessionID + `","user_id":"` + userID + `"}`
		payload, err := json.Marshal(models.StreamingAgentRunWithEventsRequest{
			ClassMethod: "streaming_agent_run_with_events",
			Input: models.StreamingAgentRunWithEventsInput{
				RequestJSON: requestJSON,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal() failed: %v", err)
		}

		w := newStringWriter()
		if err := h.streamJSONL(t.Context(), w, payload); err != nil {
			t.Fatalf("streamJSONL() failed: %v", err)
		}

		var got agentSpaceStreamResponse
		if err := json.Unmarshal([]byte(w.sb.String()), &got); err != nil {
			t.Fatalf("json.Unmarshal() failed: %v", err)
		}
		return got
	}

	first := run("Hi", externalSessionID)
	second := run("Again", first.SessionID)

	if first.SessionID == "" || first.SessionID == externalSessionID {
		t.Fatalf("first SessionID = %q, want generated backend session ID", first.SessionID)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("second SessionID = %q, want returned backend session ID %q", second.SessionID, first.SessionID)
	}

	list, err := config.SessionService.List(t.Context(), &session.ListRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(list.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(list.Sessions))
	}
}

func TestStreamJSONL_AgentSpaceUsesNonStreamingMode(t *testing.T) {
	const (
		appName           = "app"
		userID            = "test-user@example.com"
		externalSessionID = "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890"
	)

	a, err := llmagent.New(llmagent.Config{
		Name:  "StreamAware",
		Model: streamAwareLLM{},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamingAgentRunWithEventsHandler(config, appName, "streaming_agent_run_with_events", "async_stream")

	requestJSON := `{"message":{"role":"user","parts":[{"text":"What is your capabilities"}]},"session_id":"` + externalSessionID + `","user_id":"` + userID + `"}`
	payload, err := json.Marshal(models.StreamingAgentRunWithEventsRequest{
		ClassMethod: "streaming_agent_run_with_events",
		Input: models.StreamingAgentRunWithEventsInput{
			RequestJSON: requestJSON,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := newStringWriter()
	if err := h.streamJSONL(t.Context(), w, payload); err != nil {
		t.Fatalf("streamJSONL() failed: %v", err)
	}

	var got agentSpaceStreamResponse
	if err := json.Unmarshal([]byte(w.sb.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}
	if len(got.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(got.Events))
	}

	wantContent := genai.NewContentFromText("final response", genai.RoleModel)
	if diff := cmp.Diff(wantContent, got.Events[0].Content); diff != "" {
		t.Errorf("event content mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamingAgentRunWithEventsHandlerMetadata(t *testing.T) {
	handler := NewStreamingAgentRunWithEventsHandler(nil, "", "streaming_agent_run_with_events", "async_stream")

	got, err := handler.Metadata()
	if err != nil {
		t.Fatalf("Metadata() failed: %v", err)
	}

	want := map[string]any{
		"api_mode": "async_stream",
		"name":     "streaming_agent_run_with_events",
		"parameters": map[string]any{
			"properties": map[string]any{
				"request_json": map[string]any{
					"type": "string",
				},
			},
			"required": []any{"request_json"},
			"type":     "object",
		},
	}

	if diff := cmp.Diff(want, got.AsMap(), cmpopts.IgnoreMapEntries(func(k string, _ any) bool {
		return k == "description"
	})); diff != "" {
		t.Errorf("Metadata() mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamJSONL_EmitsPendingA2UIDataParts(t *testing.T) {
	const (
		appName           = "app"
		userID            = "test-user@example.com"
		externalSessionID = "projects/111111111111/locations/global/collections/default_collection/engines/test-engine/sessions/12345678901234567890"
	)

	a, err := llmagent.New(llmagent.Config{
		Name:  "StreamAware",
		Model: streamAwareLLM{},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamingAgentRunWithEventsHandler(config, appName, "streaming_agent_run_with_events", "async_stream")

	// Store pending A2UI messages
	testA2UIMsg := map[string]any{
		"version": "v0.9",
		"createSurface": map[string]any{
			"surfaceId": "test-surface-1",
			"catalogId": "https://a2ui.org/specification/v0_9/standard_catalog_definition.json",
		},
	}
	a2ui.StorePendingA2UIMessages("default", []any{testA2UIMsg})

	requestJSON := `{"message":{"role":"user","parts":[{"text":"Reconcile CTR-2026-451"}]},"session_id":"` + externalSessionID + `","user_id":"` + userID + `"}`
	payload, err := json.Marshal(models.StreamingAgentRunWithEventsRequest{
		ClassMethod: "streaming_agent_run_with_events",
		Input: models.StreamingAgentRunWithEventsInput{
			RequestJSON: requestJSON,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := newStringWriter()
	if err := h.streamJSONL(t.Context(), w, payload); err != nil {
		t.Fatalf("streamJSONL() failed: %v", err)
	}

	rawOutput := w.sb.String()
	if strings.Contains(rawOutput, `<a2a_datapart_json>`) {
		t.Fatalf("expected raw output NOT to contain '<a2a_datapart_json>', got:\n%s", rawOutput)
	}
	if !strings.Contains(rawOutput, `"kind":"data"`) {
		t.Fatalf("expected raw output to contain '\"kind\":\"data\"', got:\n%s", rawOutput)
	}
	if !strings.Contains(rawOutput, `"mimeType":"application/json+a2ui"`) {
		t.Fatalf("expected raw output to contain '\"mimeType\":\"application/json+a2ui\"', got:\n%s", rawOutput)
	}
	if !strings.Contains(rawOutput, `"data":`) {
		t.Fatalf("expected raw output to contain '\"data\":', got:\n%s", rawOutput)
	}
	if !strings.Contains(rawOutput, `"final response"`) {
		t.Fatalf("expected raw output to contain 'final response', got:\n%s", rawOutput)
	}
}

// TestGeminiEnterpriseApp_WireProtocol verifies that Discovery Engine receives valid JSONL SSE lines,
// where role: "user" is clean and role: "model" carries native A2A DataParts for UI rendering.
func TestGeminiEnterpriseApp_WireProtocol(t *testing.T) {
	const (
		appName           = "data-recon-agent"
		userID            = "analyst@globex.com"
		externalSessionID = "projects/14200540645/locations/global/collections/default_collection/engines/data-recon-agent/sessions/test-sess-1"
	)

	a, err := llmagent.New(llmagent.Config{
		Name:  "StreamAware",
		Model: streamAwareLLM{},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamingAgentRunWithEventsHandler(config, appName, "streaming_agent_run_with_events", "async_stream")

	// Stage full Basic Catalog A2UI Discrepancy Card
	card := a2ui.BuildBasicCatalogDiscrepancyCard(a2ui.DiscrepancyCardParams{
		ContractID:       "CTR-2026-451",
		AccountName:      "Globex Logistics Corporation",
		ServiceNowINC:    "INC0010042",
		BilledAmount:     115000.00,
		AgreedCap:        97000.00,
		VarianceAmount:   18000.00,
		Severity:         "CRITICAL",
		DiscrepancyCause: "Salesforce billed invoice ($115000.00) exceeds contract spend cap ($97000.00) by $18000.00.",
		Recommendation:   "Stage -$18000.00 billing adjustment credit in Salesforce Revenue Cloud.",
		ResolutionAction: "stage_salesforce_billing_adjustment",
	})
	cardMsgs := make([]any, 0, len(card))
	for _, m := range card {
		cardMsgs = append(cardMsgs, m)
	}
	a2ui.StorePendingA2UIMessages("default", cardMsgs)

	requestJSON := `{"message":{"role":"user","parts":[{"text":"Reconcile contract CTR-2026-451"}]},"session_id":"` + externalSessionID + `","user_id":"` + userID + `"}`
	payload, err := json.Marshal(models.StreamingAgentRunWithEventsRequest{
		ClassMethod: "streaming_agent_run_with_events",
		Input: models.StreamingAgentRunWithEventsInput{
			RequestJSON: requestJSON,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := newStringWriter()
	if err := h.streamJSONL(t.Context(), w, payload); err != nil {
		t.Fatalf("streamJSONL() failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(w.sb.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("Expected non-empty JSONL output from streamJSONL")
	}

	foundModelTurn := false
	foundBeginRenderingDataPart := false
	foundSurfaceUpdateDataPart := false
	foundSubmitPromptButton := false

	for lineIdx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			t.Fatalf("Line %d is not valid JSON: %v\nLine content:\n%s", lineIdx, err, line)
		}

		events, ok := chunk["events"].([]any)
		if !ok || len(events) == 0 {
			continue
		}

		for _, evAny := range events {
			ev, ok := evAny.(map[string]any)
			if !ok {
				continue
			}

			// Check content role
			content, _ := ev["content"].(map[string]any)
			if content == nil {
				if llmResp, ok := ev["llm_response"].(map[string]any); ok {
					content, _ = llmResp["content"].(map[string]any)
				}
			}

			if content != nil {
				role, _ := content["role"].(string)
				parts, _ := content["parts"].([]any)

				if role == "user" {
					// Verify role: "user" NEVER leaks raw <a2a_datapart_json> text
					for _, pAny := range parts {
						if pMap, ok := pAny.(map[string]any); ok {
							if text, ok := pMap["text"].(string); ok {
								if strings.Contains(text, "<a2a_datapart_json>") {
									t.Fatalf("role: 'user' leaked <a2a_datapart_json> text part:\n%s", text)
								}
							}
						}
					}
				}

				if role == "model" {
					foundModelTurn = true
					for _, pAny := range parts {
						if pMap, ok := pAny.(map[string]any); ok {
							if pMap["kind"] == "data" {
								metadata, _ := pMap["metadata"].(map[string]any)
								mimeType, _ := metadata["mimeType"].(string)
								if mimeType != a2ui.A2UIMimeType {
									t.Fatalf("Expected mimeType %q in data part metadata, got %q", a2ui.A2UIMimeType, mimeType)
								}

								var dataMap map[string]any
								if dm, isMap := pMap["data"].(map[string]any); isMap {
									dataMap = dm
								} else if b64Str, isStr := pMap["data"].(string); isStr {
									if bBytes, err := base64.StdEncoding.DecodeString(b64Str); err == nil {
										_ = json.Unmarshal(bBytes, &dataMap)
									}
								}

								if dataMap != nil {
									if _, hasBegin := dataMap["beginRendering"]; hasBegin {
										foundBeginRenderingDataPart = true
									}
									if su, hasSU := dataMap["surfaceUpdate"].(map[string]any); hasSU {
										foundSurfaceUpdateDataPart = true
										if comps, hasComps := su["components"].([]any); hasComps {
											for _, cAny := range comps {
												if cMap, isC := cAny.(map[string]any); isC {
													if compDef, hasC := cMap["component"].(map[string]any); hasC {
														if btn, hasBtn := compDef["Button"].(map[string]any); hasBtn {
															if action, hasAct := btn["action"].(map[string]any); hasAct {
																if action["name"] == "SubmitPrompt" {
																	foundSubmitPromptButton = true
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !foundModelTurn {
		t.Fatalf("Discovery Engine failed: No role: 'model' event found in JSONL stream")
	}
	if !foundBeginRenderingDataPart {
		t.Fatalf("Discovery Engine failed: beginRendering A2A DataPart missing from model event")
	}
	if !foundSurfaceUpdateDataPart {
		t.Fatalf("Discovery Engine failed: surfaceUpdate A2A DataPart missing from model event")
	}
	if !foundSubmitPromptButton {
		t.Fatalf("Discovery Engine failed: SubmitPrompt action button missing from surfaceUpdate component tree")
	}

	t.Logf("✓ Gemini Enterprise App wire protocol verified: Clean JSONL stream, role: 'model' carries native A2UI DataParts with SubmitPrompt buttons.")
}

// TestGeminiEnterpriseAgentPlatform_StreamQuery verifies the async_stream_query endpoint used by Agent Platform.
func TestGeminiEnterpriseAgentPlatform_StreamQuery(t *testing.T) {
	const (
		appName = "data-recon-agent"
		userID  = "tester@google.com"
	)

	a, err := llmagent.New(llmagent.Config{
		Name:  "StreamAware",
		Model: streamAwareLLM{},
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(a),
		SessionService: session.InMemoryService(),
	}
	h := NewStreamQueryHandler(config, appName, "async_stream_query", "async_stream")

	payload, err := json.Marshal(models.StreamQueryRequest{
		ClassMethod: "async_stream_query",
		Input: models.StreamQueryInput{
			Message: genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Reconcile contract CTR-2026-451"}},
			},
			UserID: userID,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := newStringWriter()
	if err := h.streamJSONL(t.Context(), w, payload); err != nil {
		t.Fatalf("streamJSONL() failed: %v", err)
	}

	rawOutput := w.sb.String()
	if !strings.Contains(rawOutput, "final response") {
		t.Fatalf("Expected stream query to output final response, got:\n%s", rawOutput)
	}

	t.Logf("✓ Gemini Enterprise Agent Platform async_stream_query verified.")
}
