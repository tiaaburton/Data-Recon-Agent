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

// Standard Catalog IDs for A2UI protocol.
const (
	BasicCatalogID   = "https://a2ui.org/specification/v0_8/standard_catalog_definition.json"
	BasicCatalogIDV9 = "https://a2ui.org/specification/v0_9/standard_catalog_definition.json"

	A2UIVersion          = "v0.8"
	ValidatedA2UIJSONKey = "validated_a2ui_json"
)

// OptionItem represents a labeled choice option in A2UI MultipleChoice.
type OptionItem struct {
	Label map[string]string `json:"label"`
	Value string            `json:"value"`
}

// ResolutionActionOptions represents standard reconciliation workflow choices.
var ResolutionActionOptions = []OptionItem{
	{Label: map[string]string{"literalString": "Stage Salesforce Billing Adjustment Credit"}, Value: "stage_salesforce_billing_adjustment"},
	{Label: map[string]string{"literalString": "Escalate to Finance Operations Audit Queue"}, Value: "escalate_finance_ops"},
	{Label: map[string]string{"literalString": "Dismiss Variance within Acceptable Tolerance"}, Value: "dismiss_variance"},
}

// ReconA2UIGuidance provides LLM instructions for A2UI discrepancy card rendering and tool usage.
const ReconA2UIGuidance = `## Autonomous Data Reconciliation A2UI Guidance & Instructions
CRITICAL RULES FOR MULTI-SYSTEM RECONCILIATION A2UI RENDERING:
1. UNIFIED POINT OF ENTRY: ALWAYS invoke 'reconcile_contract' or 'render_discrepancy_card' as your primary tool for contract variance inspection. NEVER manually construct raw A2UI JSON payloads or output raw JSON strings in chat.
2. AUTOMATIC DETERMINISTIC RENDERING: All A2UI discrepancy cards and HITL write authorization cards are automatically synthesized with the standard Basic Catalog. The tool embeds the visual UI card directly in its response.
3. CONCISE INTRODUCTORY TEXT: Accompany the interactive card with a brief, clear executive summary stating Account Name, Contract ID, financial variance amount, severity, and root cause.
4. ACTION EXECUTION: When the operator selects an action (e.g. "Stage Credit Memo", "Escalate", "Dismiss"), call 'apply_resolution_action' to commit the ledger update.
5. NEVER OUTPUT RAW A2UI JSON: You MUST NEVER output, print, or repeat raw JSON code blocks or tool response dictionaries in your text response. Gemini Enterprise automatically renders the visual card via the native A2A DataPart streaming channel.
`
