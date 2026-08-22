package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

// VerificationReport details the live check across both sandboxes.
type VerificationReport struct {
	TotalEvaluated int     `json:"total_evaluated"`
	SFVerified     int     `json:"salesforce_verified"`
	SNVerified     int     `json:"servicenow_verified"`
	MatchRatePct   float64 `json:"match_rate_percentage"`
}

// VerifyRecords queries both live instances to confirm records were loaded.
func VerifyRecords(ctx context.Context, sfdcURL, sfdcToken, snowURL, snowUser, snowPass string, records []schemas.CorrelatedReconciliationRecord) (*VerificationReport, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	report := &VerificationReport{TotalEvaluated: len(records)}

	// 1. Verify Salesforce Records
	if sfdcURL != "" && sfdcToken != "" {
		soql := "SELECT Id, Name, Amount FROM Opportunity WHERE Name LIKE '%Contract CTR-2026%'"
		queryURL := fmt.Sprintf("%s/services/data/v60.0/query?q=%s", sfdcURL, url.QueryEscape(soql))
		req, _ := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
		req.Header.Set("Authorization", "Bearer "+sfdcToken)

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var sfRes struct {
				TotalSize int `json:"totalSize"`
			}
			body, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(body, &sfRes); err == nil {
				report.SFVerified = sfRes.TotalSize
			}
		}
	}

	// 2. Verify ServiceNow Records
	if snowURL != "" && snowUser != "" {
		queryURL := fmt.Sprintf("%s/api/now/table/incident?sysparm_query=correlation_idSTARTSWITHcorr-uuid&sysparm_fields=number,correlation_id", snowURL)
		req, _ := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
		req.SetBasicAuth(snowUser, snowPass)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var snRes struct {
				Result []struct {
					Number        string `json:"number"`
					CorrelationID string `json:"correlation_id"`
				} `json:"result"`
			}
			body, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(body, &snRes); err == nil {
				report.SNVerified = len(snRes.Result)
			}
		}
	}

	if report.TotalEvaluated > 0 {
		report.MatchRatePct = (float64(report.SFVerified+report.SNVerified) / float64(report.TotalEvaluated*2)) * 100
	}

	return report, nil
}
