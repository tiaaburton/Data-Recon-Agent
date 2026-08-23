package synth

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

var enterpriseAccounts = []string{
	"Acme Global Technologies",
	"Globex Logistics Corporation",
	"Initech Healthcare Systems",
	"Cyberdyne Industrial AI",
	"Soylent Financial Solutions",
	"Massive Dynamic Labs",
	"Hooli Cloud Enterprise",
	"Stark Industries Telecom",
	"Wayne Enterprise Cloud",
	"Umbrella Biotech Analytics",
}

// GenerateRecords produces N mathematically correlated reconciliation records.
func GenerateRecords(count int) ([]schemas.CorrelatedReconciliationRecord, error) {
	// #nosec G404 -- Synthetic dataset generation for evaluation benchmarks
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	records := make([]schemas.CorrelatedReconciliationRecord, 0, count)

	// Distribution breakdown
	matchCount := int(float64(count) * 0.40)                            // 40%
	timingCount := int(float64(count) * 0.30)                           // 30%
	roundingCount := int(float64(count) * 0.20)                         // 20%
	criticalCount := count - (matchCount + timingCount + roundingCount) // 10%

	idx := 1
	baseDate := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	// 1. Generate Perfect Matches
	for i := 0; i < matchCount; i++ {
		rec := generateSingleRecord(r, idx, schemas.ArchetypeMatch, baseDate)
		records = append(records, rec)
		idx++
	}

	// 2. Generate Timing Lags
	for i := 0; i < timingCount; i++ {
		rec := generateSingleRecord(r, idx, schemas.ArchetypeTimingLag, baseDate)
		records = append(records, rec)
		idx++
	}

	// 3. Generate Tax / FX Rounding
	for i := 0; i < roundingCount; i++ {
		rec := generateSingleRecord(r, idx, schemas.ArchetypeTaxFXRounding, baseDate)
		records = append(records, rec)
		idx++
	}

	// 4. Generate Critical Discrepancies
	for i := 0; i < criticalCount; i++ {
		rec := generateSingleRecord(r, idx, schemas.ArchetypeCriticalDiscrepancy, baseDate)
		records = append(records, rec)
		idx++
	}

	return records, nil
}

