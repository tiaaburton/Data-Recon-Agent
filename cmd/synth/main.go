package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/synth"
)

func main() {
	count := flag.Int("count", 500, "Number of correlated reconciliation records to generate")
	outputPath := flag.String("output", "data/correlated_recon_500.json", "Output file path for generated JSON")
	flag.Parse()

	fmt.Printf("=== Enterprise Data Synthesizer ===\n")
	fmt.Printf("Generating %d correlated records across Salesforce and ServiceNow...\n", *count)

	records, err := synth.GenerateRecords(*count)
	if err != nil {
		log.Fatalf("failed to generate records: %v", err)
	}

	// Ensure output directory exists
	dir := filepath.Dir(*outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(*outputPath, data, 0644); err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}

	// Calculate counts
	var match, timing, rounding, critical int
	for _, r := range records {
		switch r.VarianceArchetype {
		case schemas.ArchetypeMatch:
			match++
		case schemas.ArchetypeTimingLag:
			timing++
		case schemas.ArchetypeTaxFXRounding:
			rounding++
		case schemas.ArchetypeCriticalDiscrepancy:
			critical++
		}
	}

	fmt.Printf("SUCCESS: Saved %d records to %s\n\n", len(records), *outputPath)
	fmt.Printf("Dataset Archetype Distribution:\n")
	fmt.Printf("  • Perfect Two-Way Match:      %4d (%.1f%%)\n", match, float64(match)/float64(*count)*100)
	fmt.Printf("  • Invoicing & Timing Lags:    %4d (%.1f%%)\n", timing, float64(timing)/float64(*count)*100)
	fmt.Printf("  • Regional Tax & FX Rounding: %4d (%.1f%%)\n", rounding, float64(rounding)/float64(*count)*100)
	fmt.Printf("  • Critical Discrepancies:     %4d (%.1f%%)\n", critical, float64(critical)/float64(*count)*100)
	fmt.Printf("===================================\n")
}
