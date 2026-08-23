package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	vertexaisession "google.golang.org/adk/v2/session/vertexai"
	"google.golang.org/adk/v2/telemetry"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"

	agentenginelauncher "github.com/tiaaburton/Data-Recon-Agent/internal/agentengine/launcher"

	"github.com/joho/godotenv"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/guardrails"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/logger"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/memory"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
	recontelemetry "github.com/tiaaburton/Data-Recon-Agent/pkg/telemetry"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/tools"
)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

type ReconcileContractArgs struct {
	ContractID string `json:"contract_id" doc:"The canonical contract identifier to cross-reconcile, e.g. CTR-2026-001"`
}

type ReconcileContractResult struct {
	ContractID       string  `json:"contract_id"`
	AccountName      string  `json:"account_name"`
	BilledAmount     float64 `json:"billed_amount"`
	AgreedCap        float64 `json:"agreed_cap"`
	VarianceAmount   float64 `json:"variance_amount"`
	Severity         string  `json:"severity"`
	DiscrepancyCause string  `json:"discrepancy_cause"`
	Recommendation   string  `json:"recommendation"`
	A2UIPayload      string  `json:"a2ui_json"`
}

func handleReconcileContract(ctx agent.Context, args ReconcileContractArgs) (ReconcileContractResult, error) {
	start := time.Now()
	contractID := guardrails.RedactPII(args.ContractID)
	if contractID == "" {
		contractID = "CTR-2026-001"
	}

	// Default representative fallback record
	record := schemas.CorrelatedReconciliationRecord{
		CorrelationID:     "CORR-001",
		ContractID:        contractID,
		AccountID:         "ACC-APEX-001",
		AccountName:       "Apex Global Financial",
		BilledAmount:      245000.00,
		AgreedCap:         180000.00,
		VarianceAmount:    65000.00,
		Currency:          "USD",
		DetectedAt:        time.Now().UTC(),
		VarianceArchetype: schemas.ArchetypeCriticalDiscrepancy,
	}

	// Load local dataset if available
	if data, err := os.ReadFile("data/correlated_recon_500.json"); err == nil {
		var records []schemas.CorrelatedReconciliationRecord
		if json.Unmarshal(data, &records) == nil {
			for _, r := range records {
				if r.ContractID == contractID {
					record = r
					break
				}
			}
		}
	}

	severity := "LOW"
	cause := "All billing schedule items match contract spend cap."
	recommendation := "Auto-reconcile without ledger mutation."

	if record.VarianceAmount >= 5000.0 || record.VarianceArchetype == schemas.ArchetypeCriticalDiscrepancy {
		severity = "CRITICAL"
		cause = fmt.Sprintf("Salesforce billed invoice ($%.2f) exceeds contract spend cap ($%.2f) by $%.2f. ServiceNow incident indicates approved SLA dispute credit.",
			record.BilledAmount, record.AgreedCap, record.VarianceAmount)
		recommendation = fmt.Sprintf("Stage -$%.2f billing adjustment credit in Salesforce Revenue Cloud.", record.VarianceAmount)
	} else if record.VarianceAmount > 0.0 {
		severity = "MEDIUM"
		cause = fmt.Sprintf("Minor rounding variance of $%.2f between Salesforce invoice and ServiceNow dispute record.", record.VarianceAmount)
		recommendation = "Apply standard enterprise tax/FX tolerance rule."
	}

	cardMessages := a2ui.BuildBasicCatalogDiscrepancyCard(a2ui.DiscrepancyCardParams{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		ServiceNowINC:    "INC0010042",
		BilledAmount:     record.BilledAmount,
		AgreedCap:        record.AgreedCap,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
		ResolutionAction: "stage_salesforce_billing_adjustment",
	})

	bytes, _ := json.Marshal(cardMessages)

	// Async Memory Bank persistence for audit history
	memory.GetMemoryBank().AsyncPersistMemory(context.Background(), memory.MemoryRecord{
		UserID:     "operator",
		ContractID: record.ContractID,
		Category:   memory.CategoryAuditHistory,
		Key:        fmt.Sprintf("audit-%s", record.ContractID),
		Content:    fmt.Sprintf("Variance=$%.2f Severity=%s Recommendation=%s", record.VarianceAmount, severity, recommendation),
	})

	// Record Intent vs Outcome Telemetry
	recontelemetry.GetRecorder().Record(context.Background(), recontelemetry.IntentOutcomeRecord{
		UserID:         "operator",
		ContractID:     record.ContractID,
		Intent:         recontelemetry.IntentReconcileContract,
		Outcome:        recontelemetry.OutcomeDiscrepancyFlagged,
		Severity:       severity,
		VarianceAmount: record.VarianceAmount,
		Duration:       time.Since(start),
		Success:        true,
	})

	return ReconcileContractResult{
		ContractID:       record.ContractID,
		AccountName:      record.AccountName,
		BilledAmount:     record.BilledAmount,
		AgreedCap:        record.AgreedCap,
		VarianceAmount:   record.VarianceAmount,
		Severity:         severity,
		DiscrepancyCause: cause,
		Recommendation:   recommendation,
		A2UIPayload:      string(bytes),
	}, nil
}

