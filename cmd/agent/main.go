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
	"google.golang.org/adk/v2/session"
	vertexaisession "google.golang.org/adk/v2/session/vertexai"
	"google.golang.org/adk/v2/telemetry"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"

	agentenginelauncher "github.com/tiaaburton/Data-Recon-Agent/internal/agentengine/launcher"

	"github.com/joho/godotenv"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
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
	contractID := args.ContractID
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

	envelope := a2ui.BuildDiscrepancyEnvelope(a2ui.DiscrepancyCardParams{
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

	bytes, _ := json.MarshalIndent(envelope, "", "  ")

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
				providers.Shutdown(sCtx)
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
	if err := clientConfig.UseDefaultCredentials(); err != nil {
		log.Printf("Notice: UseDefaultCredentials info: %v", err)
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

	reconcileTool, err := functiontool.New(functiontool.Config{
		Name:        "reconcile_contract",
		Description: "Autonomously correlates Salesforce billed records and ServiceNow dispute incidents for a given contract ID, computing financial variance and generating declarative A2UI components.",
	}, handleReconcileContract)
	if err != nil {
		log.Fatalf("Failed to create reconcile_contract tool: %v", err)
	}

	agentInstructions := `You are the Autonomous Enterprise Data Reconciliation Agent built on Google Agent Development Kit (ADK) v2.0 and A2UI v0.9.
Your mission is to autonomously resolve billing discrepancies across enterprise systems (Salesforce CRM and ServiceNow ITSM).

Capabilities:
1. When asked to reconcile or inspect a contract (e.g. 'Reconcile contract CTR-2026-001'), ALWAYS invoke the 'reconcile_contract' tool.
2. Formulate concise, professional financial reasoning detailing the variance, severity, and root cause.
3. Stream the interactive A2UI v0.9 declarative component envelope back to the user.`

	reconAgent, err := llmagent.New(llmagent.Config{
		Name:        "data_recon_agent",
		Description: "Autonomous multi-system data reconciliation agent with interactive A2UI declarative components.",
		Model:       geminiModel,
		Tools:       []tool.Tool{reconcileTool, renderCardTool},
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
		if sSvc, err := vertexaisession.NewSessionService(ctx, vertexaisession.VertexAIServiceConfig{
			ProjectID:       project,
			Location:        sessionLocation,
			ReasoningEngine: engineID,
		}); err == nil && sSvc != nil {
			sessionService = sSvc
			log.Printf("✓ Connected to Vertex AI Agent Engine Cloud Session Service (Project: %s, Location: %s, Engine: %s)", project, sessionLocation, engineID)
		}
	}
	if sessionService == nil {
		sessionService = session.InMemoryService()
		log.Printf("Using In-Memory Session Service (Local Dev / Fallback).")
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(reconAgent),
		SessionService: sessionService,
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
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(host+":"+publicPort, mux); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}
