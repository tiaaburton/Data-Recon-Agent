# Enterprise Data Reconciliation Agent: Technical Component Blueprint

- **Language & Runtime**: Go 1.22+ (ADK 2.0)
- **Framework**: Vertex AI Agent Engine, Google Cloud ADK (Go)
- **Protocol**: Agent-to-Agent (A2A), Agent-to-UI (A2UI v0.8), Model Context Protocol (MCP)

---

## 1. Directory Structure & Module Topology

```text
Data-Recon-Agent/
├── .github/
│   └── workflows/
│       ├── ci.yaml                   # Go lint, test, schema validation, and promptfoo eval
│       └── cd.yaml                   # Cloud Run & Vertex AI Agent Engine deployment
├── cmd/
│   ├── server/                       # Main entrypoint: Cloud Run BYO-MCP & Webhook Server
│   │   └── main.go
│   └── synth/                        # CLI Synthetic Data Generator tool
│       └── main.go
├── docs/                             # Full architectural & deployment documentation
│   ├── README.md
│   ├── architecture.md
│   ├── architecture.drawio
│   ├── blueprint.md
│   ├── deployment_guide.md
│   ├── a2ui_custom_catalog.md
│   ├── code_review_matrix.md
│   ├── backlog_task_breakdown.md
│   ├── blog_post.md
│   ├── research_paper.md
│   └── adr/                          # Architecture Decision Records
│       ├── README.md
│       ├── 0001-golang-agent-runtime.md
│       ├── 0002-vertex-agent-engine-compaction.md
│       ├── 0003-multi-agent-coordinator-async-memory.md
│       ├── 0004-strategic-model-routing.md
│       ├── 0005-hitl-signed-webhook-intercepts.md
│       ├── 0006-cloud-dlp-pii-redaction.md
│       ├── 0007-custom-a2ui-catalog-styling.md
│       ├── 0008-pubsub-handcrafted-toolset.md
│       └── 0009-byo-mcp-cloud-run-cmek.md
├── pkg/
│   ├── a2ui/                         # A2UI v0.8 declarative card builders & schemas
│   │   ├── catalog.go
│   │   ├── schemas.go
│   │   └── templates.go
│   ├── agent/                        # Core Go ADK 2.0 agent definitions & constitutions
│   │   ├── coordinator.go
│   │   ├── system_prompt.go
│   │   └── router.go
│   ├── compaction/                   # Token-based sliding window compaction engine
│   │   └── compactor.go
│   ├── connectors/                   # Enterprise 3P/1P connectors & SAP Mock
│   │   ├── sap_mock.go               # MockReconciler interface & OData simulation
│   │   ├── servicenow.go
│   │   ├── salesforce.go
│   │   ├── slack.go
│   │   └── drive.go
│   ├── errorhandling/                # Guided Error Handling structs & recovery advice
│   │   └── errors.go
│   ├── hitl/                         # Human-in-the-loop cryptographically signed gate
│   │   └── webhook.go
│   ├── memory/                       # Async memory engine (Goroutines + channels)
│   │   └── async_store.go
│   ├── middleware/                   # Cloud DLP PII redaction & OTel tracing
│   │   ├── dlp_redactor.go
│   │   └── otel_tracing.go
│   ├── observability/                # slog structured logger with Intent vs Outcome
│   │   ├── logger.go
│   │   └── intent_outcome.go
│   ├── pubsubtool/                   # google.adk.tools.pubsub Go implementation
│   │   └── toolset.go
│   ├── schemas/                      # Strict input/output JSON schemas with Go tags
│   │   ├── discrepancy.go
│   │   ├── tickets.go
│   │   └── billing.go
│   ├── synthetic/                    # High-speed synthetic data generator
│   │   └── generator.go
│   └── tools/                        # Handcrafted Go ADK 2.0 tool definitions
│       ├── servicenow_tools.go
│       ├── salesforce_tools.go
│       └── sap_tools.go
├── terraform/                        # Production GCP Infrastructure as Code
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── terraform.tfvars.example
│   └── modules/
│       ├── agent_engine/
│       ├── cloud_run/
│       ├── firestore/
│       ├── pubsub/
│       └── security_kms/
├── tests/                            # Automated Regression Suite & Golden Dataset Tests
│   ├── golden/
│   │   └── discrepancies_golden.json
│   ├── eval_test.go
│   └── mock_test.go
├── go.mod
├── go.sum
└── README.md
```

---

## 2. Go Type Contracts & Strict Schemas

### 2.1. Enterprise Discrepancy & Reconciliation Domain Models

