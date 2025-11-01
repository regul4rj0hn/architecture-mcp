# Product Requirement Document (PRD)

## MCP for Architecture Guidelines, Patterns & ADRs 

**Product Name:** Model Context Protocol (MCP) Service for Architecture Guidelines, Patterns & ADRs 
**Goal:** Implement a simple and efficient service to translate human-readable architectural guidelines, patterns, and Architecture Decision Records (ADRs) in Markdown, that are version-controlled on a Git repository into structured, consumable context for internal AI agents and applications, adhering to the principles of the Model Context Protocol (MCP) standard.

-----

## 1\. Context and Motivation

Internal AI agents (like coding assistants or code reviewers) require up-to-date architectural rules and decisions to function correctly. This information will be created and maintained in human-readable Markdown files (ADRs and Guidelines) on GitHub. We need a simple service that acts as the authoritative source, serving this content directly from the version-controlled repository in a format optimized for AI consumption, without requiring an intermediate database or complex conversion pipeline.

### 1.1 Main Use Case

- As a software engineer I must be able configure and include the MCP Architecture Service on my IDE before I start doing Spec Driven Development with assisted by an AI Agent.

- When I start planning a new service or feature, the AI Agent must consult the MCP Architecture service for available Guidelines and Design Patterns

- Using the initial steering list from the MCP and the developer's desired design input, the AI Agent can choose to call another endpoint on the MCP service and retrieve a specific guideline or pattern that applies to the design

- Upon review or planning, the AI Agent can pull ADRs on previous decisions about specific areas or technologies

-----

## 2\. Solution and Architectural Choice

### 2.1 Implementation Stack

| Component | Selection | Justification |
| :--- | :--- | :--- |
| **Language** | **Golang** | Preferred for network services: high concurrency, robust standard library, excellent performance for I/O tasks, and lightweight static binaries suitable for Alpine Linux containers. |
| **Interface** | **REST** | Simple, stateless, and widely compatible, minimizing client-side complexity for this garage PoC. |
| **Context Source** | **GitHub Repository (Markdown)** | Direct requirement: Single source of truth for version control, human-readability, and easy editing. |
| **OS/Runtime** | **Docker Alpine Linux** | Aligns with existing server pod environment. |

### 2.2 Core Logic - Markdown to Context

The service's core function is to read a Markdown file from the local Git clone and convert its structure into a simple, parsable context format. This honors the request to use Markdown directly while providing a structure for the consuming AI agent.

-----

## 3\. Functional Requirements (FR)

### 3.1 Source Control and Ingestion

  * **FR-1.1: Git Repository Source:** The service **must** run alongside a specified internal GitHub repository and branch.
  * **FR-1.2: Human-Editable Source:** All source documents **must** be standard Markdown (`.md`) files (e.g., `docs/adrs/ADR-001.md`, `policies/style-guide.md`).
  * **FR-1.3: Refresh Mechanism:** The service **must** implement a mechanism (e.g., a simple API endpoint or a lightweight background cron job) to trigger a `git pull` and update the local file cache without a service restart.
      * *Proposed Endpoint:* `POST /api/v1/context/refresh`

### 3.2 Model Context Retrieval (REST API)

  * **FR-2.1: Guideline Retrieval:** The service **must** expose a REST endpoint to retrieve a specific architectural guideline or policy document.
      * *Proposed Endpoint:* `GET /api/v1/context/guideline/{path}` (e.g., `/api/v1/context/guideline/policies/style-guide`).
  * **FR-2.2: ADR Retrieval:** The service **must** expose a dedicated REST endpoint for retrieving a specific Architecture Decision Record.
      * *Proposed Endpoint:* `GET /api/v1/context/adr/{adr_id}` (e.g., `/api/v1/context/adr/ADR-001`).
  * **FR-2.3: Context Format:** The response **must** contain the following simplified structured context derived from the Markdown content:
      * **Goal:** Provide the raw text (the simplest option) for the AI agent to interpret directly, or a structured output based on Markdown headers.
      * *Option A (Simplest PoC):* Return the **raw Markdown text** of the file with a `200 OK`.
      * *Option B (Preferred Structure):* Return a simple JSON structure where the top-level Markdown headers (`#`, `##`) are keys, and their content is the value.

> **Example (Option B JSON Structure):**
>
> ```json
> {
>   "document_title": "Architecture Guideline A",
>   "sections": [
>     {
>       "heading": "Introduction",
>       "level": 1,
>       "content": "Raw markdown content under this header..."
>     },
>     {
>       "heading": "Rule 1: Concurrency Model",
>       "level": 2,
>       "content": "Raw markdown content for the specific rule..."
>     }
>     // ...
>   ]
> }
> ```

-----

## 4\. Non-Functional Requirements (NFR)

  * **NFR-4.1: Performance:** The service must serve content quickly. Given the small scope, the maximum latency for document retrieval must be under **500 ms** (P95).
  * **NFR-4.2: Security (PoC Scope):**
      * Communication with the GitHub repository **must** use a read-only access token stored securely as an environment variable (Alpine-friendly `ENV`).
      * The service **must not** expose any file system access or write operations.
  * **NFR-4.3: Resilience:** On startup, if the initial Git clone fails, the service should log the error and enter a retry loop or remain in a failing state.
  * **NFR-4.4: Observability:** Standard container logging (stdout/stderr) for errors, startup, and refresh operations is required.

-----

## 5\. Success Criteria (PoC)

1.  A new Markdown file pushed to the GitHub repository is retrievable by the service's API within the refresh window.
2.  The internal AI agent can successfully call the REST API and parse the returned context for a specific policy (e.g., *“Retrieve the section titled 'Golang Concurrency Rules' from the `style-guide.md` document”*).
3.  The service maintains low resource utilization suitable for a lightweight Alpine-based container.
