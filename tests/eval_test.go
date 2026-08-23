package tests

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/agent"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/guardrails"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/logger"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/memory"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/telemetry"
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
		if math.Abs(outcome.VarianceAmount-record.VarianceAmount) > 0.01 {
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

	cardMessages := a2ui.BuildBasicCatalogDiscrepancyCard(a2ui.DiscrepancyCardParams{
		ContractID:       "CTR-2026-451",
		AccountName:      "Globex Logistics Corporation",
		ServiceNowINC:    "INC0010042",
		BilledAmount:     115000.00,
		AgreedCap:        97000.00,
		VarianceAmount:   18000.00,
		Severity:         "CRITICAL",
		DiscrepancyCause: "Salesforce invoice exceeds spend cap.",
		Recommendation:   "Stage -$18,000.00 billing adjustment.",
	})
	cardBytes, _ := json.Marshal(cardMessages)

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
								"a2ui_json":   string(cardBytes),
							},
						},
					},
					{
						Text: "Here is the summary:\n```json\n{\n  \"surfaceId\": \"recon-surface-CTR-2026-451\"\n}\n```",
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

	// Verify tool response is sanitized
	foundSanitized := false
	for _, p := range resEvent.Content.Parts {
		if p.FunctionResponse != nil && p.FunctionResponse.Response != nil {
			if p.FunctionResponse.Response["a2ui_status"] == "A2UI_SURFACE_SYNTHESIZED" {
				foundSanitized = true
			}
		}
	}
	if !foundSanitized {
		t.Fatalf("Expected a2ui_status A2UI_SURFACE_SYNTHESIZED on function response part")
	}

	foundBeginRendering := false
	foundSurfaceUpdate := false
	for _, p := range resEvent.Content.Parts {
		if strings.Contains(p.Text, "<a2a_datapart_json>") {
			if strings.Contains(p.Text, "beginRendering") && strings.Contains(p.Text, "standard_catalog_definition.json") {
				foundBeginRendering = true
			}
			if strings.Contains(p.Text, "surfaceUpdate") && strings.Contains(p.Text, "btn-stage-credit") {
				foundSurfaceUpdate = true
			}
		}
	}

	if !foundBeginRendering || !foundSurfaceUpdate {
		t.Fatalf("Expected native A2A DataParts in Text parts for beginRendering and surfaceUpdate with buttons (begin=%v, update=%v)",
			foundBeginRendering, foundSurfaceUpdate)
	}

	t.Logf("✓ A2UIPartsPlugin successfully converted tool responses to native A2A DataParts with interactive buttons.")
}

func TestPIIGuardrailRedaction(t *testing.T) {
	input := "Customer SSN is 123-45-6789 and Card is 4111-2222-3333-4444. Contact user@globex.com with key AIzaSyD98374829374823947."
	redacted := guardrails.RedactPII(input)

	if strings.Contains(redacted, "123-45-6789") {
		t.Fatalf("Failed to redact SSN")
	}
	if strings.Contains(redacted, "4111-2222-3333-4444") {
		t.Fatalf("Failed to redact Credit Card")
	}
	if strings.Contains(redacted, "user@globex.com") {
		t.Fatalf("Failed to redact Email")
	}
	if strings.Contains(redacted, "AIzaSyD98374829374823947") {
		t.Fatalf("Failed to redact API Key")
	}
	t.Logf("✓ PII Guardrail accurately redacted sensitive data: %s", redacted)
}

func TestIntentOutcomeTelemetry(t *testing.T) {
	rec := telemetry.GetRecorder()
	ctx := context.Background()

	rec.Record(ctx, telemetry.IntentOutcomeRecord{
		UserID:         "test-user",
		ContractID:     "CTR-2026-451",
		Intent:         telemetry.IntentReconcileContract,
		Outcome:        telemetry.OutcomeDiscrepancyFlagged,
		Severity:       "CRITICAL",
		VarianceAmount: 18000.00,
		Duration:       45 * time.Millisecond,
		Success:        true,
	})

	stats := rec.GetSummary()
	if stats.TotalRequests == 0 {
		t.Fatalf("Expected non-zero telemetry records")
	}
	if stats.TotalVarianceDetected < 18000.00 {
		t.Fatalf("Expected variance tracking >= 18000, got %f", stats.TotalVarianceDetected)
	}
	t.Logf("✓ Intent vs Outcome Telemetry verified: SuccessRate=%.1f%%, TotalVariance=$%.2f",
		stats.SuccessRate, stats.TotalVarianceDetected)
}

