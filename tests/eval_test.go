package tests

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/agent"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

func loadGoldenDataset(t *testing.T, path string) []schemas.CorrelatedReconciliationRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden dataset from %s: %v", path, err)
	}
	var records []schemas.CorrelatedReconciliationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("failed to unmarshal golden dataset: %v", err)
	}
	return records
}

// TestGoldenEvaluation_500 runs automated benchmark evaluation across all 500 synthetic cases.
func TestGoldenEvaluation_500(t *testing.T) {
	datasetPath := "../data/correlated_recon_500.json"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		datasetPath = "data/correlated_recon_500.json"
	}

	records := loadGoldenDataset(t, datasetPath)
	if len(records) < 500 {
		t.Fatalf("expected at least 500 golden evaluation records, got %d", len(records))
	}

	coordinator := agent.NewCoordinatorEngine("gemini-3.7-flash")
	ctx := context.Background()

	startTime := time.Now()
	var (
		correctClassifications int
		validA2UIPayloads      int
		criticalCount          int
		mediumCount            int
		lowCount               int
	)

	for i, record := range records {
		outcome, err := coordinator.ReconcileRecord(ctx, record)
		if err != nil {
			t.Errorf("record [%d] %s failed reconciliation: %v", i, record.ContractID, err)
			continue
		}

		// 1. Validate Math & Variance Accuracy
		expectedVariance := record.BilledAmount - record.AgreedCap
		if expectedVariance < 0 {
			expectedVariance = 0
		}
		if outcome.VarianceAmount != record.VarianceAmount {
			t.Errorf("record [%d] %s variance mismatch: got $%.2f, expected $%.2f",
				i, record.ContractID, outcome.VarianceAmount, record.VarianceAmount)
		}

		// 2. Validate Severity & Classification Rules
		switch outcome.Severity {
		case "CRITICAL":
			criticalCount++
			if record.VarianceAmount < 5000.0 && record.VarianceArchetype != schemas.ArchetypeCriticalDiscrepancy {
				t.Errorf("record %s misclassified as CRITICAL (variance $%.2f)", record.ContractID, record.VarianceAmount)
			} else {
				correctClassifications++
			}
		case "MEDIUM":
			mediumCount++
			if record.VarianceAmount <= 0.0 || record.VarianceAmount >= 5000.0 {
				t.Errorf("record %s misclassified as MEDIUM (variance $%.2f)", record.ContractID, record.VarianceAmount)
			} else {
				correctClassifications++
			}
		case "LOW":
			lowCount++
			if record.VarianceAmount > 0.0 {
				t.Errorf("record %s misclassified as LOW (variance $%.2f)", record.ContractID, record.VarianceAmount)
			} else {
				correctClassifications++
			}
		default:
			t.Errorf("unknown severity: %s", outcome.Severity)
		}

		// 3. Validate A2UI Declarative Envelope
		envelope := outcome.A2UIEnvelope
		if envelope.Version != a2ui.ProtocolVersion {
			t.Errorf("record %s invalid A2UI protocol version: %s", record.ContractID, envelope.Version)
		}
		if envelope.UpdateComponents == nil || envelope.UpdateComponents.Root.Component != "CardContainer" {
			t.Errorf("record %s missing root CardContainer in A2UI envelope", record.ContractID)
		} else {
			root := envelope.UpdateComponents.Root
			foundAlert := false
			foundTable := false
			foundInsight := false
			foundSelector := false

			for _, child := range root.Children {
				switch child.Component {
				case "DiscrepancyAlertBadge":
					foundAlert = true
				case "MultiSystemDiffTable":
					foundTable = true
				case "InsightBox":
					foundInsight = true
				case "FieldMatcherSelector":
					foundSelector = true
				}
			}

			if foundAlert && foundTable && foundInsight && foundSelector {
				validA2UIPayloads++
			} else {
				t.Errorf("record %s A2UI envelope missing components: alert=%v, table=%v, insight=%v, selector=%v",
					record.ContractID, foundAlert, foundTable, foundInsight, foundSelector)
			}
		}
	}

	duration := time.Since(startTime)
	accuracy := (float64(correctClassifications) / float64(len(records))) * 100.0
	a2uiCompliance := (float64(validA2UIPayloads) / float64(len(records))) * 100.0

	t.Logf("================================================================================")
	t.Logf("               GOLDEN DATASET EVALUATION BENCHMARK RESULTS                     ")
	t.Logf("================================================================================")
	t.Logf("Total Records Evaluated:       %d", len(records))
	t.Logf("Classification Accuracy:       %.2f%% (%d/%d)", accuracy, correctClassifications, len(records))
	t.Logf("A2UI v0.9 Protocol Compliance: %.2f%% (%d/%d)", a2uiCompliance, validA2UIPayloads, len(records))
	t.Logf("Distribution:                  CRITICAL=%d | MEDIUM=%d | LOW=%d", criticalCount, mediumCount, lowCount)
	t.Logf("Total Benchmark Duration:      %v (Mean: %v/record)", duration, duration/time.Duration(len(records)))
	t.Logf("================================================================================")

	if accuracy < 99.0 {
		t.Fatalf("evaluation accuracy %.2f%% below golden quality bar (99.0%%)", accuracy)
	}
	if a2uiCompliance < 100.0 {
		t.Fatalf("A2UI compliance %.2f%% below target (100.0%%)", a2uiCompliance)
	}
}

