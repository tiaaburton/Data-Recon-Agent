package schemas

import (
	"time"
)

// VarianceArchetype defines the mathematical categorization of a discrepancy.
type VarianceArchetype string

const (
	ArchetypeMatch               VarianceArchetype = "PERFECT_MATCH"
	ArchetypeTimingLag           VarianceArchetype = "TIMING_LAG"
	ArchetypeTaxFXRounding       VarianceArchetype = "TAX_FX_ROUNDING"
	ArchetypeCriticalDiscrepancy VarianceArchetype = "CRITICAL_DISCREPANCY"
)

// SalesforceOpportunitySeed represents an Opportunity record to be seeded into Salesforce.
type SalesforceOpportunitySeed struct {
	Name        string  `json:"Name"`
	Amount      float64 `json:"Amount"`
	StageName   string  `json:"StageName"`
	CloseDate   string  `json:"CloseDate"`
	Description string  `json:"Description"`
}

// ServiceNowIncidentSeed represents an Incident record to be seeded into ServiceNow.
type ServiceNowIncidentSeed struct {
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Impact           string `json:"impact"`
	Urgency          string `json:"urgency"`
	State            string `json:"state"`
	CorrelationID    string `json:"correlation_id"`
	CloseCode        string `json:"close_code,omitempty"`
	CloseNotes       string `json:"close_notes,omitempty"`
}

// CorrelatedReconciliationRecord binds Salesforce and ServiceNow records with shared keys.
type CorrelatedReconciliationRecord struct {
	CorrelationID     string                    `json:"correlation_id"`
	ContractID        string                    `json:"contract_id"`
	AccountID         string                    `json:"account_id"`
	AccountName       string                    `json:"account_name"`
	VarianceArchetype VarianceArchetype         `json:"variance_archetype"`
	BilledAmount      float64                   `json:"billed_amount"`
	AgreedCap         float64                   `json:"agreed_cap"`
	VarianceAmount    float64                   `json:"variance_amount"`
	Currency          string                    `json:"currency"`
	DetectedAt        time.Time                 `json:"detected_at"`
	Salesforce        SalesforceOpportunitySeed `json:"salesforce_opportunity"`
	ServiceNow        ServiceNowIncidentSeed    `json:"servicenow_incident"`
}
