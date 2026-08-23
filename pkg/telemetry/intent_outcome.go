package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/logger"
)

// IntentType categorizes incoming user requests.
type IntentType string

const (
	IntentReconcileContract      IntentType = "RECONCILE_CONTRACT"
	IntentInvestigateDiscrepancy IntentType = "INVESTIGATE_DISCREPANCY"
	IntentExecuteResolution      IntentType = "EXECUTE_RESOLUTION"
	IntentGeneralInquiry         IntentType = "GENERAL_INQUIRY"
)

// OutcomeStatus defines the reconciliation and execution resolution status.
type OutcomeStatus string

const (
	OutcomeReconciledClean    OutcomeStatus = "RECONCILED_CLEAN"
	OutcomeDiscrepancyFlagged OutcomeStatus = "DISCREPANCY_FLAGGED"
	OutcomeActionApplied      OutcomeStatus = "ACTION_APPLIED"
	OutcomeError              OutcomeStatus = "ERROR"
)

// IntentOutcomeRecord captures end-to-end telemetry for each user interaction.
type IntentOutcomeRecord struct {
	TraceID        string        `json:"trace_id"`
	SessionID      string        `json:"session_id"`
	UserID         string        `json:"user_id"`
	ContractID     string        `json:"contract_id,omitempty"`
	Intent         IntentType    `json:"intent"`
	Outcome        OutcomeStatus `json:"outcome"`
	Severity       string        `json:"severity,omitempty"`
	VarianceAmount float64       `json:"variance_amount,omitempty"`
	Duration       time.Duration `json:"duration"`
	Timestamp      time.Time     `json:"timestamp"`
	Success        bool          `json:"success"`
	ErrorMessage   string        `json:"error_message,omitempty"`
}

// TelemetryRecorder aggregates Intent vs. Outcome operational metrics.
type TelemetryRecorder struct {
	mu      sync.RWMutex
	records []IntentOutcomeRecord
}

var globalRecorder = &TelemetryRecorder{
	records: make([]IntentOutcomeRecord, 0, 1000),
}

// GetRecorder returns the global telemetry recorder instance.
func GetRecorder() *TelemetryRecorder {
	return globalRecorder
}

// Record captures an intent-to-outcome lifecycle event and outputs structured JSON logs.
func (r *TelemetryRecorder) Record(ctx context.Context, record IntentOutcomeRecord) {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	r.mu.Lock()
	r.records = append(r.records, record)
	r.mu.Unlock()

	// Emit structured JSON telemetry event for Google Cloud Logging & BigQuery export
	logger.Info(ctx, "Reconciliation Telemetry: Intent vs Outcome",
		"trace_id", record.TraceID,
		"session_id", record.SessionID,
		"user_id", record.UserID,
		"contract_id", record.ContractID,
		"intent", string(record.Intent),
		"outcome", string(record.Outcome),
		"severity", record.Severity,
		"variance_amount", record.VarianceAmount,
		"duration_ms", record.Duration.Milliseconds(),
		"success", record.Success,
		"error", record.ErrorMessage,
	)
}

// SummaryStats returns operational summary metrics for observability.
type SummaryStats struct {
	TotalRequests         int     `json:"total_requests"`
	SuccessRate           float64 `json:"success_rate"`
	TotalVarianceDetected float64 `json:"total_variance_detected"`
	CriticalDiscrepancies int     `json:"critical_discrepancies"`
	MeanDurationMs        float64 `json:"mean_duration_ms"`
}

// GetSummary returns rolled-up observability stats across all interactions.
func (r *TelemetryRecorder) GetSummary() SummaryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.records)
	if total == 0 {
		return SummaryStats{}
	}

	successCount := 0
	var totalVariance float64
	var totalDuration time.Duration
	criticalCount := 0

	for _, rec := range r.records {
		if rec.Success {
			successCount++
		}
		totalVariance += rec.VarianceAmount
		totalDuration += rec.Duration
		if rec.Severity == "CRITICAL" {
			criticalCount++
		}
	}

	return SummaryStats{
		TotalRequests:         total,
		SuccessRate:           float64(successCount) / float64(total) * 100.0,
		TotalVarianceDetected: totalVariance,
		CriticalDiscrepancies: criticalCount,
		MeanDurationMs:        float64(totalDuration.Milliseconds()) / float64(total),
	}
}
