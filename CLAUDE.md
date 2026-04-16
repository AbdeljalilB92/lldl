# CLAUDE.md

This file provides guidance to Claude Code when working with the llcd (LinkedIn Learning Downloader) repository.

## Project Overview

LinkedIn Learning course downloader. Go backend + Wails GUI (coming). Downloads videos, subtitles, and exercise files from LinkedIn Learning using authenticated API access and HTML scraping for exercise file URL resolution.

## Build & Development Commands

```bash
go build -o llcd .                       # Build binary
go run .                                 # Run directly
go test ./...                            # Run all tests
go test ./features/auth/...              # Run tests for specific package
go vet ./...                             # Static analysis
~/go/bin/golangci-lint run ./...         # Linter
gofmt -l .                               # Check formatting (list unformatted)
gofmt -w .                               # Format all files in-place
./scripts/check-all.sh                   # Run ALL quality gates (format + vet + lint + build + test)
```

## Validation Gate (must pass before declaring done)

```bash
./scripts/check-all.sh
```

This runs 5 checks in order:
1. **gofmt** — no unformatted files
2. **go vet** — static analysis, zero warnings
3. **golangci-lint** — 17 linters (see `.golangci.yml`), zero issues
4. **go build** — compilation succeeds
5. **go test** — all tests pass

Zero warnings. Zero failures. No exceptions.

**MANDATORY: Every agent (main, subagent, or spawned worker) MUST:**
1. Read this CLAUDE.md before writing any code
2. Run `./scripts/check-all.sh` before claiming work is done
3. Fix any gate failure before reporting completion

Delivering code that hasn't passed all gates is not acceptable.

## Architecture

### Target Directory Structure (FSD for Go)

```
llcd/
├── main.go                          # Entrypoint — calls app.Run()
├── app/
│   ├── app.go                       # Application orchestrator — wires all dependencies
│   └── wire.go                      # Dependency injection wiring (constructor-based)
│
├── features/                        # Feature modules — NO cross-feature imports
│   ├── auth/
│   │   ├── provider.go              # AuthProvider interface
│   │   ├── linkedin.go              # LinkedIn token validation, CSRF, enterprise hash
│   │   ├── types.go                 # AuthResult, TokenInfo, CSRFToken types
│   │   └── provider_test.go
│   │
│   ├── course/
│   │   ├── fetcher.go               # CourseFetcher interface
│   │   ├── linkedin.go              # LinkedIn API course fetcher implementation
│   │   ├── parser.go                # Course structure parsing (chapters, videos)
│   │   ├── types.go                 # Course, Chapter, Video, TranscriptLine
│   │   ├── api_types.go             # Typed LinkedIn API response structs
│   │   └── fetcher_test.go
│   │
│   ├── video/
│   │   ├── resolver.go              # VideoURLResolver interface
│   │   ├── linkedin.go              # Video stream URL resolution + transcript extraction
│   │   ├── types.go                 # VideoResult, StreamInfo, TranscriptResult
│   │   └── resolver_test.go
│   │
│   ├── exercise/
│   │   ├── resolver.go              # ExerciseFileResolver interface
│   │   ├── linkedin.go              # BPR HTML scraping, ambry URL extraction
│   │   ├── types.go                 # ExerciseFile, ExerciseFileResult
│   │   └── resolver_test.go
│   │
│   ├── download/
│   │   ├── engine.go                # DownloadEngine interface
│   │   ├── concurrent.go            # Concurrent download implementation (worker pool)
│   │   ├── types.go                 # DownloadJob, DownloadResult, Progress
│   │   └── engine_test.go
│   │
│   ├── config/
│   │   ├── store.go                 # ConfigStore interface
│   │   ├── json_store.go            # JSON file-based config implementation
│   │   ├── types.go                 # Config, ConfigPath
│   │   └── store_test.go
│   │
│   └── ui/
│       ├── presenter.go             # Presenter interface (output abstraction)
│       ├── cli.go                   # CLI presenter (current TUI replacement)
│       └── wails.go                 # Wails GUI presenter (future)
│
├── shared/                          # Cross-cutting — features depend on this, never each other
│   ├── domain/
│   │   └── types.go                 # Shared value objects: Quality, SafeFileName, URL wrappers
│   ├── http/
│   │   ├── client.go                # AuthenticatedClient interface + constructor
│   │   └── retry.go                 # Retry middleware with backoff
│   ├── errors/
│   │   ├── errors.go                # Typed errors: AuthError, ParseError, NetworkError, etc.
│   │   └── errors_test.go
│   ├── logging/
│   │   └── logger.go                # Structured slog setup, log levels
│   └── validation/
│       └── validator.go             # Input validation helpers (URL, token, path)
│
└── lib/                             # Low-level utilities — never imports features/
    ├── sanizite/
    │   └── filename.go              # ToSafeFileName, path sanitization
    └── atomic/
        └── file.go                  # Atomic file write (temp + rename)
```

