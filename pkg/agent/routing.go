package agent

import (
	"context"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/logger"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

const (
	ModelFlash = "publishers/google/models/gemini-3.7-flash"
	ModelPro   = "publishers/google/models/gemini-2.5-pro"
)

// RoutingDecision captures the model selection and reasoning rationale.
type RoutingDecision struct {
	SelectedModel string  `json:"selected_model"`
	Tier          string  `json:"tier"` // "FLASH" | "PRO"
	Rationale     string  `json:"rationale"`
	RiskScore     float64 `json:"risk_score"`
}

// StrategicModelRouter dynamically selects the optimal model tier based on task complexity and financial risk.
type StrategicModelRouter struct {
	CriticalThreshold float64
}

// NewStrategicModelRouter creates a new model routing instance.
func NewStrategicModelRouter() *StrategicModelRouter {
	return &StrategicModelRouter{
		CriticalThreshold: 10000.00,
	}
}

// RouteForRecord evaluates a reconciliation record and selects Flash vs Pro.
func (r *StrategicModelRouter) RouteForRecord(ctx context.Context, record schemas.CorrelatedReconciliationRecord) RoutingDecision {
	// High-risk financial variance (> $10,000) or complex multi-party dispute routes to Pro
	if record.VarianceAmount >= r.CriticalThreshold || record.VarianceArchetype == schemas.ArchetypeCriticalDiscrepancy {
		decision := RoutingDecision{
			SelectedModel: ModelPro,
			Tier:          "PRO",
			Rationale:     "High-value financial variance (> $10k) requires multi-step deep reasoning and audit compliance analysis.",
			RiskScore:     0.95,
		}
		logger.Info(ctx, "Strategic Model Router: Routing to PRO model tier",
			"contract_id", record.ContractID,
			"variance_amount", record.VarianceAmount,
			"selected_model", decision.SelectedModel,
		)
		return decision
	}

	// Standard low/medium variance routes to Flash for ultra-low latency & cost efficiency
	decision := RoutingDecision{
		SelectedModel: ModelFlash,
		Tier:          "FLASH",
		Rationale:     "Standard reconciliation task routed to Flash for sub-100ms processing and cost efficiency.",
		RiskScore:     0.20,
	}
	logger.Info(ctx, "Strategic Model Router: Routing to FLASH model tier",
		"contract_id", record.ContractID,
		"variance_amount", record.VarianceAmount,
		"selected_model", decision.SelectedModel,
	)
	return decision
}
