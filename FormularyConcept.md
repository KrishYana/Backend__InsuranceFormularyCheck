Attached is the structured layout for your **FormularyCheck: Project Architecture & Data Strategy** document. This structure is optimized for conversion into a professional PDF technical specification.

---

# FormularyCheck: Project Architecture & Data Strategy

## Executive Summary
**FormularyCheck** is a mobile application designed for physicians to quickly determine whether a specific medication is covered under a patient's insurance plan, including tier level, prior authorization requirements, step therapy rules, and quantity limits. The app eliminates the need for EMR integration by providing a standalone, physician-friendly interface organized by state and insurance plan.

This document defines the data sources, API integrations, database architecture, and ingestion strategy required to build and maintain the application's backend.

---

## 1. The Data Landscape: Where Formulary Information Lives
The U.S. insurance ecosystem stores formulary data across several distinct layers, each governed by different regulatory frameworks and access mechanisms. Understanding this landscape is essential because no single source provides complete national coverage across all plan types.

### 1.1 Medicare Part D (Federal — CMS Public Use Files)
CMS publishes the **Prescription Drug Plan Formulary, Pharmacy Network, and Pricing Information** as a monthly Public Use File (PUF). This is the single richest publicly available formulary dataset in the country.

* **What it contains (9 separate pipe-delimited flat files):**
    * **Plan Information File:** Provides plan name, contract ID, plan ID, service area, and plan type.
    * **Basic Drugs Formulary File:** Contains NDCs, tier level, and boolean indicators for step therapy, quantity limits, and prior authorization.
    * **Excluded Drugs Formulary File:** Covers enhanced alternative plans covering excluded drugs.
    * **Beneficiary Cost File:** Plan-level cost sharing for preferred, non-preferred, and mail order pharmacies.
* **Access:** Free download from `data.cms.gov`.
* **Key identifiers:** Combination of `CONTRACT_ID` + `PLAN_ID` + `SEGMENT_ID`.
* **Update cadence:** Monthly.
* **Limitations:** Excludes employer-sponsored and PACE plans.

### 1.2 ACA Marketplace / Qualified Health Plans (Federal — Machine-Readable JSON Files)
Under 45 C.F.R. § 156.122(d)(2), all QHP issuers on Federally-Facilitated Exchanges must publish machine-readable JSON files (index.json, plans.json, providers.json, and drugs.json).

* **What drugs.json contains:** RxNorm ID, drug name, `drug_tier` indicator, and a plans array mapping to specific IDs.
* **Discovery:** CMS publishes the **Machine-Readable URL PUF (MR-PUF)**. For PY2026, this is distributed as `machine-readable-url-puf.zip`.
* **Data quality:** A Dec 2025 analysis found 46% of issuers exhibited errors (missing `cost_sharing` or identifier mismatches).
* **Coverage:** FFE, SBE-FP, and State-based Exchanges (which publish through the SBE QHP PUF).

### 1.3 Payer FHIR APIs (DaVinci US Drug Formulary Implementation Guide)
The CMS Interoperability Rules (CMS-9115-F and CMS-0057-F) require regulated payers to implement standardized FHIR APIs via the **DaVinci PDex US Drug Formulary IG**.

* **Profiles:** `PayerInsurancePlan`, `Formulary`, `FormularyItem`, and `FormularyDrug`.
* **Requirement:** Mandatory for Medicare Advantage, Medicaid FFS/Managed Care, CHIP, and QHP issuers on FFEs.
* **Known implementations:** Anthem/Elevance Health (STU2 Release 2.0.0), BCBS of Minnesota.
* **Access pattern:** Decentralized; discovery of specific payer FHIR endpoints is a key challenge.

### 1.4 Surescripts Network
Dominant health information network. Integration requires a contractual relationship and certification.

* **Relevant products:** *Formulary* (comprehensive files), *On-Demand Formulary* (real-time), *Real-Time Prescription Benefit (RTPB)* (patient-specific pricing), and *Eligibility*.
* **Strategic consideration:** Targeted for Phase 2 enhancement due to high barrier to entry.

### 1.5 State Medicaid Formularies
Managed state-by-state via Preferred Drug Lists (PDL). No centralized database exists; data is often managed by PBMs (Magellan, CVS Caremark, OptumRx).

### 1.6 Drug Reference Data (RxNorm — NLM)
The critical linking layer for interoperability.
* **Mappings:** NDC → RxCUI (many-to-one), RxCUI → Class, RxCUI → Related Drugs.
* **Update cadence:** First Monday of each month.
* **Licensing:** UMLS license required for full access.

---

## 2. API Integration Architecture



### 2.1 Architecture Overview
* **Ingestion Pipeline:** Parser for CMS PUF, Crawler for QHP JSON, Client for FHIR, and Sync for RxNorm.
* **Normalization Engine:** NDC to RxCUI resolution, plan ID harmonization, and tier level standardization.
* **Storage:** PostgreSQL (Unified Formulary Database).
* **Delivery:** REST/GraphQL API to Mobile Application.

---

## 3. Database Architecture

### 3.1 Core Schema (PostgreSQL)