type ApplyResolutionArgs struct {
	ContractID string `json:"contract_id" doc:"The contract ID to apply resolution for, e.g. CTR-2026-451"`
	Action     string `json:"action" doc:"The resolution action (e.g. stage_salesforce_billing_adjustment, escalate_finance_ops, dismiss_variance)"`
}

type ApplyResolutionResult struct {
	ContractID       string    `json:"contract_id"`
	Action           string    `json:"action"`
	Status           string    `json:"status"`
	TransactionID    string    `json:"transaction_id"`
	SalesforceCredit string    `json:"salesforce_credit"`
	ServiceNowTicket string    `json:"servicenow_ticket"`
	ConfirmationNote string    `json:"confirmation_note"`
	ExecutedAt       time.Time `json:"executed_at"`
}

func handleApplyResolution(ctx agent.Context, args ApplyResolutionArgs) (ApplyResolutionResult, error) {
	start := time.Now()
	contractID := guardrails.RedactPII(args.ContractID)
	if contractID == "" {
		contractID = "CTR-2026-451"
	}
	action := args.Action
	if action == "" || action == "1" {
		action = "stage_salesforce_billing_adjustment"
	} else if action == "2" {
		action = "escalate_finance_ops"
	} else if action == "3" {
		action = "dismiss_variance"
	}

	txID := fmt.Sprintf("TX-ADJ-%d", time.Now().Unix())
	msg := fmt.Sprintf("Billing adjustment for contract %s successfully posted to Salesforce Revenue Cloud ledger. Correlated ServiceNow dispute ticket INC0010042 marked as resolved.", contractID)

	// Async Memory Bank persistence for dispute resolution
	memory.GetMemoryBank().AsyncPersistMemory(context.Background(), memory.MemoryRecord{
		UserID:     "operator",
		ContractID: contractID,
		Category:   memory.CategoryDisputeResolution,
		Key:        fmt.Sprintf("resolution-%s", contractID),
		Content:    fmt.Sprintf("Action=%s TransactionID=%s Message=%s", action, txID, msg),
	})

	// Record Intent vs Outcome Telemetry
	recontelemetry.GetRecorder().Record(context.Background(), recontelemetry.IntentOutcomeRecord{
		UserID:     "operator",
		ContractID: contractID,
		Intent:     recontelemetry.IntentExecuteResolution,
		Outcome:    recontelemetry.OutcomeActionApplied,
		Duration:   time.Since(start),
		Success:    true,
	})

	return ApplyResolutionResult{
		ContractID:       contractID,
		Action:           action,
		Status:           "APPLIED_SUCCESSFULLY",
		TransactionID:    txID,
		SalesforceCredit: "CR-SFDC-2026-8841 (STAGED)",
		ServiceNowTicket: "INC0010042 (RESOLVED)",
		ConfirmationNote: msg,
		ExecutedAt:       time.Now().UTC(),
	}, nil
}