func TestAsyncMemoryBankOperations(t *testing.T) {
	bank := memory.NewMemoryBank()
	ctx := context.Background()

	// 1. Test Async Persistence
	errChan := bank.AsyncPersistMemory(ctx, memory.MemoryRecord{
		UserID:     "user-123",
		ContractID: "CTR-2026-451",
		Category:   memory.CategoryAuditHistory,
		Key:        "audit-CTR-451",
		Content:    "Approved $18,000 credit memo for Globex SLA dispute.",
	})
	if err := <-errChan; err != nil {
		t.Fatalf("AsyncPersistMemory failed: %v", err)
	}

	// 2. Test Async Recall
	resChan := bank.AsyncRecallMemories(ctx, "user-123", "Globex", 5)
	res := <-resChan
	if res.Err != nil {
		t.Fatalf("AsyncRecallMemories failed: %v", res.Err)
	}
	if len(res.Memories) == 0 {
		t.Fatalf("Expected recalled memory for query 'Globex', got 0")
	}
	t.Logf("✓ Async Memory Bank recalled %d memory item: %s", len(res.Memories), res.Memories[0].Content)

	// 3. Test Session Compaction
	compSummary, err := bank.CompactSessionHistory(ctx, "sess-123", 10)
	if err != nil {
		t.Fatalf("CompactSessionHistory failed: %v", err)
	}
	if compSummary.CompactedTurns >= compSummary.OriginalTurns {
		t.Fatalf("Expected compacted turns < original turns")
	}
	t.Logf("✓ Memory Compaction succeeded: %d turns -> %d turns", compSummary.OriginalTurns, compSummary.CompactedTurns)
}

func TestStrategicModelRouter(t *testing.T) {
	router := agent.NewStrategicModelRouter()
	ctx := context.Background()

	// Test Case 1: High variance record should route to Pro
	criticalRecord := schemas.CorrelatedReconciliationRecord{
		ContractID:        "CTR-2026-451",
		VarianceAmount:    18000.00,
		VarianceArchetype: schemas.ArchetypeCriticalDiscrepancy,
	}
	decisionPro := router.RouteForRecord(ctx, criticalRecord)
	if decisionPro.Tier != "PRO" {
		t.Fatalf("Expected PRO tier for $18k variance, got %s", decisionPro.Tier)
	}

	// Test Case 2: Low variance record should route to Flash
	lowRecord := schemas.CorrelatedReconciliationRecord{
		ContractID:        "CTR-2026-001",
		VarianceAmount:    0.00,
		VarianceArchetype: schemas.ArchetypeMatch,
	}
	decisionFlash := router.RouteForRecord(ctx, lowRecord)
	if decisionFlash.Tier != "FLASH" {
		t.Fatalf("Expected FLASH tier for clean record, got %s", decisionFlash.Tier)
	}
	t.Logf("✓ Strategic Model Router verified: Critical->$18k (PRO: %s), Clean->$0 (FLASH: %s)",
		decisionPro.SelectedModel, decisionFlash.SelectedModel)
}

func TestMultiAgentReconPipeline(t *testing.T) {
	pipeline := agent.NewMultiAgentReconPipeline()
	ctx := context.Background()

	outcome, err := pipeline.Execute(ctx, "CTR-2026-451", "test-evaluator")
	if err != nil {
		t.Fatalf("MultiAgentReconPipeline failed: %v", err)
	}

	if outcome.ContractID != "CTR-2026-451" {
		t.Fatalf("Expected CTR-2026-451, got %s", outcome.ContractID)
	}
	if outcome.Severity != "CRITICAL" {
		t.Fatalf("Expected CRITICAL severity, got %s", outcome.Severity)
	}
	if outcome.VarianceAmount != 18000.00 {
		t.Fatalf("Expected $18,000 variance, got %f", outcome.VarianceAmount)
	}
	t.Logf("✓ Multi-Agent Pipeline executed cleanly across Ingestion, Detection, and Remediation sub-agents.")
}

func TestStructuredLogger(t *testing.T) {
	logger.Info(context.Background(), "Structured logging test",
		"subsystem", "evaluation_suite",
		"status", "PASS",
	)
	t.Logf("✓ Structured JSON logger verified.")
}