### Dependency Rules (Strict — Violations Fail Review)

```
app/       →  features/*  →  shared/  →  lib/
                             ↓
                        (Go stdlib)
```

1. **`app/`** imports from `features/*` and `shared/`. Wires dependencies. Contains no business logic.
2. **`features/X/`** imports from `shared/` and `lib/` only. NEVER imports another `features/Y/`.
3. **`shared/`** imports from `lib/` and stdlib only. Defines cross-cutting contracts (interfaces, types, errors).
4. **`lib/`** imports stdlib only. Pure utilities with zero domain knowledge.
5. **`main.go`** imports `app/` only. One line: `app.Run()`.

### Interface Contracts (Defined in Features, Implemented in Features)

Every feature defines its own interface in a `provider.go`/`fetcher.go`/etc. file:

```go
// features/auth/provider.go
type AuthProvider interface {
    ValidateToken(ctx context.Context, token string) (AuthResult, error)
    CSRFToken() CSRFToken
}

// features/course/fetcher.go
type CourseFetcher interface {
    FetchCourse(ctx context.Context, slug string) (*Course, error)
}

// features/video/resolver.go
type VideoURLResolver interface {
    ResolveURL(ctx context.Context, videoSlug string) (VideoResult, error)
    FetchTranscript(ctx context.Context, videoSlug string) (TranscriptResult, error)
}

// features/exercise/resolver.go
type ExerciseFileResolver interface {
    ResolveURLs(ctx context.Context, courseSlug, videoSlug string) ([]ExerciseFile, error)
}

// features/download/engine.go
type DownloadEngine interface {
    Download(ctx context.Context, jobs []DownloadJob) ([]DownloadResult, error)
}

// features/config/store.go
type ConfigStore interface {
    Load() (*Config, error)
    Save(config *Config) error
}

// features/ui/presenter.go
type Presenter interface {
    ShowInfo(msg string)
    ShowError(msg string)
    ShowSuccess(msg string)
    PromptString(label string) (string, error)
    PromptPassword(label string) (string, error)
    PromptQuality() (Quality, error)
    ShowProgress(jobID string, progress Progress)
}
```

### Dependency Injection (Constructor Only — No Setters)

```go
// BAD — setter injection
func NewDownloader(dir string) *Downloader { ... }
func (d *Downloader) SetAuthClient(client *http.Client, token string) { ... }

// GOOD — constructor injection
func NewConcurrentDownloader(dir string, client AuthenticatedClient, opts DownloadOpts) DownloadEngine {
    return &concurrentDownloader{
        dir:    dir,
        client: client,
        opts:   opts,
    }
}
```

All dependencies received at construction. No post-hoc mutation. No `Set*` methods.

## Code Standards

### Go (Strict)

- `go vet` clean — no exceptions
- No `panic()` in library/business code — only in truly unrecoverable init scenarios
- No `interface{}` / `any` for JSON deserialization — always use typed structs
- No global mutable state — use dependency injection
- No `init()` functions — use explicit constructor functions
- No unexported package-level mutable variables (only consts and pure function vars)

### SOLID (Strict)

