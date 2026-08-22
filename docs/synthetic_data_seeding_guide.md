# Synthetic Data Generation & Live Multi-System Seeding Runbook

This guide details the complete specification, data model design, and operational procedures for generating mathematically correlated reconciliation datasets and **programmatically loading and validating live records in ServiceNow Developer Instances and Salesforce Developer Orgs**.

---

## 1. Executive Philosophy: Why Real Environment Seeding Matters

An enterprise data reconciliation agent cannot be proven with static mock JSON. To demonstrate true agentic value:
1. **Live SOQL / REST Execution**: The `SalesforceWorker` must query authentic Salesforce objects (`Opportunity`, `Contract`, `Account`, `BillingSchedule`) across live REST API endpoints.
2. **Live Table API Execution**: The `ServiceNowWorker` must query live ServiceNow REST endpoints (`/api/now/table/incident`) with real HTTP Basic/OAuth authentication.
3. **Correlated Enterprise Relational Graph**: Records across Salesforce and ServiceNow must share consistent business keys (`contract_id`, `correlation_id`) with realistic variances.

---

## 2. Developer Instance Schema & Field Mapping

To ensure compatibility with standard **Salesforce Developer Editions** (e.g. Agentforce Dev Orgs) and **ServiceNow Personal Developer Instances (PDIs)** without requiring proprietary pre-installed packages, the system leverages native standard objects and fields with structured metadata:

```mermaid
classDiagram
    class SalesforceOpportunity {
        +String Id
        +String Name "[Acme Corp] - Contract CTR-2026-001"
        +Double Amount "145,000.00"
        +String StageName "Closed Won"
        +Date CloseDate "2026-07-31"
        +String Description "JSON: {correlation_id, contract_id, contract_cap: 130750.00}"
    }

    class ServiceNowIncident {
        +String sys_id
        +String number "INC0010042"
        +String correlation_id "CORR-UUID-771"
        +String short_description "Billing Dispute: Overage on Contract CTR-2026-001"
        +String description "JSON: {disputed_amount: 14250.00, contract_id: CTR-2026-001}"
        +String state "6 (Resolved)"
        +String category "Billing"
    }

    SalesforceOpportunity "1" -- "1" ServiceNowIncident : Correlated via CTR-2026-001 / CORR-UUID-771
```

### 2.1. Salesforce Target Object: `Opportunity` (Standard)
- **Target Object**: `Opportunity`
- **Name**: Formatted as `"<Account_Name> - Contract [CTR-2026-XXXX]"` (Enables immediate text/SOQL search out-of-the-box).
- **Amount**: Billed contract value (e.g. `$145,000.00`).
- **StageName**: `"Closed Won"`.
- **Description**: Stores strict JSON metadata:
  ```json
  {
    "correlation_id": "corr-uuid-771",
    "contract_id": "CTR-2026-001",
    "agreed_cap": 130750.00,
    "variance_expected": -14250.00
  }
  ```

### 2.2. ServiceNow Target Table: `incident` (Standard ITSM)
- **Target Table**: `incident` (Present out-of-the-box in 100% of ServiceNow instances).
- **`correlation_id`**: Standard native ServiceNow field on the `task` table. Stores `corr-uuid-771`.
- **`short_description`**: `"Billing Overcharge Dispute - Contract CTR-2026-001"`.
- **`description`**: Structured JSON payload:
  ```json
  {
    "disputed_amount": 14250.00,
    "currency": "USD",
    "correlation_id": "corr-uuid-771",
    "contract_id": "CTR-2026-001",
    "discrepancy_type": "OVERAGE_CREDIT_APPROVED",
    "resolution_summary": "Credit Memo approved by Finance for $14,250 overage beyond contract cap."
  }
  ```
- **`state`**: `2` (In Progress) for active dispute tickets; `6` (Resolved) upon reconciliation.
- **`category`**: `"inquiry"` or `"billing"`.

### 2.3. ServiceNow Personal Developer Instance (PDI) Setup & API Permissions Guide

When setting up a ServiceNow Personal Developer Instance (PDI) on `https://devXXXXX.service-now.com`, follow these mandatory configuration steps to enable seamless REST Table API access:

#### 1. Initial Password Reset & 1-Time Browser Confirmation
- When an instance is provisioned or its password is reset from [developer.servicenow.com](https://developer.servicenow.com), ServiceNow sets `password_needs_reset = true`.
- **Action Required**: Log in via your web browser at `https://devXXXXX.service-now.com` at least once with the temporary password and complete the initial password confirmation. This clears the setup gate and enables Table API basic authentication.

#### 2. Required Roles for Table API Ingestion (`sys_user`)
In modern ServiceNow releases (Washington DC / Xanadu), inbound REST calls require explicit internal and API execution roles:
1. In the Filter Navigator, navigate to **`sys_user.list`**.
2. Open the **`admin`** user record (or your dedicated integration service user).
3. Under the **Roles** related list at the bottom, click **Edit...** and assign:
   - **`snc_platform_rest_api_access`** / **`snc_internal`**: Grants access to execute platform REST web services.
   - **`snc_basic_api`**: Authorizes Basic Authentication requests against REST endpoints.
   - **`rest_service`**: Authorizes general inbound REST Table API invocations.
   - **`itil`**: Grants read and write permissions to the standard `incident` table.
4. Verify that **`Locked out`** is **Unchecked**, **`Password needs reset`** is **Unchecked**, and **`Active`** is **Checked**.

#### 3. Data Policy Compliance for Incident Creation
- Initial dispute records are ingested with **`state = "2"` (In Progress)** to represent active billing disputes under investigation.
- If inserting in **`state = "6"` (Resolved)**, ServiceNow Data Policies require mandatory **`close_code`** (Resolution code, e.g. `"Solved (Permanently)"`) and **`close_notes`** (e.g. `"Resolved via automated cross-system billing reconciliation."`).

---

## 3. Step 1: Generating the Correlated Synthetic Dataset (`cmd/synth`)

The Go synthetic data generator builds a mathematically correlated relational graph with 4 parameterized variance distributions:

```bash
# Generate 500 correlated enterprise business transactions
go run cmd/synth/main.go \
  --count=500 \
  --variance-dist="40:match,30:timing,20:rounding,10:critical" \
  --output="data/correlated_recon_500.json"
```

### Dataset Structure (`data/correlated_recon_500.json`):
```json
[
  {
    "correlation_id": "corr-uuid-771",
    "contract_id": "CTR-2026-001",
    "account_name": "Acme Global Technologies",
    "variance_archetype": "CRITICAL_DISCREPANCY",
    "salesforce_opportunity": {
      "name": "Acme Global - Contract [CTR-2026-001]",
      "billed_amount": 145000.00,
      "agreed_cap": 130750.00,
      "stage": "Closed Won",
      "close_date": "2026-07-31"
    },
    "servicenow_incident": {
      "short_description": "Billing Dispute - Contract CTR-2026-001",
      "disputed_amount": 14250.00,
      "category": "billing",
      "state": "6"
    }
  }
]
```

---

## 4. Step 2: Programmatically Loading Live Data (`cmd/loader`)

### 4.1. Environment Configuration (`.env.local`)
Create a local `.env.local` file with your credentials:

```bash
# Salesforce Instance (e.g. orgfarm Developer Edition)
SFDC_INSTANCE_URL="https://orgfarm-b2f2a8eb8d-dev-ed.develop.my.salesforce.com"
SFDC_USERNAME="tiaburton.dad9d78120c9@agentforce.com"
SFDC_PASSWORD="your_password_and_token"

# ServiceNow Instance (e.g. dev410998 PDI)
SERVICENOW_INSTANCE_URL="https://dev410998.service-now.com"
SERVICENOW_USERNAME="admin"
SERVICENOW_PASSWORD="your_servicenow_password"
```

### 4.2. Seeding Salesforce Developer Org
```bash
# Load opportunities into Salesforce
go run cmd/loader/main.go \
  --target=salesforce \
  --input="data/correlated_recon_500.json" \
  --batch-size=50
```

### 4.3. Seeding ServiceNow Developer Instance
```bash
# Load dispute incidents into ServiceNow
go run cmd/loader/main.go \
  --target=servicenow \
  --input="data/correlated_recon_500.json" \
  --batch-size=25
```

---

## 5. Step 3: Programmatically Validating Loaded Records (`cmd/verifier`)

To verify that records were loaded programmatically and are queryable by the agent sub-agents:

```bash
# Execute programmatic verification suite
go run cmd/verifier/main.go --input="data/correlated_recon_500.json"
```

### Verification Report:
```text
================================================================================
          LIVE DATA SEEDING VALIDATION REPORT (Salesforce & ServiceNow)         
================================================================================
[Salesforce CRM]
  Endpoint: https://orgfarm-b2f2a8eb8d-dev-ed.develop.my.salesforce.com
  Target Object: Opportunity
  Records Found: 500 / 500 (100% Verified)
  Sample Query: SELECT Id, Name, Amount FROM Opportunity WHERE Name LIKE '%CTR-2026-001%'
  Sample Result: ID: 0065e000002Xz7QAA0 | Amount: $130,750.00 | Status: Closed Won

[ServiceNow ITSM]
  Endpoint: https://dev410998.service-now.com
  Target Table: incident
  Records Found: 500 / 500 (100% Verified)
  Sample Query: /api/now/table/incident?sysparm_query=correlation_id=corr-uuid-771
  Sample Result: Number: INC0010042 | Category: billing | State: 6 (Resolved)

[Multi-System Correlation Health]
  Correlation Key Match Rate: 100.0%
  Variance Ingestion Status: READY FOR AGENTIC RECONCILIATION
================================================================================
```

---

## 6. Step 4: Running Live Agentic Reconciliation

With live data seeded and validated, trigger the Go Multi-Agent Coordinator against your live sandboxes:

```bash
# Reconcile Contract CTR-2026-001 against live Salesforce and ServiceNow APIs
go run cmd/agent/main.go \
  --contract-id="CTR-2026-001" \
  --mode="LIVE"
```