func generateSingleRecord(r *rand.Rand, index int, archetype schemas.VarianceArchetype, baseDate time.Time) schemas.CorrelatedReconciliationRecord {
	corrID := fmt.Sprintf("corr-uuid-%04d", index)
	contractID := fmt.Sprintf("CTR-2026-%03d", index)
	accountID := fmt.Sprintf("ACC-%04d", index)
	accountName := enterpriseAccounts[index%len(enterpriseAccounts)]
	closeDate := baseDate.AddDate(0, 0, index%60)

	baseAmount := float64(50000 + (r.Intn(100) * 1000))
	billedAmount := baseAmount
	agreedCap := baseAmount
	variance := 0.0

	var sfDescMap map[string]any
	var snDescMap map[string]any
	shortDesc := ""
	category := "billing"
	state := "6" // Resolved

	switch archetype {
	case schemas.ArchetypeMatch:
		variance = 0.0
		sfDescMap = map[string]any{
			"correlation_id":    corrID,
			"contract_id":       contractID,
			"agreed_cap":        agreedCap,
			"billed_amount":     billedAmount,
			"variance_expected": 0.0,
			"status":            "RECONCILED",
		}
		snDescMap = map[string]any{
			"correlation_id":  corrID,
			"contract_id":     contractID,
			"disputed_amount": 0.0,
			"resolution":      "Monthly consumption matched contract baseline perfectly.",
		}
		shortDesc = fmt.Sprintf("Standard Billing Check - %s - %s", contractID, accountName)

	case schemas.ArchetypeTimingLag:
		variance = 0.0
		lagDays := 3 + r.Intn(3) // 3-5 days
		sfDescMap = map[string]any{
			"correlation_id":     corrID,
			"contract_id":        contractID,
			"agreed_cap":         agreedCap,
			"billed_amount":      billedAmount,
			"provision_lag_days": lagDays,
			"status":             "TIMING_LAG_APPROVED",
		}
		snDescMap = map[string]any{
			"correlation_id":    corrID,
			"contract_id":       contractID,
			"provisioning_date": closeDate.AddDate(0, 0, lagDays).Format("2006-01-02"),
			"resolution":        fmt.Sprintf("Provisioning completed within %d-day grace period.", lagDays),
		}
		shortDesc = fmt.Sprintf("Timing Window Verification - %s (%d-day Lag)", contractID, lagDays)

	case schemas.ArchetypeTaxFXRounding:
		variance = math.Round((0.50+r.Float64()*4.0)*100) / 100 // $0.50 - $4.50
		billedAmount = agreedCap + variance
		sfDescMap = map[string]any{
			"correlation_id": corrID,
			"contract_id":    contractID,
			"agreed_cap":     agreedCap,
			"billed_amount":  billedAmount,
			"fx_variance":    variance,
			"status":         "FX_ROUNDING_TOLERANCE",
		}
		snDescMap = map[string]any{
			"correlation_id":  corrID,
			"contract_id":     contractID,
			"disputed_amount": variance,
			"resolution":      fmt.Sprintf("Tax and VAT fractional rounding variance of $%.2f within tolerance threshold.", variance),
		}
		shortDesc = fmt.Sprintf("Tax & FX Rounding Inquiry - %s ($%.2f delta)", contractID, variance)

	case schemas.ArchetypeCriticalDiscrepancy:
		if index == 1 {
			// CUJ-01 Benchmark: Acme Corp $14,250.00
			agreedCap = 130750.00
			billedAmount = 145000.00
			variance = 14250.00
		} else {
			variance = float64(5000 + (r.Intn(40) * 1000))
			billedAmount = agreedCap + variance
		}
		sfDescMap = map[string]any{
			"correlation_id":    corrID,
			"contract_id":       contractID,
			"agreed_cap":        agreedCap,
			"billed_amount":     billedAmount,
			"variance_expected": -variance,
			"status":            "CRITICAL_OVERAGE_PENDING_HITL",
		}
		snDescMap = map[string]any{
			"correlation_id":   corrID,
			"contract_id":      contractID,
			"disputed_amount":  variance,
			"discrepancy_type": "OVERAGE_CREDIT_APPROVED",
			"resolution":       fmt.Sprintf("Finance approved credit memo of $%.2f for overage beyond contract cap.", variance),
		}
		shortDesc = fmt.Sprintf("CRITICAL Billing Dispute - %s ($%.2f Overage)", contractID, variance)
	}

	sfDescBytes, _ := json.Marshal(sfDescMap)
	snDescBytes, _ := json.Marshal(snDescMap)

	sfSeed := schemas.SalesforceOpportunitySeed{
		Name:        fmt.Sprintf("[%s] - Contract %s", accountName, contractID),
		Amount:      billedAmount,
		StageName:   "Closed Won",
		CloseDate:   closeDate.Format("2006-01-02"),
		Description: string(sfDescBytes),
	}

	snSeed := schemas.ServiceNowIncidentSeed{
		ShortDescription: shortDesc,
		Description:      string(snDescBytes),
		Category:         category,
		Impact:           "2",
		Urgency:          "2",
		State:            state,
		CorrelationID:    corrID,
		CloseCode:        "Solved (Permanently)",
		CloseNotes:       "Resolved via automated cross-system billing reconciliation.",
	}

	return schemas.CorrelatedReconciliationRecord{
		CorrelationID:     corrID,
		ContractID:        contractID,
		AccountID:         accountID,
		AccountName:       accountName,
		VarianceArchetype: archetype,
		BilledAmount:      billedAmount,
		AgreedCap:         agreedCap,
		VarianceAmount:    variance,
		Currency:          "USD",
		DetectedAt:        time.Now().UTC(),
		Salesforce:        sfSeed,
		ServiceNow:        snSeed,
	}
}