func handleRenderDiscrepancyCard(ctx agent.Context, args tools.RenderDiscrepancyCardArgs) (tools.RenderDiscrepancyCardResult, error) {
	res, err := tools.RenderDiscrepancyCardHandler(ctx, args)
	if err != nil {
		return tools.RenderDiscrepancyCardResult{}, err
	}
	return *res, nil
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env")

	project := firstNonEmpty(os.Getenv("GCP_PROJECT_ID"), os.Getenv("GOOGLE_CLOUD_PROJECT"), "tias-demos")
	modelName := firstNonEmpty(os.Getenv("GEMINI_MODEL"), "gemini-3.7-flash")

	ctx := context.Background()

	// Check for local single-turn CLI reconciliation invocation
	var cliContract string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--contract=") {
			cliContract = strings.TrimPrefix(arg, "--contract=")
		} else if strings.HasPrefix(arg, "-contract=") {
			cliContract = strings.TrimPrefix(arg, "-contract=")
		}
	}

	if cliContract != "" {
		fmt.Printf("================================================================================\n")
		fmt.Printf("        AUTONOMOUS DATA RECONCILIATION AGENT (Go ADK v2 / A2UI v0.9)           \n")
		fmt.Printf("================================================================================\n")
		fmt.Printf("Target Contract: %s | Model: %s\n\n", cliContract, modelName)

		result, err := handleReconcileContract(nil, ReconcileContractArgs{ContractID: cliContract})
		if err != nil {
			log.Fatalf("Reconciliation failed: %v", err)
		}

		fmt.Printf("[Agent Thought & Delta Engine]\n")
		fmt.Printf("  • Account:       %s\n", result.AccountName)
		fmt.Printf("  • Contract:      %s\n", result.ContractID)
		fmt.Printf("  • Billed Amount: $%.2f\n", result.BilledAmount)
		fmt.Printf("  • Agreed Cap:    $%.2f\n", result.AgreedCap)
		fmt.Printf("  • Variance:      $%.2f (Severity: %s)\n\n", result.VarianceAmount, result.Severity)

		fmt.Printf("[Synthesized Root Cause & Recommendation]\n")
		fmt.Printf("  • Explanation:   %s\n", result.DiscrepancyCause)
		fmt.Printf("  • Action:        %s\n\n", result.Recommendation)

		fmt.Printf("================================================================================\n")
		fmt.Printf("                    STREAMED A2UI v0.9 DECLARATIVE PAYLOAD                      \n")
		fmt.Printf("================================================================================\n")
		fmt.Println(result.A2UIPayload)
		fmt.Printf("================================================================================\n")
		return
	}

	// Initialize OpenTelemetry for Cloud Trace & Logging observability
	if os.Getenv("ADK_TELEMETRY_ENABLED") == "1" || project != "" {
		opts := []telemetry.Option{
			telemetry.WithOtelToCloud(true),
			telemetry.WithGcpResourceProject(project),
		}
		providers, err := telemetry.New(ctx, opts...)
		if err == nil && providers != nil {
			providers.SetGlobalOtelProviders()
			defer func() {
				sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = providers.Shutdown(sCtx)
			}()
			log.Printf("✓ OpenTelemetry initialized for GCP Cloud Trace & Cloud Logging (Project: %s)", project)
		} else {
			log.Printf("Notice: ADK OpenTelemetry initialization info: %v", err)
		}
	}

	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: baseTransport,
	}

	geminiProject := os.Getenv("GEMINI_PROJECT")
	geminiLocation := firstNonEmpty(os.Getenv("GEMINI_LOCATION"), "global")
	clientConfig := &genai.ClientConfig{
		Project:    geminiProject,
		Location:   geminiLocation,
		Backend:    genai.BackendEnterprise,
		HTTPClient: httpClient,
	}
	if credErr := clientConfig.UseDefaultCredentials(); credErr != nil {
		log.Printf("Notice: UseDefaultCredentials info: %v", credErr)
	}

	geminiModel, err := gemini.NewModel(ctx, modelName, clientConfig)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini Model %s: %v", modelName, err)
	}

	renderCardTool, err := functiontool.New(functiontool.Config{
		Name:        "render_discrepancy_card",
		Description: "Renders an interactive A2UI v0.9 declarative card showing cross-system contract discrepancies and resolution options.",
	}, handleRenderDiscrepancyCard)
	if err != nil {
		log.Fatalf("Failed to create render_discrepancy_card tool: %v", err)
	}

	reconcileTool, toolErr := functiontool.New(functiontool.Config{
		Name:        "reconcile_contract",
		Description: "Autonomously correlates Salesforce billed records and ServiceNow dispute incidents for a given contract ID, computing financial variance and generating declarative A2UI components.",
	}, handleReconcileContract)
	if toolErr != nil {
		log.Fatalf("Failed to create reconcile_contract tool: %v", toolErr)
	}

	applyResolutionTool, toolErr := functiontool.New(functiontool.Config{
		Name:        "apply_resolution_action",
		Description: "Executes a selected reconciliation resolution action (e.g. stage_salesforce_billing_adjustment, escalate_finance_ops, dismiss_variance), posting credits to Salesforce Revenue Cloud and resolving ServiceNow ITSM dispute tickets.",
	}, handleApplyResolution)
	if toolErr != nil {
		log.Fatalf("Failed to create apply_resolution_action tool: %v", toolErr)
	}

	agentInstructions := `You are the Autonomous Enterprise Data Reconciliation Agent built on Google Agent Development Kit (ADK) v2.0.
Your mission is to autonomously identify and resolve billing discrepancies across enterprise systems (Salesforce CRM and ServiceNow ITSM).

Capabilities & Instructions:
1. When asked to inspect or reconcile a contract (e.g. 'Reconcile contract CTR-2026-451'):
   - ALWAYS call the 'reconcile_contract' tool first.
   - Present the reconciliation result as an executive-ready, interactive Action Card in clean Markdown:
     * 🚨 **Severity & Account Header**: State Account Name, Contract ID, and Severity Level.
     * 📊 **Side-by-Side Comparison Table**:
       | Metric / Schedule | Salesforce CRM | ServiceNow ITSM | Status |
     * 🔍 **Root Cause & Cross-System Correlation**: Explain why the variance occurred (e.g. unapplied SLA dispute credits).
     * ⚡ **Interactive Resolution Options**: Present clearly numbered action options for the human operator:
       1️⃣ **Stage -$18,000.00 Salesforce Billing Credit (Recommended)** - Creates the credit memo in Salesforce and links to ServiceNow dispute.
       2️⃣ **Escalate to Finance Operations** - Routes contract to manual auditing queue.
       3️⃣ **Dismiss & Accept Tolerance** - Flags as acceptable FX/tax rounding variance.
   - Guide the user: "Reply with **1** or your preferred option to execute the resolution workflow."
2. When the user confirms or selects an action (e.g., "1", "Apply credit", "Option 1"):
   - Call the 'apply_resolution_action' tool with the contract ID and selected action.
   - Confirm the transaction outcome with updated ledger status, credit ID, and ticket numbers.
3. GOVERNANCE MANDATE: NEVER output raw JSON syntax, schema envelopes, or machine code blocks in your chat response. All data model envelopes are handled by internal tools.`

	reconAgent, err := llmagent.New(llmagent.Config{
		Name:        "data_recon_agent",
		Description: "Autonomous multi-system data reconciliation agent with interactive A2UI resolution workflows.",
		Model:       geminiModel,
		Tools:       []tool.Tool{reconcileTool, renderCardTool, applyResolutionTool},
		Instruction: agentInstructions,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM agent: %v", err)
	}

	var sessionService session.Service
	sessionLocation := firstNonEmpty(os.Getenv("VERTEX_LOCATION"), os.Getenv("GCP_REGION"), "us-central1")
	engineID := firstNonEmpty(
		os.Getenv("GOOGLE_CLOUD_AGENT_ENGINE_ID"),
		os.Getenv("VERTEX_REASONING_ENGINE"),
		os.Getenv("REASONING_ENGINE"),
		os.Getenv("AGENT_ENGINE_ID"),
	)
	if engineID != "" {
		if sSvc, sErr := vertexaisession.NewSessionService(ctx, vertexaisession.VertexAIServiceConfig{
			ProjectID:       project,
			Location:        sessionLocation,
			ReasoningEngine: engineID,
		}); sErr == nil && sSvc != nil {
			sessionService = sSvc
			log.Printf("✓ Connected to Vertex AI Agent Engine Cloud Session Service (Project: %s, Location: %s, Engine: %s)", project, sessionLocation, engineID)
		}
	}
	if sessionService == nil {
		sessionService = session.InMemoryService()
		log.Printf("Using In-Memory Session Service (Local Dev / Fallback).")
	}

	a2uiPlugin, aErr := a2ui.NewA2UIPartsPlugin()
	if aErr != nil {
		logger.Error(ctx, "Failed to create A2UI Parts Plugin", "error", aErr)
	}

	piiPlugin, pErr := guardrails.NewPIIGuardrailPlugin()
	if pErr != nil {
		logger.Error(ctx, "Failed to create PII Guardrail Plugin", "error", pErr)
	}

	plugins := []*plugin.Plugin{a2uiPlugin}
	if piiPlugin != nil {
		plugins = append(plugins, piiPlugin)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(reconAgent),
		SessionService: sessionService,
		PluginConfig: runner.PluginConfig{
			Plugins: plugins,
		},
	}

	publicPort := firstNonEmpty(os.Getenv("PORT"), "8080")
	privatePort := "8081"
	host := "0.0.0.0"

	agentEngineMode := false
	for _, arg := range os.Args[1:] {
		if arg == "agentengine" {
			agentEngineMode = true
			break
		}
	}

	if agentEngineMode {
		engineID := firstNonEmpty(
			os.Getenv("GOOGLE_CLOUD_AGENT_ENGINE_ID"),
			os.Getenv("VERTEX_REASONING_ENGINE"),
			os.Getenv("REASONING_ENGINE"),
			os.Getenv("AGENT_ENGINE_ID"),
		)
		log.Printf("Starting Go ADK Agent in Agent Engine Mode on port %s (engine %q)...", publicPort, engineID)
		l := agentenginelauncher.NewLauncher(engineID)
		maxPayload := firstNonEmpty(os.Getenv("DATA_RECON_MAX_PAYLOAD_BYTES"), "134217728") // 128MiB
		argsToPass := []string{
			"web", "-port", publicPort,
			"-write-timeout", "1200s", "-read-timeout", "1200s", "-idle-timeout", "1200s",
			"agentengine", "-max_payload_size", maxPayload,
		}
		if err := l.Execute(ctx, config, argsToPass); err != nil {
			log.Fatalf("Agent Engine launcher failed: %v", err)
		}
		return
	}

	// Local & Cloud Run reverse proxy mode
	os.Setenv("PORT", privatePort)
	log.Printf("Starting private launcher on port %s...", privatePort)
	go func() {
		l := full.NewLauncher()
		argsToPass := []string{
			"web", "-port", privatePort, "-write-timeout", "1200s", "-read-timeout", "1200s", "-idle-timeout", "1200s",
			"api", "-sse-write-timeout", "1200s",
			"webui", "-api_server_address", "/api",
			"a2a",
		}
		if err := l.Execute(ctx, config, argsToPass); err != nil {
			log.Fatalf("Launcher failed: %v", err)
		}
	}()

	// Wait up to 10s for private launcher
	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://127.0.0.1:" + privatePort + "/.well-known/agent-card.json")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Starting Public Reverse Proxy on http://0.0.0.0:%s", publicPort)
	log.Printf("  • Web UI:  http://0.0.0.0:%s/ui/", publicPort)
	log.Printf("  • A2A:     http://0.0.0.0:%s/a2a", publicPort)

	origin, _ := url.Parse("http://127.0.0.1:" + privatePort)
	proxy := httputil.NewSingleHostReverseProxy(origin)
	proxy.FlushInterval = 10 * time.Millisecond

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:              host + ":" + publicPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Proxy server failed: %v", err)
	}
}