- **S — Single Responsibility**: One reason to change per file. Max 300 lines per file — split if exceeding. One struct per file, one interface per file.
- **O — Open/Closed**: Extend via composition and interfaces, not modification of existing code.
- **L — Liskov Substitution**: Implementations must satisfy the full interface contract. No partial implementations.
- **I — Interface Segregation**: Small, focused interfaces (2-4 methods max). No god-interfaces. Consumers depend only on what they use.
- **D — Dependency Inversion**: Features define interfaces. `app/` wires concrete implementations. Business logic never imports concrete types from other features.

### SRP Per File (Strict)

Every file has exactly one responsibility:
- One struct + its methods per file
- One interface definition per file
- Types can share a file when they belong to the same domain concept (`types.go`)
- API response types in separate `api_types.go` files

### Comments: The WHY, Not the What

```go
// BAD: Increment counter by 1
count++

// GOOD: Compensate for zero-indexed chapters vs 1-indexed directory names
count++
```

Code shows WHAT. Comments explain WHY. If the logic isn't self-evident, comment the reasoning. If it is, don't comment at all.

### Error Handling (Strict)

No raw `fmt.Errorf` wrapping alone. Use typed errors from `shared/errors/`:

```go
// shared/errors/errors.go
type AuthError struct { Cause error }
type ParseError struct { Source string; Cause error }
type NetworkError struct { URL string; Cause error; Retryable bool }

// Usage:
return Course{}, &errors.ParseError{Source: "course API response", Cause: err}
```

Callers type-switch on error kind. No string matching on error messages.

All functions return `(result, error)`. No panics propagated across package boundaries.

### Typed API Responses (No map[string]interface{})

```go
// BAD
func parseCourse(data map[string]interface{}) *Course { ... }

// GOOD — define the exact API response shape
type coursesAPIResponse struct {
    Elements []courseElement `json:"elements"`
    Paging   pagingInfo      `json:"paging"`
}

type courseElement struct {
    Title    string        `json:"title"`
    Slug     string        `json:"slug"`
    Chapters []chapterRef  `json:"chapters"`
}

// Parse directly into typed struct
var resp coursesAPIResponse
if err := json.NewDecoder(body).Decode(&resp); err != nil { ... }
```

Every external API response gets its own typed struct in `api_types.go`. No `map[string]interface{}` anywhere.

### No Module-Level Side Effects

Never create HTTP clients, open files, or initialize state at package level. Use constructor functions:

```go
// BAD
var defaultClient = &http.Client{Timeout: 30 * time.Second}

// GOOD
func NewAuthenticatedClient(baseURL string, token string) AuthenticatedClient {
    return &authenticatedClient{
        client: &http.Client{Timeout: 30 * time.Second},
        token:  token,
    }
}
```

### Logging

Structured logging via `shared/logging/` — wraps `slog`. Never `fmt.Println` or `log.Println`:

```go
import "llcd/shared/logging"

logger := logging.New("[Auth][ValidateToken]")
logger.Info("token validated", "csrf", csrfToken, "enterprise", hasEnterprise)
```

### HTTP Client Abstraction

All HTTP calls go through `shared/http/client.go`. Never create `*http.Client` directly in feature code:

```go
type AuthenticatedClient interface {
    Get(ctx context.Context, url string) (*http.Response, error)
    GetWithRetry(ctx context.Context, url string, maxRetries int) (*http.Response, error)
}
```

### Security (Zero Trust)

- All input validated at feature boundaries (URL format, token format, file paths)
- No string interpolation into URLs — always use `url.URL` construction
- Auth tokens never logged or included in error messages
- Config files use atomic write with 0600 permissions
- File paths sanitized through `lib/sanitize/` before any filesystem operation
- No trust on downloaded filenames — always sanitize

### Concurrency

- Worker pool size configurable, never hardcoded
- Context propagation to all goroutines — no orphan goroutines
- Progress reporting through channels, not shared mutable state
- Graceful shutdown on context cancellation

## Feature-Sliced Design (FSD) — Go Adaptation

```
app/  →  features/*  →  shared/  →  lib/
```

