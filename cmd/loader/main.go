package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/seeder"
)

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func main() {
	inputPath := flag.String("input", "data/correlated_recon_500.json", "Path to synthetic dataset")
	target := flag.String("target", "all", "Target platform: salesforce, servicenow, or all")
	dryRun := flag.Bool("dry-run", false, "Simulate insertion without calling live APIs")
	limit := flag.Int("limit", 0, "Max number of records to load (0 = all)")
	flag.Parse()

	// Load local credentials if available
	loadEnvFile(".env.local")
	loadEnvFile(".env")

	fmt.Printf("=== Enterprise Data Seeder ===\n")
	fmt.Printf("Dataset: %s | Target: %s | DryRun: %v\n\n", *inputPath, *target, *dryRun)

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("failed to read dataset file: %v", err)
	}

	var records []schemas.CorrelatedReconciliationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		log.Fatalf("failed to parse dataset JSON: %v", err)
	}

	if *limit > 0 && *limit < len(records) {
		records = records[:*limit]
	}

	fmt.Printf("Loaded %d records for ingestion.\n", len(records))

	ctx := context.Background()

	// 1. Salesforce Seeding
	if *target == "salesforce" || *target == "all" {
		fmt.Printf("\n--- Seeding Salesforce CRM ---\n")
		sfdcURL := os.Getenv("SFDC_INSTANCE_URL")
		sfdcToken := os.Getenv("SFDC_ACCESS_TOKEN")
		sfdcUser := os.Getenv("SFDC_USERNAME")
		sfdcPass := os.Getenv("SFDC_PASSWORD")
		sfdcClientID := os.Getenv("SFDC_CLIENT_ID")
		sfdcClientSecret := os.Getenv("SFDC_CLIENT_SECRET")

		if *dryRun {
			fmt.Printf("[DRY-RUN] Would load %d Opportunities to %s\n", len(records), sfdcURL)
		} else if sfdcURL == "" {
			fmt.Printf("[SKIP] SFDC_INSTANCE_URL not set in environment or .env.local\n")
		} else {
			if sfdcToken == "" && sfdcClientID != "" && sfdcUser != "" {
				fmt.Printf("Authenticating with Salesforce OAuth...\n")
				token, err := seeder.AuthenticateSalesforce(sfdcURL, sfdcClientID, sfdcClientSecret, sfdcUser, sfdcPass)
				if err != nil {
					log.Printf("Salesforce auth failed: %v", err)
				} else {
					sfdcToken = token
				}
			}

			if sfdcToken != "" {
				sfdcSeeder := seeder.NewSalesforceSeeder(sfdcURL, sfdcToken)
				successCount := 0
				for i, r := range records {
					id, err := sfdcSeeder.SeedOpportunity(ctx, r.Salesforce)
					if err != nil {
						fmt.Printf("  [%d/%d] Error seeding %s: %v\n", i+1, len(records), r.ContractID, err)
					} else {
						successCount++
						if (i+1)%50 == 0 || i == len(records)-1 {
							fmt.Printf("  [%d/%d] Seeded Opportunity ID: %s (%s)\n", i+1, len(records), id, r.ContractID)
						}
					}
					time.Sleep(20 * time.Millisecond) // rate limit protection
				}
				fmt.Printf("Salesforce Seeding Complete: %d / %d loaded successfully.\n", successCount, len(records))
			} else {
				fmt.Printf("[SKIP] No valid Salesforce access token or credentials available.\n")
			}
		}
	}

	// 2. ServiceNow Seeding
	if *target == "servicenow" || *target == "all" {
		fmt.Printf("\n--- Seeding ServiceNow ITSM ---\n")
		snowURL := os.Getenv("SERVICENOW_INSTANCE_URL")
		snowUser := os.Getenv("SERVICENOW_USERNAME")
		snowPass := os.Getenv("SERVICENOW_PASSWORD")

		if *dryRun {
			fmt.Printf("[DRY-RUN] Would load %d Incidents to %s\n", len(records), snowURL)
		} else if snowURL == "" || snowUser == "" {
			fmt.Printf("[SKIP] SERVICENOW_INSTANCE_URL or credentials not set in environment or .env.local\n")
		} else {
			snowSeeder := seeder.NewServiceNowSeeder(snowURL, snowUser, snowPass)
			successCount := 0
			for i, r := range records {
				num, err := snowSeeder.SeedIncident(ctx, r.ServiceNow)
				if err != nil {
					fmt.Printf("  [%d/%d] Error seeding %s: %v\n", i+1, len(records), r.CorrelationID, err)
				} else {
					successCount++
					if (i+1)%50 == 0 || i == len(records)-1 {
						fmt.Printf("  [%d/%d] Seeded Incident: %s (Correlation: %s)\n", i+1, len(records), num, r.CorrelationID)
					}
				}
				time.Sleep(20 * time.Millisecond) // rate limit protection
			}
			fmt.Printf("ServiceNow Seeding Complete: %d / %d loaded successfully.\n", successCount, len(records))
		}
	}

	fmt.Printf("\n===================================\n")
}
