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

package a2ui

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestCleanRawJSONText_ProtectsA2ADataPart(t *testing.T) {
	msg := map[string]any{
		"beginRendering": map[string]any{
			"surfaceId": "test-surface",
			"root":      "root",
			"catalogId": BasicCatalogID,
		},
	}
	dataPartText := WrapA2UIDataPartText(msg)
	input := "Here is the discrepancy summary:\n" + dataPartText + "\nPlease review the details."

	cleaned := CleanRawJSONText(input)

	if !strings.Contains(cleaned, "<a2a_datapart_json>") {
		t.Fatalf("CleanRawJSONText stripped valid <a2a_datapart_json> block")
	}
	if !strings.Contains(cleaned, "Here is the discrepancy summary:") {
		t.Fatalf("CleanRawJSONText lost conversational text")
	}
	if !strings.Contains(cleaned, "Please review the details.") {
		t.Fatalf("CleanRawJSONText lost trailing conversational text")
	}
}

func TestCleanRawJSONText_StripsRawToolDumps(t *testing.T) {
	rawDump := `{"status": "success", "validated_a2ui_json": "[{\"beginRendering\":{}}]"}`
	cleaned := CleanRawJSONText(rawDump)
	if cleaned != "" {
		t.Fatalf("Expected raw tool dictionary dump to be stripped, got %q", cleaned)
	}

	codeBlock := "Here is the card:\n```json\n{\n  \"status\": \"success\",\n  \"validated_a2ui_json\": \"...\"\n}\n```\nLet me know what you think."
	cleaned2 := CleanRawJSONText(codeBlock)
	if strings.Contains(cleaned2, "validated_a2ui_json") {
		t.Fatalf("Expected raw JSON codeblock to be stripped, got %q", cleaned2)
	}
	if !strings.Contains(cleaned2, "Here is the card:") {
		t.Fatalf("Expected conversational prefix to remain, got %q", cleaned2)
	}
}

func TestUniquifySurfaceIDs(t *testing.T) {
	messages := []any{
		map[string]any{
			"beginRendering": map[string]any{
				"surfaceId": "recon-surface-CTR-2026-001",
				"root":      "root",
				"catalogId": BasicCatalogID,
			},
		},
		map[string]any{
			"surfaceUpdate": map[string]any{
				"surfaceId": "recon-surface-CTR-2026-001",
				"components": []map[string]any{
					{"id": "root"},
				},
			},
		},
	}

	uniquified := UniquifySurfaceIDs(messages)
	if len(uniquified) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(uniquified))
	}

	m0 := uniquified[0].(map[string]any)["beginRendering"].(map[string]any)
	m1 := uniquified[1].(map[string]any)["surfaceUpdate"].(map[string]any)

	sID0 := m0["surfaceId"].(string)
	sID1 := m1["surfaceId"].(string)

	if sID0 == "recon-surface-CTR-2026-001" {
		t.Fatalf("Expected surfaceId to be suffixed with random hex, got %s", sID0)
	}
	if sID0 != sID1 {
		t.Fatalf("Expected consistent surfaceId across beginRendering and surfaceUpdate, got %s and %s", sID0, sID1)
	}
}

func TestBuildBasicCatalogDiscrepancyCard_Structure(t *testing.T) {
	params := DiscrepancyCardParams{
		ContractID:       "CTR-2026-451",
		AccountName:      "Globex Logistics",
		ServiceNowINC:    "INC0010042",
		BilledAmount:     115000.00,
		AgreedCap:        97000.00,
		VarianceAmount:   18000.00,
		Severity:         "CRITICAL",
		DiscrepancyCause: "Salesforce invoice exceeds spend cap.",
		Recommendation:   "Stage -$18,000.00 credit.",
	}

	card := BuildBasicCatalogDiscrepancyCard(params)
	if len(card) != 2 {
		t.Fatalf("Expected 2 A2UI messages (beginRendering, surfaceUpdate), got %d", len(card))
	}

	beginRendering, ok := card[0]["beginRendering"].(map[string]any)
	if !ok || beginRendering["catalogId"] != BasicCatalogID {
		t.Fatalf("Invalid beginRendering payload: %+v", card[0])
	}

	surfaceUpdate, ok := card[1]["surfaceUpdate"].(map[string]any)
	if !ok {
		t.Fatalf("Invalid surfaceUpdate payload: %+v", card[1])
	}
	components, ok := surfaceUpdate["components"].([]map[string]any)
	if !ok || len(components) < 10 {
		t.Fatalf("Expected at least 10 basic catalog components, got %d", len(components))
	}

	// Verify SubmitPrompt buttons are correctly structured
	foundStageBtn := false
	for _, c := range components {
		if c["id"] == "btn-stage-credit" {
			compMap := c["component"].(map[string]any)
			btnMap := compMap["Button"].(map[string]any)
			actionMap := btnMap["action"].(map[string]any)
			if actionMap["name"] == "SubmitPrompt" {
				foundStageBtn = true
			}
		}
	}
	if !foundStageBtn {
		t.Fatalf("Expected btn-stage-credit with SubmitPrompt action")
	}
}

