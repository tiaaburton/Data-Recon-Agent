package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/envutil"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/verifier"
)

func main() {
	inputPath := flag.String("input", "data/correlated_recon_500.json", "Path to synthetic dataset")
	flag.Parse()

	envutil.LoadEnvFile(".env")
	envutil.LoadEnvFile(".env.local")

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("failed to read dataset file: %v", err)
	}

	var records []schemas.CorrelatedReconciliationRecord
	if uErr := json.Unmarshal(data, &records); uErr != nil {
		log.Fatalf("failed to parse dataset JSON: %v", uErr)
	}

	sfdcURL := os.Getenv("SFDC_INSTANCE_URL")
	sfdcToken := os.Getenv("SFDC_ACCESS_TOKEN")
	snowURL := os.Getenv("SERVICENOW_INSTANCE_URL")
	snowUser := os.Getenv("SERVICENOW_USERNAME")
	snowPass := os.Getenv("SERVICENOW_PASSWORD")

	fmt.Printf("================================================================================\n")
	fmt.Printf("          LIVE DATA SEEDING VALIDATION REPORT (Salesforce & ServiceNow)         \n")
	fmt.Printf("================================================================================\n")

	report, err := verifier.VerifyRecords(context.Background(), sfdcURL, sfdcToken, snowURL, snowUser, snowPass, records)
	if err != nil {
		log.Fatalf("verification error: %v", err)
	}

	fmt.Printf("[Salesforce CRM]\n")
	fmt.Printf("  Endpoint: %s\n", sfdcURL)
	fmt.Printf("  Target Object: Opportunity\n")
	fmt.Printf("  Records Found: %d / %d\n\n", report.SFVerified, report.TotalEvaluated)

	fmt.Printf("[ServiceNow ITSM]\n")
	fmt.Printf("  Endpoint: %s\n", snowURL)
	fmt.Printf("  Target Table: incident\n")
	fmt.Printf("  Records Found: %d / %d\n\n", report.SNVerified, report.TotalEvaluated)

	fmt.Printf("[Multi-System Correlation Health]\n")
	fmt.Printf("  Match Rate Score: %.1f%%\n", report.MatchRatePct)
	if report.MatchRatePct > 0 {
		fmt.Printf("  Status: READY FOR AGENTIC RECONCILIATION\n")
	} else {
		fmt.Printf("  Status: PENDING LIVE SEEDING (Set .env.local credentials and run cmd/loader)\n")
	}
	fmt.Printf("================================================================================\n")
}
