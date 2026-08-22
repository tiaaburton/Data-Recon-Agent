# Figma Design Specification & Custom Asset Guide

This specification defines the visual standards, vector asset guidelines, design tokens, and collaboration points for creating custom UI components in the **A2UI v0.9 Catalog**.

---

## 1. Executive Summary & Design Philosophy

Enterprises demand cohesive, branded agent experiences that move beyond generic conversational bubbles. The **Data Reconciliation Agent** utilizes a custom A2UI catalog rendered natively in Gemini Enterprise Chat and web workspaces.

This guide provides the exact specifications for UI/UX designers to create custom visual assets in **Figma** and export them as optimized SVGs for the Go runtime and A2UI client renderer.

---

## 2. Collaborative Asset Breakdown

| Asset Name | Target Filepath | Dimensions | Usage & Trigger |
| :--- | :--- | :--- | :--- |
| **Explosive Variance Alert Badge** | `assets/figma/badges/explosive_badge_v2.svg` | $32 \times 32\text{ px}$ (viewBox: `0 0 32 32`) | Rendered when financial discrepancy exceeds critical threshold ($> \$5,000$). |
| **Three-Way Diff Check Icon** | `assets/figma/icons/three_way_match.svg` | $24 \times 24\text{ px}$ (viewBox: `0 0 24 24`) | Rendered on reconciled rows matching SAP, SFDC, and ServiceNow. |
| **Signed Mutation Shield** | `assets/figma/badges/signed_shield.svg` | $24 \times 24\text{ px}$ (viewBox: `0 0 24 24`) | Rendered on HITL approval cards with valid Ed25519 signatures. |
| **System Logo: ServiceNow** | `assets/figma/logos/servicenow_badge.svg` | $20 \times 20\text{ px}$ (viewBox: `0 0 20 20`) | Entity source attribution tag. |
| **System Logo: Salesforce** | `assets/figma/logos/salesforce_badge.svg` | $20 \times 20\text{ px}$ (viewBox: `0 0 20 20`) | Entity source attribution tag. |
| **System Logo: SAP S/4HANA** | `assets/figma/logos/sap_badge.svg` | $20 \times 20\text{ px}$ (viewBox: `0 0 20 20`) | Entity source attribution tag. |

---

## 3. Explosive Variance Badge Specification

The **Explosive Variance Badge** is a custom visual indicator designed to immediately signal high-severity financial discrepancies to accounting and operations personnel.

```
       ▲  ★  ▲
     ★ ┌─────┐ ★
   ▲───┤ ! ! ├───▲   <-- Pulsing Explosive Starburst / Spark Burst
     ★ └─────┘ ★
       ▼  ★  ▼
```

### 3.1. Design Tokens & Palette

| Token | Hex Code | Purpose |
| :--- | :--- | :--- |
| `--recon-badge-alert-red` | `#EA4335` | Core starburst fill & alert background |
| `--recon-badge-pulse-glow` | `#FF6D00` | Outer glow radial gradient & particle ring |
| `--recon-badge-core-white` | `#FFFFFF` | Center exclamation icon & inner accent |
| `--recon-badge-shadow` | `rgba(234, 67, 53, 0.4)` | Dynamic drop shadow ($0\text{px } 4\text{px } 12\text{px}$) |

### 3.2. Figma Layer Hierarchy & Export Rules

1. **Frame Setup**:
   - Dimensions: $32 \times 32\text{ px}$ (Fixed).
   - Clip Content: `OFF` (to preserve outer particle burst glow).
2. **Layer Tree**:
   - `Layer 1: Outer_Burst_Glow` (Ellipse, Gaussian Blur $4\text{px}$, Opacity $60\%$)
   - `Layer 2: Starburst_Base` (8-point or 12-point jagged star path, Fill: Linear Gradient `#EA4335` $\to$ `#C5221F`)
   - `Layer 3: Inner_Spark_Particles` (4 directional ray sparks, Fill: `#FBBC04`)
   - `Layer 4: Icon_Exclamation` (Vector path, Fill: `#FFFFFF`)
3. **SVG Export Settings**:
   - Format: `SVG`.
   - Include `id` metadata: `YES`.
   - Outline text: `YES`.
   - Precision: `2 decimals`.

### 3.3. Starter SVG Reference Implementation

