package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/agent"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/envutil"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

func main() {
	contractID := flag.String("contract", "CTR-2026-001", "Contract ID to reconcile")
	datasetPath := flag.String("dataset", "data/correlated_recon_500.json", "Path to synthetic reconciliation dataset")
	model := flag.String("model", "gemini-3.7-flash-preview", "Gemini model to execute")
	flag.Parse()

	envutil.LoadEnvFile(".env.local")
	envutil.LoadEnvFile(".env")

	fmt.Printf("================================================================================\n")
	fmt.Printf("        AUTONOMOUS DATA RECONCILIATION AGENT (Go ADK v2 / A2UI v0.9)           \n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("Target Contract: %s | Model: %s\n\n", *contractID, *model)

	data, err := os.ReadFile(*datasetPath)
	if err != nil {
		log.Fatalf("failed to read dataset %s: %v", *datasetPath, err)
	}

	var records []schemas.CorrelatedReconciliationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		log.Fatalf("failed to unmarshal dataset: %v", err)
	}

	// Find the requested contract
	var targetRecord *schemas.CorrelatedReconciliationRecord
	for _, r := range records {
		if r.ContractID == *contractID {
			targetRecord = &r
			break
		}
	}

	if targetRecord == nil {
		targetRecord = &records[0] // fallback to first
	}

	coordinator := agent.NewCoordinatorEngine(*model)
	outcome, err := coordinator.ReconcileRecord(context.Background(), *targetRecord)
	if err != nil {
		log.Fatalf("reconciliation failed: %v", err)
	}

	fmt.Printf("[Agent Thought & Delta Engine]\n")
	fmt.Printf("  • Account:       %s\n", outcome.AccountName)
	fmt.Printf("  • Contract:      %s\n", outcome.ContractID)
	fmt.Printf("  • Billed Amount: $%.2f\n", targetRecord.BilledAmount)
	fmt.Printf("  • Agreed Cap:    $%.2f\n", targetRecord.AgreedCap)
	fmt.Printf("  • Variance:      $%.2f (Severity: %s)\n\n", outcome.VarianceAmount, outcome.Severity)

	fmt.Printf("[Synthesized Root Cause & Recommendation]\n")
	fmt.Printf("  • Explanation:   %s\n", outcome.DiscrepancyCause)
	fmt.Printf("  • Action:        %s\n\n", outcome.Recommendation)

	a2uiJSON, _ := json.MarshalIndent(outcome.A2UIEnvelope, "", "  ")
	fmt.Printf("================================================================================\n")
	fmt.Printf("                    STREAMED A2UI v0.9 DECLARATIVE PAYLOAD                      \n")
	fmt.Printf("================================================================================\n")
	fmt.Println(string(a2uiJSON))
	fmt.Printf("================================================================================\n")
}