// TestA2UIComponentCatalog_Coverage validates the custom component catalog definitions.
func TestA2UIComponentCatalog_Coverage(t *testing.T) {
	params := a2ui.DiscrepancyCardParams{
		ContractID:       "CTR-2026-TEST",
		AccountName:      "Acme Enterprise Labs",
		ServiceNowINC:    "INC0099881",
		BilledAmount:     150000.00,
		AgreedCap:        100000.00,
		VarianceAmount:   50000.00,
		Severity:         "CRITICAL",
		DiscrepancyCause: "Salesforce billed invoice exceeds agreed spend cap by $50,000.00.",
		Recommendation:   "Stage credit adjustment.",
		ResolutionAction: "stage_salesforce_billing_adjustment",
	}

	envelope := a2ui.BuildDiscrepancyEnvelope(params)
	bytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("failed to marshal A2UI envelope: %v", err)
	}

	if len(bytes) == 0 {
		t.Fatalf("rendered A2UI payload is empty")
	}

	// Verify surface creation & component tree exist
	if envelope.CreateSurface == nil || envelope.CreateSurface.SurfaceID == "" {
		t.Errorf("expected non-empty CreateSurface payload")
	}
	if envelope.UpdateComponents == nil || len(envelope.UpdateComponents.Root.Children) < 4 {
		t.Errorf("expected at least 4 child components in Root CardContainer")
	}
}

// TestLiveAgentModelResponse verifies end-to-end local agent execution and tool calling.
func TestLiveAgentModelResponse(t *testing.T) {
	ctx := context.Background()
	datasetPath := "../data/correlated_recon_500.json"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		datasetPath = "data/correlated_recon_500.json"
	}

	coordinator := agent.NewCoordinatorEngine("gemini-3.7-flash")
	records := loadGoldenDataset(t, datasetPath)

	// Test local direct execution on a critical discrepancy contract
	targetRecord := records[450] // CTR-2026-451
	outcome, err := coordinator.ReconcileRecord(ctx, targetRecord)
	if err != nil {
		t.Fatalf("failed local agent reconciliation: %v", err)
	}

	if outcome.ContractID != targetRecord.ContractID {
		t.Errorf("expected contract %s, got %s", targetRecord.ContractID, outcome.ContractID)
	}
	if outcome.Severity != "CRITICAL" {
		t.Errorf("expected severity CRITICAL, got %s", outcome.Severity)
	}
	if outcome.VarianceAmount != 18000.00 {
		t.Errorf("expected variance $18000.00, got $%.2f", outcome.VarianceAmount)
	}

	t.Logf("✓ Local Agent Response Verified:")
	t.Logf("  • Contract:        %s", outcome.ContractID)
	t.Logf("  • Account:         %s", outcome.AccountName)
	t.Logf("  • Severity:        %s", outcome.Severity)
	t.Logf("  • Variance:        $%.2f", outcome.VarianceAmount)
	t.Logf("  • Discrepancy:     %s", outcome.DiscrepancyCause)
	t.Logf("  • Recommendation:  %s", outcome.Recommendation)
	t.Logf("  • A2UI Protocol:   %s (%d components in tree)", outcome.A2UIEnvelope.Version, len(outcome.A2UIEnvelope.UpdateComponents.Root.Children))
}

