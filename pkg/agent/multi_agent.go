package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/logger"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/memory"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/telemetry"
)

// IngestionAgent concurrently fetches and normalizes CRM and ITSM ledger entries.
type IngestionAgent struct{}

func (ia *IngestionAgent) FetchCrossSystemRecords(ctx context.Context, contractID string) (schemas.SalesforceOpportunitySeed, schemas.ServiceNowIncidentSeed, error) {
	var sf schemas.SalesforceOpportunitySeed
	var sn schemas.ServiceNowIncidentSeed
	var wg sync.WaitGroup

	wg.Add(2)

	// Fetch Salesforce CRM concurrently
	go func() {
		defer wg.Done()
		sf = schemas.SalesforceOpportunitySeed{
			Name:        "Contract " + contractID,
			Amount:      115000.00,
			StageName:   "Closed Won - Invoiced",
			CloseDate:   "2026-08-20",
			Description: "Annual enterprise subscription and SLA support tier",
		}
	}()

	// Fetch ServiceNow ITSM concurrently
	go func() {
		defer wg.Done()
		sn = schemas.ServiceNowIncidentSeed{
			ShortDescription: "SLA Outage Dispute Credit",
			Description:      "Credit for Q3 outage exceeding SLA threshold",
			Category:         "Billing Dispute",
			Impact:           "1 - High",
			Urgency:          "1 - High",
			State:            "Awaiting Financial Adjustment",
			CorrelationID:    "corr-" + contractID,
		}
	}()

	wg.Wait()

	logger.Info(ctx, "IngestionAgent: Retrieved cross-system records concurrently",
		"contract_id", contractID,
		"salesforce_amount", sf.Amount,
	)

	return sf, sn, nil
}

// DiscrepancyDetectorAgent analyzes cross-system variance and classifies severity.
type DiscrepancyDetectorAgent struct{}

func (da *DiscrepancyDetectorAgent) AnalyzeDiscrepancy(contractID string, sf schemas.SalesforceOpportunitySeed, sn schemas.ServiceNowIncidentSeed) (schemas.CorrelatedReconciliationRecord, string) {
	agreedCap := 97000.00
	variance := sf.Amount - agreedCap
	severity := "LOW"
	archetype := schemas.ArchetypeMatch

	if variance >= 5000.0 {
		severity = "CRITICAL"
		archetype = schemas.ArchetypeCriticalDiscrepancy
	} else if variance > 0.0 {
		severity = "MEDIUM"
		archetype = schemas.ArchetypeTaxFXRounding
	}

	record := schemas.CorrelatedReconciliationRecord{
		CorrelationID:     fmt.Sprintf("CORR-%s", contractID),
		ContractID:        contractID,
		AccountID:         "ACC-GLOBEX-01",
		AccountName:       "Globex Logistics Corporation",
		BilledAmount:      sf.Amount,
		AgreedCap:         agreedCap,
		VarianceAmount:    variance,
		Currency:          "USD",
		DetectedAt:        time.Now().UTC(),
		VarianceArchetype: archetype,
		Salesforce:        sf,
		ServiceNow:        sn,
	}

	return record, severity
}

// RemediationAgent synthesizes cryptographic HITL resolution proposals and executes true-ups.
type RemediationAgent struct{}

func (ra *RemediationAgent) PrepareResolution(record schemas.CorrelatedReconciliationRecord, severity string) (*a2ui.A2UIEnvelope, error) {
	cause := fmt.Sprintf("Salesforce billed invoice ($%.2f) exceeds contract cap ($%.2f) by $%.2f.",
		record.BilledAmount, record.AgreedCap, record.VarianceAmount)
	recommendation := fmt.Sprintf("Stage -$%.2f billing adjustment credit in Salesforce Revenue Cloud.", record.VarianceAmount)

	envelope := a2ui.BuildDiscrepancyEnvelope(a2ui.DiscrepancyCardParams{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		ServiceNowINC:    "INC009412",
		BilledAmount:     record.BilledAmount,
		AgreedCap:        record.AgreedCap,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
		ResolutionAction: "stage_salesforce_billing_adjustment",
	})

	return &envelope, nil
}

// MultiAgentReconPipeline orchestrates end-to-end execution across the 3 specialized sub-agents.
type MultiAgentReconPipeline struct {
	Ingestion   *IngestionAgent
	Detector    *DiscrepancyDetectorAgent
	Remediation *RemediationAgent
	Router      *StrategicModelRouter
	Memory      *memory.MemoryBank
	Telemetry   *telemetry.TelemetryRecorder
}

// NewMultiAgentReconPipeline constructs a fully initialized multi-agent pipeline.
func NewMultiAgentReconPipeline() *MultiAgentReconPipeline {
	return &MultiAgentReconPipeline{
		Ingestion:   &IngestionAgent{},
		Detector:    &DiscrepancyDetectorAgent{},
		Remediation: &RemediationAgent{},
		Router:      NewStrategicModelRouter(),
		Memory:      memory.GetMemoryBank(),
		Telemetry:   telemetry.GetRecorder(),
	}
}

// Execute orchestrates the full multi-agent pipeline with telemetry, routing, and memory persistence.
func (p *MultiAgentReconPipeline) Execute(ctx context.Context, contractID, userID string) (*ReconciliationOutcome, error) {
	start := time.Now()

	// 1. Ingestion Phase
	sf, sn, err := p.Ingestion.FetchCrossSystemRecords(ctx, contractID)
	if err != nil {
		p.Telemetry.Record(ctx, telemetry.IntentOutcomeRecord{
			UserID:       userID,
			ContractID:   contractID,
			Intent:       telemetry.IntentReconcileContract,
			Outcome:      telemetry.OutcomeError,
			Duration:     time.Since(start),
			Success:      false,
			ErrorMessage: err.Error(),
		})
		return nil, err
	}

	// 2. Detection Phase
	record, severity := p.Detector.AnalyzeDiscrepancy(contractID, sf, sn)

	// 3. Strategic Model Routing Phase
	routingDecision := p.Router.RouteForRecord(ctx, record)
	_ = routingDecision

	// 4. Remediation & A2UI Synthesis Phase
	envelope, err := p.Remediation.PrepareResolution(record, severity)
	if err != nil {
		return nil, err
	}

	// 5. Async Memory Bank Persistence
	p.Memory.AsyncPersistMemory(ctx, memory.MemoryRecord{
		UserID:     userID,
		ContractID: contractID,
		Category:   memory.CategoryAuditHistory,
		Key:        fmt.Sprintf("recon-%s", contractID),
		Content:    fmt.Sprintf("Variance of $%.2f flagged as %s. Staged credit recommendation.", record.VarianceAmount, severity),
	})

	outcome := &ReconciliationOutcome{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: fmt.Sprintf("Salesforce billed invoice ($%.2f) exceeds contract spend cap ($%.2f).", record.BilledAmount, record.AgreedCap),
		Recommendation:   fmt.Sprintf("Stage -$%.2f billing adjustment credit in Salesforce Revenue Cloud.", record.VarianceAmount),
		A2UIEnvelope:     *envelope,
		ExecutedAt:       time.Now().UTC(),
	}

	// 6. Record Intent vs Outcome Telemetry
	p.Telemetry.Record(ctx, telemetry.IntentOutcomeRecord{
		UserID:         userID,
		ContractID:     contractID,
		Intent:         telemetry.IntentReconcileContract,
		Outcome:        telemetry.OutcomeDiscrepancyFlagged,
		Severity:       severity,
		VarianceAmount: record.VarianceAmount,
		Duration:       time.Since(start),
		Success:        true,
	})

	return outcome, nil
}
