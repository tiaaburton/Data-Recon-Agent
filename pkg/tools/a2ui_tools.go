package tools

import (
	"context"
	"encoding/json"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
)

// RenderDiscrepancyCardArgs defines the typed input schema for the LLM tool.
type RenderDiscrepancyCardArgs struct {
	ContractID       string  `json:"contract_id" jsonschema:"description=The canonical contract identifier, e.g. CTR-2026-001"`
	AccountName      string  `json:"account_name" jsonschema:"description=The legal name of the enterprise customer"`
	ServiceNowINC    string  `json:"servicenow_inc_id" jsonschema:"description=The correlated ServiceNow dispute incident number, e.g. INC0010042"`
	BilledAmount     float64 `json:"billed_amount" jsonschema:"description=The total billed invoice amount from Salesforce"`
	AgreedCap        float64 `json:"agreed_cap" jsonschema:"description=The contractually agreed spend cap"`
	VarianceAmount   float64 `json:"variance_amount" jsonschema:"description=The mathematical discrepancy amount"`
	Severity         string  `json:"severity" jsonschema:"description=Classification: CRITICAL, HIGH, MEDIUM, LOW"`
	DiscrepancyCause string  `json:"discrepancy_cause" jsonschema:"description=Synthesized root cause explanation"`
	Recommendation   string  `json:"recommendation" jsonschema:"description=Actionable resolution advice"`
}

// RenderDiscrepancyCardResult is the returned A2UI payload.
type RenderDiscrepancyCardResult struct {
	A2UIPayload string `json:"a2ui_json"`
	Status      string `json:"status"`
}

// RenderDiscrepancyCardHandler executes the parameterized card builder.
func RenderDiscrepancyCardHandler(ctx context.Context, args RenderDiscrepancyCardArgs) (*RenderDiscrepancyCardResult, error) {
	envelope := a2ui.BuildDiscrepancyEnvelope(a2ui.DiscrepancyCardParams{
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

	bytes, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, err
	}

	return &RenderDiscrepancyCardResult{
		A2UIPayload: string(bytes),
		Status:      "RENDERED",
	}, nil
}
