package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/envutil"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/seeder"
)

func main() {
	inputPath := flag.String("input", "data/correlated_recon_500.json", "Path to synthetic dataset")
	target := flag.String("target", "all", "Target platform: salesforce, servicenow, or all")
	dryRun := flag.Bool("dry-run", false, "Simulate insertion without calling live APIs")
	limit := flag.Int("limit", 0, "Max number of records to load (0 = all)")
	nonInteractive := flag.Bool("non-interactive", false, "Disable interactive prompts for missing credentials")
	flag.Parse()

	// Load local credentials if available
	envutil.LoadEnvFile(".env.local")
	envutil.LoadEnvFile(".env")

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

	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 1. Salesforce Seeding
	// -------------------------------------------------------------------------
	if *target == "salesforce" || *target == "all" {
		fmt.Printf("\n--- Seeding Salesforce CRM ---\n")
		sfdcURL := os.Getenv("SFDC_INSTANCE_URL")
		sfdcToken := os.Getenv("SFDC_ACCESS_TOKEN")
		sfdcUser := os.Getenv("SFDC_USERNAME")
		sfdcPass := os.Getenv("SFDC_PASSWORD")
		sfdcClientID := os.Getenv("SFDC_CLIENT_ID")
		sfdcClientSecret := os.Getenv("SFDC_CLIENT_SECRET")

		sfDryRun := *dryRun

		// Check if credentials are missing and we are allowed to prompt
		if !sfDryRun && (sfdcURL == "" || (sfdcToken == "" && sfdcPass == "")) {
			if !*nonInteractive {
				choices := []string{
					"Enter Salesforce credentials now (saves to .env.local)",
					"Run in DRY-RUN mode (simulate loading without API calls)",
					"Skip Salesforce and continue",
					"Exit",
				}
				choice := envutil.PromptChoice(reader, "[!] Salesforce credentials missing in environment/.env.local.", choices, 2)
				switch choice {
				case 1:
					sfdcURL = envutil.PromptString(reader, "Salesforce Instance URL", "https://orgfarm-b2f2a8eb8d-dev-ed.develop.my.salesforce.com")
					sfdcUser = envutil.PromptString(reader, "Salesforce Username", "tiaburton.dad9d78120c9@agentforce.com")
					sfdcPass = envutil.PromptString(reader, "Salesforce Password + Security Token", "")
					sfdcToken = envutil.PromptString(reader, "Salesforce Access Token (Optional if using password)", "")
					
					updates := map[string]string{
						"SFDC_INSTANCE_URL": sfdcURL,
						"SFDC_USERNAME":     sfdcUser,
						"SFDC_PASSWORD":     sfdcPass,
					}
					if sfdcToken != "" {
						updates["SFDC_ACCESS_TOKEN"] = sfdcToken
					}
					if err := envutil.UpdateEnvLocal(".env.local", updates); err != nil {
						fmt.Printf("Warning: failed to write to .env.local: %v\n", err)
					} else {
						fmt.Printf("Saved Salesforce credentials to .env.local\n")
					}
				case 2:
					sfDryRun = true
				case 3:
					fmt.Printf("[SKIP] Skipping Salesforce seeding.\n")
					goto SnowSection
				case 4:
					fmt.Printf("Exiting.\n")
					os.Exit(0)
				}
			} else {
				sfDryRun = true
			}
		}

		if sfDryRun {
			fmt.Printf("[DRY-RUN] Simulated loading %d Opportunities to %s\n", len(records), sfdcURL)
		} else {
			if sfdcToken == "" && sfdcClientID != "" && sfdcUser != "" && sfdcPass != "" {
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
					time.Sleep(20 * time.Millisecond)
				}
				fmt.Printf("Salesforce Seeding Complete: %d / %d loaded successfully.\n", successCount, len(records))
			} else {
				fmt.Printf("[SKIP] No valid Salesforce access token or credentials provided.\n")
			}
		}
	}

SnowSection:
	// -------------------------------------------------------------------------
	// 2. ServiceNow Seeding
	// -------------------------------------------------------------------------
	if *target == "servicenow" || *target == "all" {
		fmt.Printf("\n--- Seeding ServiceNow ITSM ---\n")
		snowURL := os.Getenv("SERVICENOW_INSTANCE_URL")
		snowUser := os.Getenv("SERVICENOW_USERNAME")
		snowPass := os.Getenv("SERVICENOW_PASSWORD")

		snowDryRun := *dryRun

		if !snowDryRun && (snowURL == "" || snowUser == "" || snowPass == "") {
			if !*nonInteractive {
				choices := []string{
					"Enter ServiceNow credentials now (saves to .env.local)",
					"Run in DRY-RUN mode (simulate loading without API calls)",
					"Skip ServiceNow and finish",
					"Exit",
				}
				choice := envutil.PromptChoice(reader, "[!] ServiceNow credentials missing in environment/.env.local.", choices, 2)
				switch choice {
				case 1:
					snowURL = envutil.PromptString(reader, "ServiceNow Instance URL", "https://dev410998.service-now.com")
					snowUser = envutil.PromptString(reader, "ServiceNow Username", "admin")
					snowPass = envutil.PromptString(reader, "ServiceNow Password", "")

					updates := map[string]string{
						"SERVICENOW_INSTANCE_URL": snowURL,
						"SERVICENOW_USERNAME":     snowUser,
						"SERVICENOW_PASSWORD":     snowPass,
					}
					if err := envutil.UpdateEnvLocal(".env.local", updates); err != nil {
						fmt.Printf("Warning: failed to write to .env.local: %v\n", err)
					} else {
						fmt.Printf("Saved ServiceNow credentials to .env.local\n")
					}
				case 2:
					snowDryRun = true
				case 3:
					fmt.Printf("[SKIP] Skipping ServiceNow seeding.\n")
					goto FinishSection
				case 4:
					fmt.Printf("Exiting.\n")
					os.Exit(0)
				}
			} else {
				snowDryRun = true
			}
		}

		if snowDryRun {
			fmt.Printf("[DRY-RUN] Simulated loading %d Incidents to %s\n", len(records), snowURL)
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
				time.Sleep(20 * time.Millisecond)
			}
			fmt.Printf("ServiceNow Seeding Complete: %d / %d loaded successfully.\n", successCount, len(records))
		}
	}

FinishSection:
	fmt.Printf("\n===================================\n")
}
