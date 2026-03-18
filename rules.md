# Go and HTMX: Project Architecture and Technical Standards

## Core Architecture
This project follows the standard Go project layout to ensure clear separation between the transport layer (HTTP/HTMX) and the domain logic (Ranked Choice Voting).

## 1. Project Directory Structure

.
├── cmd/
│   └── server/
│       └── main.go           # Entry point: dependency injection & server start
├── internal/                 # Private packages (inaccessible to external modules)
│   ├── voting/               # Domain Logic: RCV Tabulation algorithm
│   ├── models/               # Data Models: Shared structs
│   ├── database/             # Persistence: DB Connection & Repository patterns
│   └── handlers/             # Transport: HTTP/HTMX Handlers (Controllers)
├── ui/                       # Frontend Assets
│   ├── html/                 # Go Templates (.html or .tmpl)
│   └── static/               # Static assets (CSS, JS, HTMX source)
├── db/
│   └── migrations/           # SQL Migration files
├── go.mod                    # Module definition
└── go.sum                    # Dependency checksums

## 2. Dependency Management
* Go Modules: All internal imports must be prefixed with the module name defined in go.mod.
* Standard Library: Favor the Go standard library (net/http, html/template) where possible to minimize external dependencies.

## 3. Development Standards

### Backend Logic
* Package Isolation: Business logic (e.g., the tabulate algorithm) must reside in internal/voting and remain agnostic of the HTTP layer.
* Exported Identifiers: Structs and functions required across package boundaries must be capitalized (e.g., models.Ballot, voting.Tabulate).
* Error Handling: Errors must be treated as values. Handlers should catch domain errors and return appropriate HTML error fragments for HTMX to swap.

### HTMX & Templating
* Fragment-Driven Design: Use html/template blocks to define components. Handlers should be capable of returning either a full page or a partial fragment based on the presence of the HX-Request header.
* State Management: Application state is managed server-side. The UI reflects the current state through server-driven hypermedia swaps.
* Efficiency: Use hx-boost for full-page transitions and hx-indicator for long-running calculations (like RCV tabulation rounds).

## 4. Implementation Checklist
1. Module Scope: All local package imports must follow the pattern <module-name>/internal/<package>.
2. Type Safety: Use domain-specific types for IDs (e.g., type CandidateID int) to ensure type safety across the voting logic.
3. Template Embedding: Use //go:embed in production-ready versions to bundle the ui/ directory into the application binary.
4. Security: Implement CSRF protection for all non-GET requests utilizing HTMX.