1. **No cross-feature imports** — `features/auth/` NEVER imports from `features/course/`. Shared needs → `shared/`.
2. **Interface = Public API** — Each feature's interface file defines its contract. Consumers depend on the interface, not the implementation.
3. **Shared owns cross-cutting contracts** — types, errors, HTTP abstractions used by 2+ features.
4. **`lib/` never imports from `features/` or `shared/`** — pure utilities only.
5. **`app/` is the only place that knows about concrete types** — it wires implementations to interfaces.

## Testing

### Framework

- Standard `testing` package + `testify` for assertions (if added). No external test frameworks beyond this.
- Table-driven tests for all functions with multiple input cases.

### File Placement

- Co-located test files: `features/auth/provider_test.go` tests `features/auth/provider.go`.
- Test files for `features/X/` only import from `features/X/` internals and `shared/` — same FSD rule as production code.
- Integration tests in `tests/integration/` at project root.

### What to Test

1. **Pure functions first** — parsing, URL construction, filename sanitization, type conversions. Highest ROI, zero mocking.
2. **Interface compliance** — `var _ AuthProvider = (*linkedinAuth)(nil)` compile-time checks.
3. **Error paths** — every typed error returned under correct conditions.
4. **Mock at boundaries** — mock HTTP responses, mock filesystem. Never mock internal functions.
5. **No table-driven tests without meaningful assertions** — test behavior, not just "doesn't crash".

### Priority Order

1. `shared/` — domain types, errors, validation
2. `lib/` — pure utilities (filename sanitization, atomic write)
3. `features/*/parser.go` — parsing logic (pure functions, easy to test)
4. `features/*/linkedin.go` — API interaction (mock HTTP)
5. `features/download/` — concurrent engine (integration tests)
6. `app/` — wiring (smoke tests)

### Test Rules

- Never implement a feature without writing its tests in the same pass.
- Every exported function gets at least one test.
- Every error path gets a test.
- AI-generated tests are untrusted — review that assertions are meaningful.
- No tests that only check `err == nil` without verifying the result.

## AI Coding Discipline

- **Simplicity first.** Before proposing any solution, ask: "Is there a simpler way?"
- **Minimal impact.** Only touch what's necessary. A bug fix doesn't need surrounding cleanup.
- **No temporary fixes.** Find root causes. No hacky workarounds.
- **Search before creating.** Search for existing implementations before writing new code.
- **Plan before coding** on any task with 3+ steps. If something goes wrong, STOP and re-plan.
- **Never accept code you can't explain.** Complex logic requires a WHY comment.
- **Small, focused changes.** One concern per commit.
- **Autonomous bug fixing.** When given a bug: just fix it. Zero hand-holding required.
- **Treat all generated code as untrusted** — review for security at boundaries.

## Wails GUI Integration (Post-Refactor)

After the refactoring is complete and all tests pass, a Wails GUI will be added:
- `frontend/` — React/Svelte/TypeScript frontend (Wails standard layout)
- `features/ui/wails.go` — Wails presenter implementation
- `app/wails_app.go` — Wails application wrapper
- Features remain identical — only the `Presenter` implementation changes
- All business logic stays in Go. Frontend is pure presentation.

The refactoring MUST produce a clean interface boundary so the GUI swaps in without touching any feature code.

## Git Workflow

- `feat/feature-name` — New features
- `fix/bug-name` — Bug fixes
- `refactor/scope-name` — Refactoring

## Migration Notes (Current → Target)

The current codebase has these issues that MUST be resolved during refactoring:
- `Extractor` god object (~500 lines) → split into auth, course, video, exercise features
- `map[string]interface{}` JSON parsing → typed structs in `api_types.go`
- Zero interfaces → interfaces in every feature
- Setter injection (`SetAuthClient`) → constructor injection only
- `panic()` for URL parsing → proper error returns
- `any`/`interface{}` → typed structs everywhere
- Hardcoded paths and constants → configurable
- No tests → full test coverage
- Empty `subtitle/` package → remove or implement
- Duplicate `userAgent` constant → shared constant
