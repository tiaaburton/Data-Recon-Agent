# Enterprise Data Reconciliation Agent: Technical Component Blueprint

- **Language & Runtime**: Go 1.22+ (ADK 2.0)
- **Framework**: Vertex AI Agent Engine, Google Cloud ADK (Go)
- **Protocol**: Agent-to-Agent (A2A), Agent-to-UI (A2UI v0.9), Model Context Protocol (MCP)

---

## 1. Directory Structure & Module Topology

```text
Data-Recon-Agent/
├── .env.example                      # Base environment template (committed)
├── .env.local.example                # Local sandbox credentials template (committed)
├── .env.local                        # Local developer credentials (gitignored)
├── .gitignore                        # Git ignore rules protecting local credentials
├── main.go                           # Root server entrypoint: Cloud Run BYO-MCP & Gemini Extension
├── go.mod                            # Go 1.22 module definition
├── go.sum
├── Dockerfile                        # Multi-stage optimized Go container build
├── terraform/                        # Infrastructure as Code
│   ├── main.tf
│   ├── variables.tf
│   └── outputs.tf
├── docs/                             # Complete documentation suite
│   ├── README.md                     # Documentation hub
│   ├── critical_user_journeys.md     # 5 Enterprise CUJs & natural language queries
│   ├── architecture.md               # RFC/TDD & C4 diagrams
│   ├── architecture.drawio           # Visual Draw.io diagram
│   ├── blueprint.md                  # Go code contracts & interfaces
│   ├── gemini_enterprise_integration.md # Extension manifest & SSE streaming
│   ├── synthetic_data_seeding_guide.md  # Live Salesforce/ServiceNow seeding guide
│   ├── figma_design_spec.md          # Vector asset specs & Figma tokens
│   ├── a2ui_custom_catalog.md        # A2UI v0.9 declarative schema
│   ├── tools_reference.md            # ADK & MCP tool reference manual
│   ├── operations_runbook.md         # SRE Day-2 maintenance & DLQ replay
│   ├── evaluation_benchmark.md       # 500-sample golden evaluation guide
│   ├── code_review_matrix.md         # 95/95 compliance scorecard
│   ├── backlog_task_breakdown.md     # 23-task implementation tracking
│   ├── blog_post.md                  # Technical blog post
│   └── adr/                          # 9 Architecture Decision Records
├── tools/                            # Standalone CLI tools (thin wrappers over pkg/)
│   ├── synth/main.go                 # Calls pkg/synth.Generate()
│   ├── loader/main.go                # Calls pkg/seeder.SeedAll()
│   └── verifier/main.go              # Calls pkg/verifier.VerifyAll()
├── pkg/                              # Modular, highly importable Go packages
│   ├── a2ui/                         # A2UI v0.9 declarative card builders
│   ├── agent/                        # Coordinator & sub-agent orchestrators
│   ├── compaction/                   # Sliding window token compactor
│   ├── connectors/                   # ServiceNow & Salesforce connectors
│   ├── errorhandling/                # GuidedError recovery models
│   ├── gateway/                      # Gemini Enterprise SSE streamer & OpenAPI
│   ├── hitl/                         # Ed25519 & HMAC-SHA256 signature validator
│   ├── memory/                       # Async channel persistence engine
│   ├── middleware/                   # Cloud DLP PII redactor & OTel tracing
│   ├── observability/                # slog structured Intent vs Outcome logger
│   ├── pubsubtool/                   # google.adk.tools.pubsub implementation
│   ├── router/                       # Strategic model selector (Flash 3.7 vs Pro 3.1)
│   ├── schemas/                      # Strict JSON schemas & Go structs
│   ├── seeder/                       # Live Salesforce & ServiceNow API seeders
│   ├── synth/                        # High-speed correlated synthetic generator
│   └── verifier/                     # Programmatic live data validation engine
└── tests/                            # Integration tests & golden evaluation
    ├── cuj_test.go                   # Verification of all 5 enterprise CUJs
    ├── regression_test.go            # Automated regression suite
    └── golden/                       # 500-sample benchmark datasets
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
	VarianceAmount float64             `json:"variance_amount"`
	Currency       string              `json:"currency" validate:"required,len=3"`
	Severity       DiscrepancySeverity `json:"severity" validate:"required,oneof=LOW MEDIUM HIGH CRITICAL"`
	DetectedAt     time.Time           `json:"detected_at" validate:"required"`
	Description    string              `json:"description" validate:"required"`
}

// MultiSystemSnapshot encapsulates the raw retrieved states across systems.
type MultiSystemSnapshot struct {
	ServiceNowTicket  *ServiceNowTicketDetails `json:"servicenow_ticket,omitempty"`
	SalesforceRecord  *SalesforceContract      `json:"salesforce_contract,omitempty"`
	DeltaCalculations map[string]FieldDelta    `json:"delta_calculations"`
}

// FieldDelta captures a specific field-level mismatch across systems.
type FieldDelta struct {
	FieldName         string  `json:"field_name"`
	ServiceNowVal     any     `json:"servicenow_value"`
	SalesforceVal     any     `json:"salesforce_value"`
	RecommendedSource string  `json:"recommended_source"`
	ConfidenceScore   float64 `json:"confidence_score"`
}
```

---

## 3. Salesforce & ServiceNow Connector Interfaces

```go
package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

// SalesforceConnector abstracts Salesforce CRM & Revenue Cloud interactions.
type SalesforceConnector interface {
	GetContract(ctx context.Context, contractID string) (*schemas.SalesforceContract, error)
	GetBillingSchedule(ctx context.Context, accountID string) (*schemas.BillingSchedule, error)
	StageBillingAdjustment(ctx context.Context, req schemas.BillingAdjustmentRequest) (*schemas.AdjustmentResult, error)
}

// ServiceNowConnector abstracts ServiceNow ITSM dispute and incident management.
type ServiceNowConnector interface {
	GetIncident(ctx context.Context, incidentID string) (*schemas.ServiceNowTicketDetails, error)
	AppendWorkNotes(ctx context.Context, incidentID, notes string) error
	ResolveDispute(ctx context.Context, incidentID, resolutionCode string) error
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