```go
package schemas

import (
	"time"
)

// DiscrepancySeverity defines the priority classification of a detected variance.
type DiscrepancySeverity string

const (
	SeverityLow      DiscrepancySeverity = "LOW"
	SeverityMedium   DiscrepancySeverity = "MEDIUM"
	SeverityHigh     DiscrepancySeverity = "HIGH"
	SeverityCritical DiscrepancySeverity = "CRITICAL"
)

// ReconciliationStatus tracks the end-to-end resolution state machine.
type ReconciliationStatus string

const (
	StatusDetected       ReconciliationStatus = "DETECTED"
	StatusUnderReview    ReconciliationStatus = "UNDER_REVIEW"
	StatusPendingHITL    ReconciliationStatus = "PENDING_HITL_APPROVAL"
	StatusApproved       ReconciliationStatus = "APPROVED"
	StatusMutated        ReconciliationStatus = "MUTATED"
	StatusAutoReconciled ReconciliationStatus = "AUTO_RECONCILED"
	StatusRejected       ReconciliationStatus = "REJECTED"
)

// ReconciliationEvent represents an ingested cross-system discrepancy trigger.
type ReconciliationEvent struct {
	EventID        string              `json:"event_id" validate:"required,uuid4"`
	CorrelationID  string              `json:"correlation_id" validate:"required"`
	AccountID      string              `json:"account_id" validate:"required"`
	AccountName    string              `json:"account_name" validate:"required"`
	ServiceNowINC  string              `json:"servicenow_inc_id,omitempty"`
	SalesforceCTR  string              `json:"salesforce_ctr_id,omitempty"`
	SAPInvoiceID   string              `json:"sap_invoice_id,omitempty"`
	VarianceAmount float64             `json:"variance_amount"`
	Currency       string              `json:"currency" validate:"required,len=3"`
	Severity       DiscrepancySeverity `json:"severity" validate:"required,oneof=LOW MEDIUM HIGH CRITICAL"`
	DetectedAt     time.Time           `json:"detected_at" validate:"required"`
	Description    string              `json:"description" validate:"required"`
}

// MultiSystemSnapshot encapsulates the raw retrieved states across systems.
type MultiSystemSnapshot struct {
	ServiceNowTicket *ServiceNowTicketDetails `json:"servicenow_ticket,omitempty"`
	SalesforceRecord *SalesforceContract      `json:"salesforce_contract,omitempty"`
	SAPBillingDoc    *SAPBillingDocument      `json:"sap_billing_doc,omitempty"`
	DeltaCalculations map[string]FieldDelta   `json:"delta_calculations"`
}

// FieldDelta captures a specific field-level mismatch across systems.
type FieldDelta struct {
	FieldName         string `json:"field_name"`
	ServiceNowVal     any    `json:"servicenow_value"`
	SalesforceVal     any    `json:"salesforce_value"`
	SAPVal            any    `json:"sap_value"`
	RecommendedSource string `json:"recommended_source"`
	ConfidenceScore   float64 `json:"confidence_score"`
}
```

---

## 3. SAP `MockReconciler` Interface & Implementation

```go
package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

// MockReconciler abstracts SAP OData v4 interactions for environments where direct SAP sandbox is unavailable.
type MockReconciler interface {
	GetBillingDocument(ctx context.Context, invoiceID string) (*schemas.SAPBillingDocument, error)
	GetJournalEntry(ctx context.Context, accountID string, postingDate time.Time) (*schemas.SAPJournalEntry, error)
	StageCreditMemo(ctx context.Context, req schemas.CreditMemoRequest) (*schemas.CreditMemoResult, error)
	SimulateODataPost(ctx context.Context, endpoint string, payload []byte) (int, []byte, error)
}

type sapMockService struct {
	invoices map[string]*schemas.SAPBillingDocument
}

func NewSAPMockService(seedData []*schemas.SAPBillingDocument) MockReconciler {
	idx := make(map[string]*schemas.SAPBillingDocument)
	for _, doc := range seedData {
		idx[doc.InvoiceID] = doc
	}
	return &sapMockService{invoices: idx}
}

func (s *sapMockService) GetBillingDocument(ctx context.Context, invoiceID string) (*schemas.SAPBillingDocument, error) {
	doc, exists := s.invoices[invoiceID]
	if !exists {
		return nil, fmt.Errorf("SAP OData v4 entity '/BillingDocument('%s')' not found", invoiceID)
	}
	return doc, nil
}

func (s *sapMockService) StageCreditMemo(ctx context.Context, req schemas.CreditMemoRequest) (*schemas.CreditMemoResult, error) {
	return &schemas.CreditMemoResult{
		CreditMemoID: fmt.Sprintf("CM-%d", time.Now().UnixNano()%100000),
		Status:       "STAGED",
		Amount:       req.Amount,
		Currency:     req.Currency,
		CreatedOn:    time.Now(),
	}, nil
}

func (s *sapMockService) GetJournalEntry(ctx context.Context, accountID string, postingDate time.Time) (*schemas.SAPJournalEntry, error) {
	return &schemas.SAPJournalEntry{
		JournalID:   fmt.Sprintf("JE-%s-%d", accountID, postingDate.Year()),
		AccountID:   accountID,
		PostingDate: postingDate,
		Balance:     0.00,
	}, nil
}

func (s *sapMockService) SimulateODataPost(ctx context.Context, endpoint string, payload []byte) (int, []byte, error) {
	return 201, []byte(`{"d":{"status":"SUCCESS","message":"OData entity created"}}`), nil
}
```

