package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/tools"
)

// ReconciliationOutcome contains the synthesized reasoning and generated A2UI payload.
type ReconciliationOutcome struct {
	ContractID       string            `json:"contract_id"`
	AccountName      string            `json:"account_name"`
	VarianceAmount   float64           `json:"variance_amount"`
	Severity         string            `json:"severity"`
	DiscrepancyCause string            `json:"discrepancy_cause"`
	Recommendation   string            `json:"recommendation"`
	A2UIEnvelope     a2ui.A2UIEnvelope `json:"a2ui_envelope"`
	ExecutedAt       time.Time         `json:"executed_at"`
}

// CoordinatorEngine orchestrates cross-system reconciliation.
type CoordinatorEngine struct {
	Model string
}

// NewCoordinatorEngine creates a new reconciliation coordinator.
func NewCoordinatorEngine(model string) *CoordinatorEngine {
	if model == "" {
		model = "gemini-3.7-flash"
	}
	return &CoordinatorEngine{Model: model}
}

// ReconcileRecord processes a single correlated reconciliation case.
func (c *CoordinatorEngine) ReconcileRecord(ctx context.Context, record schemas.CorrelatedReconciliationRecord) (*ReconciliationOutcome, error) {
	severity := "LOW"
	cause := "All billing schedule items match contract cap."
	recommendation := "Auto-reconcile without mutation."

	if record.VarianceAmount >= 5000.0 || record.VarianceArchetype == schemas.ArchetypeCriticalDiscrepancy {
		severity = "CRITICAL"
		cause = fmt.Sprintf("Salesforce billed invoice ($%.2f) exceeds contract spend cap ($%.2f) by $%.2f. ServiceNow incident indicates approved SLA dispute credit.",
			record.BilledAmount, record.AgreedCap, record.VarianceAmount)
		recommendation = fmt.Sprintf("Stage -$%.2f billing adjustment credit in Salesforce Revenue Cloud.", record.VarianceAmount)
	} else if record.VarianceAmount > 0.0 {
		severity = "MEDIUM"
		cause = fmt.Sprintf("Minor rounding variance of $%.2f between Salesforce invoice and ServiceNow record.", record.VarianceAmount)
		recommendation = "Apply standard enterprise tax/FX tolerance rule."
	}

	// Invoke parameterized A2UI tool
	cardArgs := tools.RenderDiscrepancyCardArgs{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		ServiceNowINC:    "INC0010042",
		BilledAmount:     record.BilledAmount,
		AgreedCap:        record.AgreedCap,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
	}

	res, err := tools.RenderDiscrepancyCardHandler(ctx, cardArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to render A2UI card: %w", err)
	}

	envelope := a2ui.BuildDiscrepancyEnvelope(a2ui.DiscrepancyCardParams{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		ServiceNowINC:    "INC0010042",
		BilledAmount:     record.BilledAmount,
		AgreedCap:        record.AgreedCap,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
		ResolutionAction: "stage_salesforce_billing_adjustment",
	})

	_ = res

	return &ReconciliationOutcome{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
		A2UIEnvelope:     envelope,
		ExecutedAt:       time.Now().UTC(),
	}, nil
}