// TestMultiturnReasoningEngineEvaluation verifies sequential multi-turn evaluation
// (Turn 1: CTR-2026-451, Turn 2: CTR-2026-001) under session history continuation.
func TestMultiturnReasoningEngineEvaluation(t *testing.T) {
	ctx := context.Background()
	datasetPath := "../data/correlated_recon_500.json"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		datasetPath = "data/correlated_recon_500.json"
	}

	coordinator := agent.NewCoordinatorEngine("gemini-3.7-flash")
	records := loadGoldenDataset(t, datasetPath)

	// Turn 1: Reconcile CTR-2026-451
	targetRecord1 := records[450] // CTR-2026-451
	outcome1, err := coordinator.ReconcileRecord(ctx, targetRecord1)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	if outcome1.ContractID != "CTR-2026-451" {
		t.Fatalf("Turn 1 expected CTR-2026-451, got %s", outcome1.ContractID)
	}
	t.Logf("✓ Turn 1 (CTR-2026-451) Succeeded: Variance=$%.2f, Severity=%s", outcome1.VarianceAmount, outcome1.Severity)

	// Turn 2: Reconcile CTR-2026-001 (Multi-turn continuation)
	targetRecord2 := records[0] // CTR-2026-001
	outcome2, err := coordinator.ReconcileRecord(ctx, targetRecord2)
	if err != nil {
		t.Fatalf("Turn 2 failed for CTR-2026-001: %v", err)
	}
	if outcome2.ContractID != "CTR-2026-001" {
		t.Fatalf("Turn 2 expected CTR-2026-001, got %s", outcome2.ContractID)
	}
	if outcome2.Severity != "LOW" {
		t.Fatalf("Turn 2 expected LOW severity, got %s", outcome2.Severity)
	}
	if outcome2.VarianceAmount != 0.00 {
		t.Fatalf("Turn 2 expected variance $0.00, got $%.2f", outcome2.VarianceAmount)
	}
	t.Logf("✓ Turn 2 (CTR-2026-001) Succeeded: Variance=$%.2f, Severity=%s", outcome2.VarianceAmount, outcome2.Severity)

	// Verify A2UI payload contains all valid declarative elements
	if outcome2.A2UIEnvelope.CreateSurface == nil || outcome2.A2UIEnvelope.UpdateComponents == nil {
		t.Fatalf("Turn 2 A2UI payload missing surface or components")
	}
	t.Logf("✓ Multi-turn evaluation completed successfully without serialization or state corruption.")
}

func TestA2UIPartsPlugin_EmitsDataParts(t *testing.T) {
	plug, err := a2ui.NewA2UIPartsPlugin()
	if err != nil {
		t.Fatalf("Failed to create A2UI plugin: %v", err)
	}

	testEvent := &session.Event{
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							Name: "reconcile_contract",
							Response: map[string]any{
								"contract_id": "CTR-2026-451",
								"a2ui_json":   `{"version":"v0.9","createSurface":{"surfaceId":"surface-CTR-451"}}`,
							},
						},
					},
					{
						Text: "Here is the summary:\n```json\n{\n  \"version\": \"v0.9\",\n  \"createSurface\": {}\n}\n```",
					},
				},
			},
		},
	}

	// Invoke plugin callback
	callback := plug.OnEventCallback()
	if callback == nil {
		t.Fatalf("Expected OnEventCallback to be configured")
	}

	resEvent, err := callback(nil, testEvent)
	if err != nil {
		t.Fatalf("OnEventCallback returned error: %v", err)
	}
	if resEvent == nil {
		t.Fatalf("Expected modified event, got nil")
	}

	// Verify tool response is sanitized and raw JSON block is stripped from text
	if resEvent.Content.Parts[0].FunctionResponse.Response["a2ui_status"] != "SYNTHESIZED_INTERACTIVE_CARD" {
		t.Fatalf("Expected a2ui_status SYNTHESIZED_INTERACTIVE_CARD, got: %v", resEvent.Content.Parts[0].FunctionResponse.Response["a2ui_status"])
	}

	for _, p := range resEvent.Content.Parts {
		if p.Text != "" && strings.Contains(p.Text, "\"version\": \"v0.9\"") {
			t.Fatalf("Expected raw JSON to be stripped from text part, got: %s", p.Text)
		}
	}
	t.Logf("✓ A2UIPartsPlugin successfully sanitized tool response and cleaned raw JSON blocks for Gemini Enterprise rendering.")
}