---

## 4. Guided Error Handling & LLM Recovery Architecture

```go
package errorhandling

import (
	"encoding/json"
	"fmt"
)

// GuidedError provides actionable recovery instructions back to the LLM agent.
type GuidedError struct {
	ErrorCode        string         `json:"error_code"`
	FailedSystem     string         `json:"failed_system"`
	Message          string         `json:"message"`
	RecoveryAction   string         `json:"recovery_action"`
	SuggestedPayload map[string]any `json:"suggested_retry_payload,omitempty"`
}

func (e *GuidedError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func NewNotFoundError(system, resourceID, resourceType, altSearchTool string, altParam string) *GuidedError {
	return &GuidedError{
		ErrorCode:    "ENTITY_NOT_FOUND",
		FailedSystem: system,
		Message:      fmt.Sprintf("%s '%s' was not found in %s.", resourceType, resourceID, system),
		RecoveryAction: fmt.Sprintf(
			"Do not immediately fail the reconciliation. Call alternative discovery tool '%s' using parameter '%s' to resolve the canonical ID.",
			altSearchTool, altParam,
		),
		SuggestedPayload: map[string]any{
			"recommended_tool": altSearchTool,
			"search_key":       altParam,
		},
	}
}

func NewRateLimitError(system string, retryAfterSeconds int) *GuidedError {
	return &GuidedError{
		ErrorCode:    "RATE_LIMIT_EXCEEDED",
		FailedSystem: system,
		Message:      fmt.Sprintf("%s API rate limit reached. Backoff required.", system),
		RecoveryAction: fmt.Sprintf(
			"Pause tool invocation for %d seconds or execute fallback cache lookup.", retryAfterSeconds,
		),
	}
}
```

---

## 5. Async Memory Engine (Goroutines & Buffered Channels)

```go
package memory

import (
	"context"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"
)

type MemoryEvent struct {
	SessionID string
	TurnIndex int
	Summary   string
	Timestamp time.Time
}

type AsyncMemoryEngine struct {
	client    *firestore.Client
	eventChan chan MemoryEvent
	logger    *slog.Logger
	stopChan  chan struct{}
}

func NewAsyncMemoryEngine(client *firestore.Client, bufferSize int, logger *slog.Logger) *AsyncMemoryEngine {
	engine := &AsyncMemoryEngine{
		client:    client,
		eventChan: make(chan MemoryEvent, bufferSize),
		logger:    logger,
		stopChan:  make(chan struct{}),
	}
	go engine.workerLoop()
	return engine
}

func (m *AsyncMemoryEngine) EmitMemoryEvent(event MemoryEvent) {
	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("Async memory event channel full, dropping memory update to protect latency",
			slog.String("session_id", event.SessionID),
		)
	}
}

func (m *AsyncMemoryEngine) workerLoop() {
	for {
		select {
		case event := <-m.eventChan:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := m.client.Collection("recon_sessions").Doc(event.SessionID).
				Collection("long_term_memories").Add(ctx, map[string]any{
				"turn_index": event.TurnIndex,
				"summary":    event.Summary,
				"saved_at":   event.Timestamp,
			})
			cancel()
			if err != nil {
				m.logger.Error("Failed to persist async memory to Firestore",
					slog.String("session_id", event.SessionID),
					slog.String("error", err.Error()),
				)
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *AsyncMemoryEngine) Close() {
	close(m.stopChan)
}
```

---

## 6. Cryptographically Signed HITL Webhook Intercept