func TestBuildBasicCatalogHITLCard_Structure(t *testing.T) {
	params := HITLApprovalCardParams{
		MutationID:     "MUT-SFDC-2026-001",
		ContractID:     "CTR-2026-451",
		TargetSystem:   "Salesforce Revenue Cloud",
		AdjustmentType: "Credit Memo",
		CreditAmount:   18000.00,
		SignatureHash:  "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		ExpiresInSec:   300,
	}

	card := BuildBasicCatalogHITLCard(params)
	if len(card) != 2 {
		t.Fatalf("Expected 2 A2UI messages for HITL card, got %d", len(card))
	}

	begin := card[0]["beginRendering"].(map[string]any)
	if begin["surfaceId"] != "hitl-surface-MUT-SFDC-2026-001" {
		t.Fatalf("Unexpected surfaceId: %v", begin["surfaceId"])
	}
}

func TestA2UIPartsPlugin_EndToEndInterception(t *testing.T) {
	plug, err := NewA2UIPartsPlugin()
	if err != nil {
		t.Fatalf("Failed to create A2UI plugin: %v", err)
	}

	card := BuildBasicCatalogDiscrepancyCard(DiscrepancyCardParams{
		ContractID:     "CTR-2026-001",
		AccountName:    "Apex Global",
		VarianceAmount: 0.0,
		Severity:       "LOW",
	})
	cardBytes, _ := json.Marshal(card)

	ev := &session.Event{
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							Name: "reconcile_contract",
							Response: map[string]any{
								"contract_id":         "CTR-2026-001",
								"validated_a2ui_json": string(cardBytes),
							},
						},
					},
					{
						Text: "Reconciliation analysis complete:\n```json\n{\"validated_a2ui_json\":\"...\"}\n```\nAll records matched spend cap.",
					},
				},
			},
		},
	}

	callback := plug.OnEventCallback()
	res, err := callback(nil, ev)
	if err != nil {
		t.Fatalf("OnEventCallback failed: %v", err)
	}
	if res == nil {
		t.Fatalf("Expected modified event, got nil")
	}

	// 1. Check FunctionResponse sanitized
	for _, p := range res.Content.Parts {
		if p.FunctionResponse != nil {
			if _, hasRaw := p.FunctionResponse.Response["validated_a2ui_json"]; hasRaw {
				t.Fatalf("validated_a2ui_json should be sanitized from function response")
			}
			if p.FunctionResponse.Response["a2ui_status"] != "A2UI_SURFACE_SYNTHESIZED" {
				t.Fatalf("Expected a2ui_status A2UI_SURFACE_SYNTHESIZED")
			}
		}
	}

	// 2. Check DataPart injected into parts
	foundDataPart := false
	for _, p := range res.Content.Parts {
		if strings.Contains(p.Text, "<a2a_datapart_json>") && strings.Contains(p.Text, A2UIMimeType) {
			foundDataPart = true
			break
		}
	}
	if !foundDataPart {
		t.Fatalf("Expected native <a2a_datapart_json> Part in event.Content.Parts")
	}

	// 3. Check text part cleaned of raw JSON code blocks
	foundCleanText := false
	for _, p := range res.Content.Parts {
		if strings.Contains(p.Text, "All records matched spend cap.") {
			if strings.Contains(p.Text, "```json") {
				t.Fatalf("Text still contains raw JSON code block: %s", p.Text)
			}
			foundCleanText = true
		}
	}
	if !foundCleanText {
		t.Fatalf("Expected cleaned conversational text in event.Content.Parts")
	}
}
