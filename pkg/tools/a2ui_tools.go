package tools

import (
	"context"
	"encoding/json"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
)

// RenderDiscrepancyCardArgs defines the typed input schema for the LLM tool.
type RenderDiscrepancyCardArgs struct {
	ContractID       string  `json:"contract_id" doc:"The canonical contract identifier, e.g. CTR-2026-001"`
	AccountName      string  `json:"account_name" doc:"The legal name of the enterprise customer"`
	ServiceNowINC    string  `json:"servicenow_inc_id" doc:"The correlated ServiceNow dispute incident number, e.g. INC0010042"`
	BilledAmount     float64 `json:"billed_amount" doc:"The total billed invoice amount from Salesforce"`
	AgreedCap        float64 `json:"agreed_cap" doc:"The contractually agreed spend cap"`
	VarianceAmount   float64 `json:"variance_amount" doc:"The mathematical discrepancy amount"`
	Severity         string  `json:"severity" doc:"Classification: CRITICAL, HIGH, MEDIUM, LOW"`
	DiscrepancyCause string  `json:"discrepancy_cause" doc:"Synthesized root cause explanation"`
	Recommendation   string  `json:"recommendation" doc:"Actionable resolution advice"`
}

// RenderDiscrepancyCardResult is the returned A2UI payload.
type RenderDiscrepancyCardResult struct {
	A2UIPayload       string `json:"a2ui_json"`
	ValidatedA2UIJSON string `json:"validated_a2ui_json"`
	Status            string `json:"status"`
}

// RenderDiscrepancyCardHandler executes the parameterized card builder.
func RenderDiscrepancyCardHandler(ctx context.Context, args RenderDiscrepancyCardArgs) (*RenderDiscrepancyCardResult, error) {
	cardMessages := a2ui.BuildBasicCatalogDiscrepancyCard(a2ui.DiscrepancyCardParams{
		ContractID:       args.ContractID,
		AccountName:      args.AccountName,
		ServiceNowINC:    args.ServiceNowINC,
		BilledAmount:     args.BilledAmount,
		AgreedCap:        args.AgreedCap,
		VarianceAmount:   args.VarianceAmount,
		Severity:         args.Severity,
		DiscrepancyCause: args.DiscrepancyCause,
		Recommendation:   args.Recommendation,
		ResolutionAction: "stage_salesforce_billing_adjustment",
	})

	bytes, err := json.MarshalIndent(cardMessages, "", "  ")
	if err != nil {
		return nil, err
	}

	payloadStr := string(bytes)
	return &RenderDiscrepancyCardResult{
		A2UIPayload:       payloadStr,
		ValidatedA2UIJSON: payloadStr,
		Status:            "RENDERED",
	}, nil
}

// RenderHITLApprovalCardArgs defines the input schema for the HITL card tool.
type RenderHITLApprovalCardArgs struct {
	MutationID     string  `json:"mutation_id" doc:"Unique mutation identifier, e.g. MUT-SFDC-2026-001"`
	ContractID     string  `json:"contract_id" doc:"Target contract identifier, e.g. CTR-2026-451"`
	TargetSystem   string  `json:"target_system" doc:"Destination system, e.g. Salesforce Revenue Cloud"`
	AdjustmentType string  `json:"adjustment_type" doc:"Type of ledger mutation"`
	CreditAmount   float64 `json:"credit_amount" doc:"Approved financial credit adjustment amount"`
	SignatureHash  string  `json:"signature_hash" doc:"HMAC-SHA256 authorization signature"`
	ExpiresInSec   int     `json:"expires_in_seconds" doc:"TTL for signature expiration"`
}

// RenderHITLApprovalCardHandler executes the parameterized HITL approval card builder.
func RenderHITLApprovalCardHandler(ctx context.Context, args RenderHITLApprovalCardArgs) (*RenderDiscrepancyCardResult, error) {
	cardMessages := a2ui.BuildBasicCatalogHITLCard(a2ui.HITLApprovalCardParams{
		MutationID:     args.MutationID,
		ContractID:     args.ContractID,
		TargetSystem:   args.TargetSystem,
		AdjustmentType: args.AdjustmentType,
		CreditAmount:   args.CreditAmount,
		SignatureHash:  args.SignatureHash,
		ExpiresInSec:   args.ExpiresInSec,
	})

	bytes, err := json.MarshalIndent(cardMessages, "", "  ")
	if err != nil {
		return nil, err
	}

	payloadStr := string(bytes)
	return &RenderDiscrepancyCardResult{
		A2UIPayload:       payloadStr,
		ValidatedA2UIJSON: payloadStr,
		Status:            "RENDERED",
	}, nil
}