```go
package hitl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type SignedApprovalToken struct {
	SessionID   string    `json:"session_id"`
	MutationID  string    `json:"mutation_id"`
	ApprovedBy  string    `json:"approved_by"`
	ExpiresAt   time.Time `json:"expires_at"`
	Signature   string    `json:"signature"`
}

type HITLValidator struct {
	secretKey []byte
}

func NewHITLValidator(secretKey []byte) *HITLValidator {
	return &HITLValidator{secretKey: secretKey}
}

func (h *HITLValidator) GenerateSignature(sessionID, mutationID, approvedBy string, expiresAt time.Time) string {
	payload := fmt.Sprintf("%s:%s:%s:%d", sessionID, mutationID, approvedBy, expiresAt.Unix())
	mac := hmac.New(sha256.New, h.secretKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *HITLValidator) VerifyApproval(token SignedApprovalToken) error {
	if time.Now().After(token.ExpiresAt) {
		return errors.New("approval token expired")
	}
	expectedSig := h.GenerateSignature(token.SessionID, token.MutationID, token.ApprovedBy, token.ExpiresAt)
	if !hmac.Equal([]byte(token.Signature), []byte(expectedSig)) {
		return errors.New("invalid cryptographic signature: mutation blocked")
	}
	return nil
}
```

---

## 7. Live Multi-System Seeder Contracts (`pkg/seeder/`)

### 7.1. Salesforce Live Opportunity & Contract Seeder (`pkg/seeder/salesforce.go`)

```go
package seeder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SalesforceLiveSeeder struct {
	InstanceURL string
	AccessToken string
	HTTPClient  *http.Client
}

type SFOpportunitySeed struct {
	Name          string  `json:"Name"`
	Amount        float64 `json:"Amount"`
	StageName     string  `json:"StageName"`
	CloseDate     string  `json:"CloseDate"`
	ContractID    string  `json:"Contract_Id__c"`
	CorrelationID string  `json:"Correlation_Id__c"`
}

func NewSalesforceLiveSeeder(instanceURL, accessToken string) *SalesforceLiveSeeder {
	return &SalesforceLiveSeeder{
		InstanceURL: instanceURL,
		AccessToken: accessToken,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SalesforceLiveSeeder) SeedOpportunity(ctx context.Context, opp SFOpportunitySeed) (string, error) {
	endpoint := fmt.Sprintf("%s/services/data/v60.0/sobjects/Opportunity", s.InstanceURL)
	payload, _ := json.Marshal(opp)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("salesforce api error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("salesforce upsert failed with status %d", resp.StatusCode)
	}

	var res struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.ID, nil
}
```

### 7.2. ServiceNow Live Dispute Ticket Seeder (`pkg/seeder/servicenow.go`)

```go
package seeder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ServiceNowLiveSeeder struct {
	InstanceURL string
	Username    string
	Password    string
	HTTPClient  *http.Client
}

type SNIncidentSeed struct {
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Impact           string `json:"impact"`
	Urgency          string `json:"urgency"`
	CorrelationID    string `json:"correlation_id"`
	DisputedAmount   string `json:"u_disputed_amount"`
}

func NewServiceNowLiveSeeder(instanceURL, username, password string) *ServiceNowLiveSeeder {
	return &ServiceNowLiveSeeder{
		InstanceURL: instanceURL,
		Username:    username,
		Password:    password,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *ServiceNowLiveSeeder) SeedIncident(ctx context.Context, inc SNIncidentSeed) (string, error) {
	endpoint := fmt.Sprintf("%s/api/now/table/incident", s.InstanceURL)
	payload, _ := json.Marshal(inc)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(s.Username, s.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("servicenow table api error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("servicenow table api returned status %d", resp.StatusCode)
	}

	var res struct {
		Result struct {
			SysID  string `json:"sys_id"`
			Number string `json:"number"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.Result.Number, nil
}
```

---

## 8. Gemini Enterprise Streaming Gateway Contract (`pkg/gateway/`)

```go
package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type SSEStreamer struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

func NewSSEStreamer(w http.ResponseWriter) (*SSEStreamer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported by client connection")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return &SSEStreamer{writer: w, flusher: flusher}, nil
}

func (s *SSEStreamer) EmitThought(step, detail string) {
	data, _ := json.Marshal(map[string]string{"step": step, "detail": detail})
	fmt.Fprintf(s.writer, "event: agent_thought\ndata: %s\n\n", data)
	s.flusher.Flush()
}

func (s *SSEStreamer) EmitA2UIPayload(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.writer, "event: a2ui_render\ndata: %s\n\n", data)
	s.flusher.Flush()
	return nil
}
```