```xml
<svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" id="explosive-badge-v2">
  <defs>
    <radialGradient id="burst-glow" cx="50%" cy="50%" r="50%">
      <stop offset="0%" stop-color="#FF6D00" stop-opacity="0.8"/>
      <stop offset="100%" stop-color="#EA4335" stop-opacity="0"/>
    </radialGradient>
    <linearGradient id="star-fill" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
      <stop stop-color="#EA4335"/>
      <stop offset="1" stop-color="#C5221F"/>
    </linearGradient>
  </defs>
  <!-- Outer Glow -->
  <circle cx="16" cy="16" r="14" fill="url(#burst-glow)"/>
  <!-- Starburst Geometry -->
  <path d="M16 2L19.5 8.5L26.5 6.5L25 13.5L31.5 16L25 18.5L26.5 25.5L19.5 23.5L16 30L12.5 23.5L5.5 25.5L7 18.5L0.5 16L7 13.5L5.5 6.5L12.5 8.5L16 2Z" fill="url(#star-fill)" stroke="#FFA8A4" stroke-width="1"/>
  <!-- Particle Sparks -->
  <circle cx="6" cy="6" r="1.5" fill="#FBBC04"/>
  <circle cx="26" cy="6" r="1.5" fill="#FBBC04"/>
  <circle cx="6" cy="26" r="1.5" fill="#FBBC04"/>
  <circle cx="26" cy="26" r="1.5" fill="#FBBC04"/>
  <!-- Inner Exclamation Mark -->
  <path d="M14.75 9H17.25L16.75 18H15.25L14.75 9ZM14.5 21C14.5 20.17 15.17 19.5 16 19.5C16.83 19.5 17.5 20.17 17.5 21C17.5 21.83 16.83 22.5 16 22.5C15.17 22.5 14.5 21.83 14.5 21Z" fill="#FFFFFF"/>
</svg>
```

---

## 4. Custom Card Layouts in Figma

### 4.1. Three-Way Diff Matrix Card (`360px` Mobile / `680px` Desktop)

```
┌──────────────────────────────────────────────────────────────────┐
│ [★ Explosive Badge] CRITICAL VARIANCE DETECTED: -$14,250.00      │
│ SAP Invoice: #INV-2026-9081 │ SFDC Opp: #OPP-8821 │ SN: #INC-4412│
├──────────────────────────────────────────────────────────────────┤
│ FIELD            │ SAP S/4HANA     │ SALESFORCE      │ SERVICENOW│
├──────────────────┼─────────────────┼─────────────────┼───────────┤
│ Net Amount       │ $145,000.00     │ $130,750.00 ⚠️  │ $145,000  │
│ Tax / Currency   │ USD 8.25%       │ USD 0.00% ⚠️    │ USD 8.25% │
│ Status           │ POSTED          │ CLOSED WON      │ RESOLVED  │
└──────────────────────────────────────────────────────────────────┘
```

### 4.2. HITL Cryptographic Authorization Card

```
┌──────────────────────────────────────────────────────────────────┐
│ 🛡️ HITL ACTION REQUIRED: SAP S/4HANA Mutation                     │
│ Target: Adjust Invoice #INV-2026-9081 by +$14,250.00             │
├──────────────────────────────────────────────────────────────────┤
│ Signature: 9f82ab4... (Ed25519 Valid) │ Expires in: 04:59        │
├──────────────────────────────────────────────────────────────────┤
│ [  APPROVE & EXECUTE MUTATION  ]    [  REJECT / ESCALATE  ]      │
└──────────────────────────────────────────────────────────────────┘
```

---

## 5. Asset Directory Layout

Place all exported SVG assets in the project structure as follows:

```
Data-Recon-Agent/
└── assets/
    └── figma/
        ├── badges/
        │   ├── explosive_badge_v2.svg
        │   └── signed_shield.svg
        ├── icons/
        │   ├── three_way_match.svg
        │   ├── alert_warning.svg
        │   └── check_verified.svg
        ├── logos/
        │   ├── sap_badge.svg
        │   ├── salesforce_badge.svg
        │   └── servicenow_badge.svg
        └── cards/
            ├── diff_matrix_preview.png
            └── hitl_action_preview.png
```

---

## 6. How the Go Runtime Serves Custom Assets

The Go HTTP server embeds assets directly using `embed.FS` and serves them to the Gemini A2UI renderer:

```go
package assets

import (
	"embed"
	"net/http"
)

//go:embed figma/*
var FigmaAssets embed.FS

func RegisterAssetRoutes(mux *http.ServeMux) {
	mux.Handle("/assets/figma/", http.StripPrefix("/assets/figma/", http.FileServer(http.FS(FigmaAssets))))
}
```
