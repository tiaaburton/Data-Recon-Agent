package a2ui

import (
	"fmt"
)

// DiscrepancyCardParams contains the clean parameters passed by the LLM tool invocation.
type DiscrepancyCardParams struct {
	SurfaceID        string  `json:"surface_id"`
	ContractID       string  `json:"contract_id"`
	AccountName      string  `json:"account_name"`
	ServiceNowINC    string  `json:"servicenow_inc_id"`
	BilledAmount     float64 `json:"billed_amount"`
	AgreedCap        float64 `json:"agreed_cap"`
	VarianceAmount   float64 `json:"variance_amount"`
	Severity         string  `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM", "LOW"
	DiscrepancyCause string  `json:"discrepancy_cause"`
	Recommendation   string  `json:"recommendation"`
	ResolutionAction string  `json:"resolution_action"`
}

// HITLApprovalCardParams contains parameters for the cryptographically gated write card.
type HITLApprovalCardParams struct {
	SurfaceID      string  `json:"surface_id"`
	MutationID     string  `json:"mutation_id"`
	ContractID     string  `json:"contract_id"`
	TargetSystem   string  `json:"target_system"`   // e.g. "Salesforce Revenue Cloud"
	AdjustmentType string  `json:"adjustment_type"` // e.g. "Credit Memo / Billing Schedule Adjustment"
	CreditAmount   float64 `json:"credit_amount"`
	SignatureHash  string  `json:"signature_hash"`
	ExpiresInSec   int     `json:"expires_in_seconds"`
}

// BuildDiscrepancyEnvelope creates a 100% schema-valid A2UI v0.9 envelope for a billing discrepancy.
func BuildDiscrepancyEnvelope(params DiscrepancyCardParams) A2UIEnvelope {
	surfaceID := params.SurfaceID
	if surfaceID == "" {
		surfaceID = fmt.Sprintf("surface-%s", params.ContractID)
	}

	badgeType := "standard"
	badgeColor := "#1A73E8"
	if params.Severity == "CRITICAL" || params.VarianceAmount >= 5000.0 {
		badgeType = "explosive"
		badgeColor = "#EA4335"
	}

	rootComponent := ComponentDef{
		ID:        "root-container",
		Component: "CardContainer",
		Props: map[string]any{
			"elevation":    2,
			"padding":      "16px",
			"borderRadius": "12px",
			"border":       fmt.Sprintf("1px solid %s", badgeColor),
		},
		Children: []ComponentDef{
			// 1. Header with Explosive Alert Badge
			{
				ID:        "alert-header",
				Component: "DiscrepancyAlertBadge",
				Props: map[string]any{
					"badgeType":      badgeType,
					"severity":       params.Severity,
					"varianceAmount": fmt.Sprintf("$%.2f", params.VarianceAmount),
					"headline":       fmt.Sprintf("%s Billing Variance Detected", params.Severity),
					"pulseAnimation": params.Severity == "CRITICAL",
				},
			},
			// 2. Account & Contract Metadata Row
			{
				ID:        "meta-bar",
				Component: "MetadataRow",
				Props: map[string]any{
					"accountName":   params.AccountName,
					"contractId":    params.ContractID,
					"serviceNowInc": params.ServiceNowINC,
				},
			},
			// 3. Side-by-Side 2-Way Diff Table
			{
				ID:        "diff-table",
				Component: "MultiSystemDiffTable",
				Props: map[string]any{
					"headers": []string{"Field / Metric", "Salesforce CRM", "ServiceNow ITSM"},
					"rows": []map[string]any{
						{
							"fieldName":     "Billed Invoice Total",
							"salesforceVal": fmt.Sprintf("$%.2f", params.BilledAmount),
							"serviceNowVal": fmt.Sprintf("$%.2f", params.AgreedCap),
							"isMismatch":    params.BilledAmount != params.AgreedCap,
							"severity":      "high",
						},
						{
							"fieldName":     "Contract Agreed Cap",
							"salesforceVal": fmt.Sprintf("$%.2f", params.AgreedCap),
							"serviceNowVal": fmt.Sprintf("$%.2f", params.AgreedCap),
							"isMismatch":    false,
							"severity":      "none",
						},
						{
							"fieldName":     "Disputed Overage Delta",
							"salesforceVal": fmt.Sprintf("+$%.2f ⚠️", params.VarianceAmount),
							"serviceNowVal": fmt.Sprintf("-$%.2f Credit Approved", params.VarianceAmount),
							"isMismatch":    true,
							"severity":      "critical",
						},
					},
				},
			},
			// 4. Root-Cause Explanation Box
			{
				ID:        "explanation-box",
				Component: "InsightBox",
				Props: map[string]any{
					"title":          "Automated Root Cause Synthesis",
					"explanation":    params.DiscrepancyCause,
					"recommendation": params.Recommendation,
				},
			},
			// 5. Interactive Action Resolution Selector
			{
				ID:        "action-selector",
				Component: "FieldMatcherSelector",
				Props: map[string]any{
					"title": "Select Resolution Workflow",
					"options": []map[string]any{
						{
							"id":          "stage_adjustment",
							"label":       fmt.Sprintf("Stage -$%.2f Salesforce Billing Credit (Recommended)", params.VarianceAmount),
							"description": "Aligns Salesforce invoice with ServiceNow SLA dispute credit.",
							"isDefault":   true,
						},
						{
							"id":          "request_human_review",
							"label":       "Escalate to Finance Operations",
							"description": "Routes contract CTR-2026-001 to manual auditing queue.",
							"isDefault":   false,
						},
					},
				},
				Events: map[string]ActionEvent{
					"onSelect": {
						Action: "user_selected_resolution",
						Params: map[string]any{
							"contract_id": params.ContractID,
							"action":      params.ResolutionAction,
						},
					},
				},
			},
		},
	}

	return A2UIEnvelope{
		Version: ProtocolVersion,
		CreateSurface: &CreateSurfacePayload{
			SurfaceID: surfaceID,
			CatalogID: "enterprise-recon-catalog-v09",
			Theme:     "light",
		},
		UpdateComponents: &UpdateComponentsPayload{
			SurfaceID: surfaceID,
			Root:      rootComponent,
		},
		UpdateDataModel: &UpdateDataModelPayload{
			SurfaceID: surfaceID,
			Data: map[string]any{
				"contractId":     params.ContractID,
				"varianceAmount": params.VarianceAmount,
				"status":         "PENDING_HUMAN_SELECTION",
			},
		},
	}
}

// BuildHITLApprovalEnvelope creates the cryptographically signed write authorization card.
func BuildHITLApprovalEnvelope(params HITLApprovalCardParams) A2UIEnvelope {
	surfaceID := params.SurfaceID
	if surfaceID == "" {
		surfaceID = fmt.Sprintf("hitl-surface-%s", params.MutationID)
	}

	rootComponent := ComponentDef{
		ID:        "hitl-container",
		Component: "SignedMutationCard",
		Props: map[string]any{
			"mutationId":     params.MutationID,
			"contractId":     params.ContractID,
			"targetSystem":   params.TargetSystem,
			"adjustmentType": params.AdjustmentType,
			"creditAmount":   fmt.Sprintf("$%.2f", params.CreditAmount),
			"signatureHash":  params.SignatureHash,
			"expiresInSec":   params.ExpiresInSec,
			"badgeIcon":      "shield_verified",
		},
		Events: map[string]ActionEvent{
			"onApprove": {
				Action: "execute_signed_mutation",
				Params: map[string]any{
					"mutation_id": params.MutationID,
					"signature":   params.SignatureHash,
				},
			},
			"onReject": {
				Action: "reject_mutation",
				Params: map[string]any{
					"mutation_id": params.MutationID,
				},
			},
		},
	}

	return A2UIEnvelope{
		Version: ProtocolVersion,
		CreateSurface: &CreateSurfacePayload{
			SurfaceID: surfaceID,
			CatalogID: "enterprise-recon-catalog-v09",
			Theme:     "light",
		},
		UpdateComponents: &UpdateComponentsPayload{
			SurfaceID: surfaceID,
			Root:      rootComponent,
		},
		UpdateDataModel: &UpdateDataModelPayload{
			SurfaceID: surfaceID,
			Data: map[string]any{
				"mutationId": params.MutationID,
				"authorized": false,
			},
		},
	}
}