```sql
-- REFERENCE DATA: Drugs
CREATE TABLE drugs (
    drug_id             BIGSERIAL PRIMARY KEY,
    rxcui               VARCHAR(20) NOT NULL UNIQUE,
    drug_name           VARCHAR(500) NOT NULL,
    generic_name        VARCHAR(500),
    brand_names         TEXT[],
    dose_form           VARCHAR(100),
    strength            VARCHAR(200),
    route               VARCHAR(100),
    drug_class          VARCHAR(200),
    is_specialty        BOOLEAN DEFAULT FALSE,
    is_controlled       BOOLEAN DEFAULT FALSE,
    dea_schedule        VARCHAR(10),
    last_rxnorm_sync    TIMESTAMP NOT NULL,
    created_at          TIMESTAMP DEFAULT NOW(),
    updated_at          TIMESTAMP DEFAULT NOW()
);

-- NDC-to-Drug mapping
CREATE TABLE drug_ndc_map (
    ndc                 VARCHAR(11) PRIMARY KEY,
    drug_id             BIGINT NOT NULL REFERENCES drugs(drug_id),
    ndc_status          VARCHAR(20) NOT NULL,
    manufacturer        VARCHAR(200),
    package_description VARCHAR(500),
    start_date          DATE,
    end_date            DATE,
    created_at          TIMESTAMP DEFAULT NOW()
);

-- FORMULARY DATA: Drug coverage per plan
CREATE TABLE formulary_entries (
    entry_id            BIGSERIAL PRIMARY KEY,
    plan_id             BIGINT NOT NULL REFERENCES plans(plan_id),
    drug_id             BIGINT NOT NULL REFERENCES drugs(drug_id),
    is_covered          BOOLEAN NOT NULL DEFAULT TRUE,
    tier_level          SMALLINT,
    tier_name           VARCHAR(100),
    prior_auth_required BOOLEAN DEFAULT FALSE,
    step_therapy        BOOLEAN DEFAULT FALSE,
    quantity_limit      BOOLEAN DEFAULT FALSE,
    quantity_limit_detail VARCHAR(500),
    specialty_drug      BOOLEAN DEFAULT FALSE,
    copay_amount        DECIMAL(10,2),
    coinsurance_pct     DECIMAL(5,2),
    source_type         VARCHAR(50) NOT NULL,
    source_date         DATE NOT NULL,
    is_current          BOOLEAN DEFAULT TRUE,
    superseded_by       BIGINT REFERENCES formulary_entries(entry_id),
    created_at          TIMESTAMP DEFAULT NOW()
);
```

### 3.2 Key Design Decisions
* **RxCUI as Primary Key:** Acts as the natural join key across disparate sources (PUF, QHP, FHIR).
* **Source Priority:** 1. Payer FHIR API (Direct), 2. CMS Part D PUF (Official), 3. QHP JSON (Machine-Readable), 4. Manual.
* **Soft Versioning:** `is_current` flag preserves audit history for rollback and tracking.

---

## 4. Data Ingestion Pipeline Design

### 4.1 Processing Steps
1.  **Staging:** Raw data landing.
2.  **Normalization:** Resolve NDCs to RxCUIs; map tier names (e.g., "Preferred Generic" → Tier 1).
3.  **QA Checks:** Flag plans with <100 drugs, tier distribution anomalies, or low NDC resolution rates.

---

## 5. MVP Phasing Strategy

| Phase | Focus | Timeline | Key Deliverables |
| :--- | :--- | :--- | :--- |
| **1** | Medicare Part D | Weeks 1–8 | RxNorm sync, CMS PUF ingestion, MVP Mobile UI. |
| **2** | ACA Marketplace | Weeks 9–16 | QHP Crawler, Error-tolerant parsing for 46% issuer fail rate. |
| **3** | Commercial/Employer | Weeks 17–28 | DaVinci FHIR client, Major Payer (Anthem/UHC) onboarding. |
| **4** | Surescripts | Weeks 29–40 | RTPB, Electronic Prior Auth, Real-time pricing. |
| **5** | Medicaid/Expansion | Ongoing | State PDLs, Regional PBM partnerships. |

---

## 6. Key Technical Risks & Mitigations

* **Data Freshness:** Formularies change monthly/quarterly. *Mitigation:* Display source dates; automate stale data flags (>45 days).
* **QHP Quality:** Half of issuers have errors. *Mitigation:* Fault-tolerant crawler; log errors by issuer; supplemental FHIR data.
* **Plan ID Complexity:** Physicians don't know specific plan IDs. *Mitigation:* Surface plan differences in a summary view (e.g., "Which BCBS plans cover this?").
* **Regulatory:** Licensing for UMLS. *Mitigation:* Secure UMLS license in Phase 1; include mandatory CMS disclaimers.

---

## 7. Reference Links
* **CMS Part D PUF:** `data.cms.gov`
* **Exchange PUFs:** `cms.gov/marketplace/resources/data/public-use-files`
* **DaVinci Formulary IG:** `build.fhir.org/ig/HL7/davinci-pdex-formulary/`
* **RxNorm API:** `lhncbc.nlm.nih.gov/RxNav/APIs/RxNormAPIs.html`