# mgtt-provider-kubernetes Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## ⛔ GATE — DO NOT START UNTIL

This plan was drafted before the decision to lift cross-cutting concerns into mgtt core. Several phases below duplicate work that the core SDK (`github.com/mgt-tool/mgtt/sdk/provider`, shipping in mgtt v0.0.7+) now provides. Starting this plan as written creates duplicate infrastructure that must then be retired — painful, wasted work.

**Prerequisites before executing this plan:**

1. mgtt core plan `../../../../mgtt/docs/superpowers/plans/2026-04-15-mgtt-core-hardening.md` must be executed through **Phase 7.1b (SDK importability release gate)** and tagged as mgtt **v0.0.7 or later**.
2. The scratch-directory `go get github.com/mgt-tool/mgtt/sdk/provider@v0.0.7` import test must pass.
3. This plan must be edited to apply the revisions summarized below. **Do not skip step 3 — the deletions below are mandatory, not optional.**

### Deletions (apply before starting)

After core SDK lands, delete or heavily reduce these phases. Marked **[DEPRECATED — SDK]** inline below.

- **Phase 1 Task 1.1** (`internal/kubeclient/client.go` — ~300 lines of typed errors, size cap, injectable exec) → **DELETE**. Use `sdk/provider/shell.Client` with a kubectl-specific `Classify` function (~30 lines).
- **Phase 1 Task 1.2** (`internal/kubeclient/cache.go` — memoization layer) → **REPLACE** with a 50-line cache that imports `shell.Client`. Add cache hit/miss Debugf per M2.
- **Phase 1 Task 1.3** (Timeout wrapper) → **DELETE**. SDK handles it.
- **Phase 1 Task 1.4** (`internal/probes/registry.go`) → **DELETE**. Use `provider.NewRegistry()` directly.
- **Phase 1 Task 1.5** (helpers) → **KEEP** — these are k8s-specific JSON shape parsers that don't belong in the SDK.
- **Phase 1 Task 1.6–1.7** (deployment, ingress probes migrated to registry) → **KEEP but simplify**: register into `provider.Registry` instead of local type.
- **Phase 1 Task 1.8** (main.go rewrite) → **REPLACE** with a 15-line `main.go` that calls `provider.Main(registry)`.
- **Phase 1 Task 1.9** (parity test) → **DELETE**. Core's `mgtt provider validate --live` replaces this in the provider's own CI step.
- **Phase 2** (Robustness Layer — kubectl presence check, debug logging) → **DELETE**. SDK shell.Client handles kubectl-absent via `exec.ErrNotFound → ErrEnv`. Debug logging piggybacks on core's `MGTT_DEBUG=1` tracer. The only piece to keep is a kubectl-specific `Classify` that recognizes "NotFound", "Forbidden", "Unable to connect" — ~15 lines in a new `internal/kubeclassify/kubeclassify.go`.
- **Phase 3 Task 3.2** (doctor subcommand) → **DELETE**. Core's `mgtt provider validate [--live]` replaces it.
- **Phase 3 Task 3.1** (version subcommand) → **DELETE**. SDK provides `version` automatically via `provider.Main`.

### Type-coverage triage (S4)

The "all 37 types" goal is not realistic for this cycle. Triage before Phase 4:

- **Tier 1 (must-implement in this plan — ~10 types):** deployment, statefulset, daemonset, pod, service, endpoints, ingress, pvc, node, hpa.
- **Tier 2 (nice-to-have — triage after Tier 1 ships):** namespace, configmap, secret, job, cronjob, pdb, networkpolicy, serviceaccount, role, rolebinding.
- **Tier 3 (deferred indefinitely — low troubleshooting signal):** webhooks (validating/mutating), CSI (csidriver, volumeattachment), lease, priorityclass, custom_resource, customresourcedefinition, operator.

Implement Tier 1 in this plan's Phase 4. Move Tier 2 to a follow-up plan. Tier 3 entries stay in the vocabulary YAML but get permanent exempt entries in `mgtt provider validate` output, linking to this triage rationale in the provider's README.

### Additions

- **Cache observability (M2):** the slimmed-down caching layer must log hit/miss to stderr when `MGTT_PROVIDER_DEBUG=1`. Operators debugging "why is this slow?" need visibility.

### Keep unchanged

- Phase 0 (CI, VERSION, probe contract doc — the probe contract doc becomes a pointer to core's PROBE_PROTOCOL.md).
- Phase 10 (install hook hardening).
- Phase 11 (release automation).
- Phase 12 (final sweep — but the parity assertion is replaced with `mgtt provider validate --live` as the CI check).

---


**Goal:** Bring `mgtt-provider-kubernetes` from a 2-type prototype runner to a production-grade provider — table-driven architecture, robust subprocess handling, broad type coverage, caching, diagnostics, CI, and release automation.

**Architecture:** Retain the `kubectl`-shelling-out model (no client-go dependency for v0 — the external binary plugin contract is unchanged). Introduce a table-driven probe registry so adding a type is a one-file change. Layer robustness (timeouts, error taxonomy, size caps, stderr capture, caching) underneath a new `kubeclient` helper package. Add a parity test that forces runner and vocabulary to stay in sync.

**Tech Stack:** Go 1.25, `kubectl`, `kind` for integration tests, GitHub Actions for CI, standard library only (no new runtime deps).

---

## Scope & Non-Goals

**In scope:**
- Table-driven probe dispatch refactor
- Robustness (timeouts, errors, kubectl presence, stderr, caching, size caps, debug logging)
- Runner coverage for all 37 vocabulary types (grouped)
- Vocabulary↔runner parity test
- Install-hook hardening
- CI (lint, unit, integration on kind)
- Observability (`version`, `doctor` subcommands, JSON logs)
- Release automation (tagged releases, CHANGELOG)
- Docs: probe contract, operator troubleshooting

**Explicit non-goals (YAGNI):**
- Switching from `kubectl` to `client-go` (deferred — would double binary size, complicate auth)
- Writing-path RBAC checks (provider is read-only)
- Metrics/Prometheus export (not in v0 plugin contract)
- Watch-based probes (kubectl `get` is sufficient for probe semantics)
- Cross-cluster federation
- Parallel probe execution within a single invocation (engine already parallelizes across invocations)

---

## Decision Log (locked before planning)

| Decision | Chosen | Rejected | Why |
|---|---|---|---|
| Stay on `kubectl` vs. move to `client-go` | `kubectl` | `client-go` | Keeps binary <5MB, preserves read-only-by-construction property, matches plugin contract |
| Error model | Typed sentinel errors (`ErrNotFound`, `ErrForbidden`, `ErrTransient`) exported from `internal/kubeclient` | JSON error codes | Go-idiomatic; callers use `errors.Is` |
| Missing resource behavior | Return `value: null` + `raw: ""` with exit 0 and `"status": "not_found"` | Exit 1 | Design-time simulations must tolerate missing resources; engine distinguishes via the status field |
| Caching | In-process memoization keyed on full kubectl argv, process lifetime only | Disk cache | A single `probe` invocation may read N facts of one resource; disk cache is engine's job |
| Type file split | One file per type under `internal/probes/<type>.go` + a registry in `internal/probes/registry.go` | One giant file | Files that change together live together; each type self-contained |
| Layout | Move runner code into `internal/` packages; `main.go` becomes thin entrypoint | Keep flat | With 37 types + helpers, flat layout becomes unreviewable |
| Test strategy | TDD per probe, table-driven tests, integration on kind with fixtures | Mock kubectl | Integration tests already catch real-cluster behavior (commit `75b49fc`) |
| CI | GitHub Actions: lint + unit always, integration on PR to main only | Always run integration | kind spinup is ~90s; keep PR loop fast |

---

## File Structure

```
mgtt-provider-kubernetes/
  main.go                              # thin entrypoint (parse args, dispatch to internal/probes)
  main_test.go                         # kept: helper tests move to internal/kubeclient
  internal/
    kubeclient/
      client.go                        # kubectl wrapper: timeout, stderr, size cap, error taxonomy
      client_test.go
      cache.go                         # per-process memoization of kubectl argv → JSON
      cache_test.go
      debug.go                         # debug logging, honors MGTT_PROVIDER_DEBUG
    probes/
      registry.go                      # type string → ProbeSet; fact string → ProbeFn
      registry_test.go
      parity_test.go                   # asserts every fact declared in types/*.yaml has a ProbeFn
      helpers.go                       # shared: jsonInt, jsonBool, jsonString, countList, conditionTrue, etc.
      helpers_test.go
      deployment.go                    # probes for deployment (existing behavior moved here)
      deployment_test.go
      ingress.go
      ingress_test.go
      pod.go
      pod_test.go
      statefulset.go
      statefulset_test.go
      daemonset.go                     # + test
      replicaset.go
      job.go
      cronjob.go
      service.go
      endpoints.go
      networkpolicy.go
      ingressclass.go
      hpa.go
      pdb.go
      pvc.go
      persistentvolume.go
      storageclass.go
      csidriver.go
      volumeattachment.go
      node.go
      resourcequota.go
      limitrange.go
      priorityclass.go
      lease.go
      namespace.go
      serviceaccount.go
      secret.go
      configmap.go
      operator.go
      role.go
      clusterrole.go
      rolebinding.go
      clusterrolebinding.go
      validatingwebhookconfiguration.go
      mutatingwebhookconfiguration.go
      customresourcedefinition.go
      custom_resource.go
    diag/
      version.go                       # version subcommand
      doctor.go                        # doctor subcommand (kubectl ok? api reachable? rbac ok?)
      doctor_test.go
  docs/
    design.md                          # existing
    PROBE_CONTRACT.md                  # new: what the runner guarantees on stdout/stderr/exit
    TROUBLESHOOTING.md                 # new: operator-facing diagnostics guide
  hooks/install.sh                     # hardened: go/kubectl presence, version gate
  test/integration/
    integration_test.go                # existing
    fixtures/                          # k8s yaml fixtures per type group
      workloads.yaml
      networking.yaml
      storage.yaml
      rbac.yaml
      scheduling.yaml
  .github/workflows/
    ci.yml                             # lint + unit on every push; integration on PR to main
    release.yml                        # tag-triggered: build cross-platform binaries, attach to release
  CHANGELOG.md                         # new: Keep-a-Changelog format
  VERSION                              # new: single source of truth for semver (matches manifest.yaml:meta.version)
```

---

# PHASE 0: Foundation & Safety Net

Before refactoring anything, add the safety net that will catch regressions.

## Task 0.1: Add GitHub Actions CI (lint + unit)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow file**

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: go vet
        run: go vet ./...
      - name: gofmt
        run: |
          diff=$(gofmt -s -d .)
          if [ -n "$diff" ]; then
            echo "$diff"
            echo "::error::gofmt found issues"
            exit 1
          fi

  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: unit tests
        run: go test -race -count=1 ./...

  integration:
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request' && github.base_ref == 'main'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: helm/kind-action@v1
        with:
          cluster_name: mgtt-provider-kubernetes-it
      - name: integration tests
        run: go test -tags=integration -count=1 ./test/integration/...
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add github actions — lint + unit on push, integration on PR"
```

## Task 0.2: Add VERSION file + wire into build

**Files:**
- Create: `VERSION`
- Modify: `main.go` (add `var Version string` populated via ldflags)
- Modify: `hooks/install.sh` (pass `-ldflags "-X main.Version=$(cat VERSION)"`)

- [ ] **Step 1: Write VERSION**

```
2.0.0
```

- [ ] **Step 2: Write failing test for version variable**

Add to `main_test.go`:

```go
func TestVersion_HasDefault(t *testing.T) {
	// If unset, Version should fall back to "dev"
	if Version == "" {
		t.Fatal("Version should never be empty; expected ldflags-injected value or default")
	}
}
```

- [ ] **Step 3: Run — expect failure**

Run: `go test -run TestVersion_HasDefault`
Expected: FAIL — Version is empty.

- [ ] **Step 4: Add Version variable to main.go**

At the top of `main.go` after imports:

```go
// Version is the provider version, injected via ldflags at build time.
var Version = "dev"
```

- [ ] **Step 5: Re-run test**

Run: `go test -run TestVersion_HasDefault`
Expected: PASS.

- [ ] **Step 6: Update install.sh**

Replace the `go build` line in `hooks/install.sh`:

```bash
#!/bin/bash
set -e
cd "$(dirname "$0")/.."
mkdir -p bin
VERSION=$(cat VERSION)
go build -ldflags "-X main.Version=${VERSION}" -o bin/mgtt-provider-kubernetes .
echo "✓ built bin/mgtt-provider-kubernetes ${VERSION}"
```

- [ ] **Step 7: Verify install builds**

Run: `bash hooks/install.sh`
Expected: `✓ built bin/mgtt-provider-kubernetes 2.0.0`

- [ ] **Step 8: Commit**

```bash
git add VERSION main.go main_test.go hooks/install.sh
git commit -m "chore: single-source VERSION file + ldflags injection"
```

## Task 0.3: Write PROBE_CONTRACT.md

**Files:**
- Create: `docs/PROBE_CONTRACT.md`

- [ ] **Step 1: Write the contract**

```markdown
# Probe Contract

The runner binary (`mgtt-provider-kubernetes`) is invoked as:

    mgtt-provider-kubernetes probe <component-name> <fact-name> [--namespace NS] [--type TYPE]

## Output format

On success, exit 0 and print a single JSON object to stdout:

    {"value": <typed value or null>, "raw": "<human-readable string>", "status": "ok"|"not_found"}

- `value` is typed per the fact's declared type in `types/<type>.yaml` (int, bool, string, or null).
- `raw` is a short, human-readable rendering for operators.
- `status` is one of:
  - `ok` — probe executed, value is authoritative.
  - `not_found` — the underlying resource does not exist in the cluster. `value` is null, `raw` is empty. This is NOT an error — simulations and design-time tooling rely on it.

On any other failure, exit non-zero and write a single-line diagnostic to stderr. Exit codes:

| Exit | Meaning |
|---|---|
| 0 | Success (including `status: not_found`) |
| 1 | Usage error (bad args, unknown type/fact) |
| 2 | Environment error (kubectl not on PATH, kubeconfig unreadable) |
| 3 | Forbidden — RBAC denied |
| 4 | Transient — network/timeout; caller may retry |
| 5 | Protocol — kubectl returned malformed JSON |

## Timeouts & size limits

- Each kubectl call is bounded by `MGTT_PROBE_TIMEOUT` (default 10s).
- kubectl output >10 MiB is truncated and treated as exit 5 (protocol error).

## Debug output

Setting `MGTT_PROVIDER_DEBUG=1` writes timing and argv traces to stderr. Never write debug to stdout — it would corrupt the JSON contract.
```

- [ ] **Step 2: Commit**

```bash
git add docs/PROBE_CONTRACT.md
git commit -m "docs: add probe contract — stdout/stderr/exit code guarantees"
```

---

# PHASE 1: Architecture Refactor — Table-Driven Probe Registry

> ⚠️ **PARTIALLY DEPRECATED — SEE GATE AT TOP.** This entire phase was drafted assuming a provider-local `kubeclient` and `probes.Registry`. With the core SDK, most of it is replaced by imports. Before executing any task in this phase, re-read the Deletions list at the top of this plan. Tasks 1.1, 1.2, 1.3, 1.4, 1.8, 1.9 are **DELETE or REPLACE**; only Tasks 1.5 (helpers), 1.6–1.7 (deployment/ingress registrations) survive in simplified form.

Everything after this phase depends on the registry. Do not skip steps.

## Task 1.1: Create internal/kubeclient package with kubectlJSON moved

**Files:**
- Create: `internal/kubeclient/client.go`
- Create: `internal/kubeclient/client_test.go`

- [ ] **Step 1: Write the failing test**

`internal/kubeclient/client_test.go`:

```go
package kubeclient

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestClient_GetJSON_ReturnsErrNotFoundForMissingResource(t *testing.T) {
	// Fake kubectl that exits with NotFound stderr.
	c := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return nil, []byte(`Error from server (NotFound): deployments.apps "missing" not found`),
			&exec.ExitError{ProcessState: nil}
	}}
	_, err := c.GetJSON(context.Background(), "get", "deploy", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_GetJSON_ReturnsErrForbiddenFor403(t *testing.T) {
	c := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return nil, []byte(`Error from server (Forbidden): pods is forbidden: User "x"`),
			&exec.ExitError{}
	}}
	_, err := c.GetJSON(context.Background(), "get", "pods")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestClient_GetJSON_ParsesJSON(t *testing.T) {
	c := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return []byte(`{"status":{"readyReplicas":3}}`), nil, nil
	}}
	got, err := c.GetJSON(context.Background(), "get", "deploy", "x")
	if err != nil {
		t.Fatal(err)
	}
	if got["status"].(map[string]any)["readyReplicas"].(float64) != 3 {
		t.Fatalf("parse mismatch: %+v", got)
	}
}

func TestClient_GetJSON_ProtocolErrorOnBadJSON(t *testing.T) {
	c := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return []byte(`not json`), nil, nil
	}}
	_, err := c.GetJSON(context.Background(), "get", "deploy", "x")
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestClient_GetJSON_SizeCapTriggersProtocolError(t *testing.T) {
	big := make([]byte, 11*1024*1024) // 11 MiB
	for i := range big {
		big[i] = '{'
	}
	c := &Client{
		MaxBytes: 10 * 1024 * 1024,
		Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			return big, nil, nil
		},
	}
	_, err := c.GetJSON(context.Background(), "get", "deploy", "x")
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected ErrProtocol for oversize output, got %v", err)
	}
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/kubeclient/...`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

`internal/kubeclient/client.go`:

```go
// Package kubeclient wraps kubectl with typed errors, timeouts, and size limits.
package kubeclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrNotFound  = errors.New("kubernetes: resource not found")
	ErrForbidden = errors.New("kubernetes: forbidden")
	ErrTransient = errors.New("kubernetes: transient error")
	ErrProtocol  = errors.New("kubernetes: protocol error")
	ErrEnv       = errors.New("kubernetes: environment error")
)

type ExecFn func(ctx context.Context, args ...string) (stdout, stderr []byte, err error)

type Client struct {
	Exec     ExecFn
	MaxBytes int
}

const defaultMaxBytes = 10 * 1024 * 1024

// New returns a Client using real kubectl.
func New() *Client {
	return &Client{
		MaxBytes: defaultMaxBytes,
		Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			cmd := exec.CommandContext(ctx, "kubectl", args...)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			return out, []byte(stderr.String()), err
		},
	}
}

// GetJSON runs `kubectl <args> -o json`, returning the parsed map.
// Missing resources surface as ErrNotFound — callers convert to status:not_found.
func (c *Client) GetJSON(ctx context.Context, args ...string) (map[string]any, error) {
	full := append(append([]string{}, args...), "-o", "json")
	out, stderr, err := c.Exec(ctx, full...)
	if err != nil {
		return nil, classify(err, stderr)
	}
	limit := c.MaxBytes
	if limit == 0 {
		limit = defaultMaxBytes
	}
	if len(out) > limit {
		return nil, fmt.Errorf("%w: kubectl output %d bytes exceeds cap %d", ErrProtocol, len(out), limit)
	}
	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("%w: parse kubectl json: %v", ErrProtocol, err)
	}
	return data, nil
}

func classify(runErr error, stderr []byte) error {
	msg := string(stderr)
	switch {
	case strings.Contains(msg, "NotFound"):
		return fmt.Errorf("%w: %s", ErrNotFound, firstLine(msg))
	case strings.Contains(msg, "Forbidden"):
		return fmt.Errorf("%w: %s", ErrForbidden, firstLine(msg))
	case strings.Contains(msg, "Unable to connect"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "context deadline exceeded"):
		return fmt.Errorf("%w: %s", ErrTransient, firstLine(msg))
	}
	return fmt.Errorf("%w: %v", ErrEnv, firstLine(msg))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/kubeclient/...`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kubeclient/
git commit -m "feat(kubeclient): typed errors, size cap, injectable exec for tests"
```

## Task 1.2: Add kubectlJSON memoization cache

**Files:**
- Create: `internal/kubeclient/cache.go`
- Create: `internal/kubeclient/cache_test.go`

- [ ] **Step 1: Write the failing test**

```go
package kubeclient

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestCachingClient_MemoizesIdenticalArgs(t *testing.T) {
	var calls int32
	inner := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte(`{"a":1}`), nil, nil
	}}
	c := NewCaching(inner)
	_, _ = c.GetJSON(context.Background(), "get", "deploy", "x")
	_, _ = c.GetJSON(context.Background(), "get", "deploy", "x")
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 kubectl call, got %d", calls)
	}
}

func TestCachingClient_DistinctArgsHitBackend(t *testing.T) {
	var calls int32
	inner := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte(`{}`), nil, nil
	}}
	c := NewCaching(inner)
	_, _ = c.GetJSON(context.Background(), "get", "deploy", "x")
	_, _ = c.GetJSON(context.Background(), "get", "deploy", "y")
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 kubectl calls, got %d", calls)
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/kubeclient/... -run Caching`
Expected: FAIL — NewCaching undefined.

- [ ] **Step 3: Implement cache**

`internal/kubeclient/cache.go`:

```go
package kubeclient

import (
	"context"
	"strings"
	"sync"
)

// Getter is the subset of Client used by the cache. It exists so that tests
// can swap in fakes.
type Getter interface {
	GetJSON(ctx context.Context, args ...string) (map[string]any, error)
}

// Caching wraps a Getter with process-lifetime memoization keyed on argv.
// Errors are also cached — callers that want retry should skip the cache.
type Caching struct {
	inner Getter
	mu    sync.Mutex
	hits  map[string]cacheEntry
}

type cacheEntry struct {
	data map[string]any
	err  error
}

func NewCaching(g Getter) *Caching {
	return &Caching{inner: g, hits: map[string]cacheEntry{}}
}

func (c *Caching) GetJSON(ctx context.Context, args ...string) (map[string]any, error) {
	key := strings.Join(args, "\x00")
	c.mu.Lock()
	if e, ok := c.hits[key]; ok {
		c.mu.Unlock()
		return e.data, e.err
	}
	c.mu.Unlock()

	data, err := c.inner.GetJSON(ctx, args...)

	c.mu.Lock()
	c.hits[key] = cacheEntry{data, err}
	c.mu.Unlock()
	return data, err
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/kubeclient/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/kubeclient/cache.go internal/kubeclient/cache_test.go
git commit -m "feat(kubeclient): per-process argv cache — one kubectl per resource"
```

## Task 1.3: Add timeout wrapper

**Files:**
- Modify: `internal/kubeclient/client.go`
- Modify: `internal/kubeclient/client_test.go`

- [ ] **Step 1: Write the failing test** — append to `client_test.go`:

```go
func TestClient_GetJSON_Timeout(t *testing.T) {
	c := &Client{
		Timeout: 1 * time.Millisecond,
		Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			<-ctx.Done()
			return nil, []byte("context deadline exceeded"), ctx.Err()
		},
	}
	_, err := c.GetJSON(context.Background(), "get", "deploy", "x")
	if !errors.Is(err, ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}
```

Add `"time"` to imports.

- [ ] **Step 2: Run — expect failure (Timeout field missing)**

- [ ] **Step 3: Modify Client to add Timeout**

In `client.go`, add `Timeout time.Duration` to struct; in `GetJSON`, wrap `ctx`:

```go
func (c *Client) GetJSON(ctx context.Context, args ...string) (map[string]any, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	// ... rest unchanged
}
```

In `New()`, read env var:

```go
func New() *Client {
	timeout := 10 * time.Second
	if v := os.Getenv("MGTT_PROBE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	return &Client{
		MaxBytes: defaultMaxBytes,
		Timeout:  timeout,
		// ... Exec unchanged
	}
}
```

Add `"os"` and `"time"` imports.

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/kubeclient/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/kubeclient/
git commit -m "feat(kubeclient): per-call timeout (default 10s, MGTT_PROBE_TIMEOUT override)"
```

## Task 1.4: Create probe registry skeleton

**Files:**
- Create: `internal/probes/registry.go`
- Create: `internal/probes/registry_test.go`

- [ ] **Step 1: Write the failing test**

```go
package probes

import (
	"context"
	"errors"
	"testing"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

func TestRegistry_UnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Probe(context.Background(), Request{Type: "bogus", Fact: "x"})
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("expected ErrUnknownType, got %v", err)
	}
}

func TestRegistry_UnknownFact(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"known": func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
			return Result{Value: 1, Raw: "1", Status: "ok"}, nil
		},
	})
	_, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "unknown"})
	if !errors.Is(err, ErrUnknownFact) {
		t.Fatalf("expected ErrUnknownFact, got %v", err)
	}
}

func TestRegistry_DispatchesToRegisteredFn(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"bar": func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
			return Result{Value: 42, Raw: "42", Status: "ok"}, nil
		},
	})
	got, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 42 {
		t.Fatalf("expected 42, got %v", got.Value)
	}
}

func TestRegistry_NotFoundBecomesStatus(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", map[string]ProbeFn{
		"bar": func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
			return Result{}, kubeclient.ErrNotFound
		},
	})
	got, err := r.Probe(context.Background(), Request{Type: "foo", Fact: "bar"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Status != "not_found" {
		t.Fatalf("expected status not_found, got %q", got.Status)
	}
	if got.Value != nil {
		t.Fatalf("expected nil value, got %v", got.Value)
	}
}
```

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/probes/...`
Expected: FAIL — package/types undefined.

- [ ] **Step 3: Implement**

`internal/probes/registry.go`:

```go
// Package probes wires a type+fact pair to a ProbeFn. Types register themselves
// via init(); main.go only needs to dispatch.
package probes

import (
	"context"
	"errors"
	"fmt"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

var (
	ErrUnknownType = errors.New("probes: unknown type")
	ErrUnknownFact = errors.New("probes: unknown fact")
)

type Request struct {
	Type      string
	Name      string
	Namespace string
	Fact      string
}

type Result struct {
	Value  any    `json:"value"`
	Raw    string `json:"raw"`
	Status string `json:"status"`
}

type ProbeFn func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error)

type Registry struct {
	types map[string]map[string]ProbeFn
}

func NewRegistry() *Registry {
	return &Registry{types: map[string]map[string]ProbeFn{}}
}

func (r *Registry) Register(typ string, facts map[string]ProbeFn) {
	r.types[typ] = facts
}

func (r *Registry) Probe(ctx context.Context, req Request) (Result, error) {
	facts, ok := r.types[req.Type]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownType, req.Type)
	}
	fn, ok := facts[req.Fact]
	if !ok {
		return Result{}, fmt.Errorf("%w: type %q has no fact %q", ErrUnknownFact, req.Type, req.Fact)
	}
	res, err := fn(ctx, kubeclient.NewCaching(kubeclient.New()), req)
	if errors.Is(err, kubeclient.ErrNotFound) {
		return Result{Value: nil, Raw: "", Status: "not_found"}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if res.Status == "" {
		res.Status = "ok"
	}
	return res, nil
}

// Default is the package-level registry that types register into via init().
var Default = NewRegistry()
```

Change registry to accept an injectable client for testing. Adjust `Probe` to accept an optional client, or add a `ProbeWith` method. Simpler: expose a package-level `clientFactory`:

```go
var clientFactory = func() kubeclient.Getter { return kubeclient.NewCaching(kubeclient.New()) }
```

And in tests replace it. But the existing test passes the client directly via `ProbeFn` signature — no factory needed. Remove the `r.types[typ][fact](ctx, client, req)` closure's hardcoded client by exposing `ProbeWith`:

Final shape — add:

```go
func (r *Registry) ProbeWith(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	facts, ok := r.types[req.Type]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownType, req.Type)
	}
	fn, ok := facts[req.Fact]
	if !ok {
		return Result{}, fmt.Errorf("%w: type %q has no fact %q", ErrUnknownFact, req.Type, req.Fact)
	}
	res, err := fn(ctx, c, req)
	if errors.Is(err, kubeclient.ErrNotFound) {
		return Result{Value: nil, Raw: "", Status: "not_found"}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if res.Status == "" {
		res.Status = "ok"
	}
	return res, nil
}

func (r *Registry) Probe(ctx context.Context, req Request) (Result, error) {
	return r.ProbeWith(ctx, kubeclient.NewCaching(kubeclient.New()), req)
}
```

Tests in Step 1 using `r.Probe` without a custom client will still run — they don't hit kubectl because the registered fn returns synchronously.

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/probes/...`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/probes/registry.go internal/probes/registry_test.go
git commit -m "feat(probes): type/fact registry with NotFound→status translation"
```

## Task 1.5: Extract probe helpers

**Files:**
- Create: `internal/probes/helpers.go`
- Create: `internal/probes/helpers_test.go`

- [ ] **Step 1: Write failing tests**

Copy the existing helper tests (`TestJsonInt*`, `TestMaxRestartCount*`, `TestCountEndpointAddresses*`, `TestIntResult*`) from `main_test.go` into `internal/probes/helpers_test.go`, changing the package to `probes` and renaming `intResult` → `IntResult`. Add new helpers:

```go
package probes

import "testing"

func TestJSONBool_TrueString(t *testing.T) {
	data := map[string]any{"status": map[string]any{"v": "True"}}
	if got := JSONBool(data, "status", "v"); !got {
		t.Fatal("expected true")
	}
}

func TestJSONBool_FalseString(t *testing.T) {
	data := map[string]any{"status": map[string]any{"v": "False"}}
	if got := JSONBool(data, "status", "v"); got {
		t.Fatal("expected false")
	}
}

func TestJSONBool_ActualBool(t *testing.T) {
	data := map[string]any{"v": true}
	if !JSONBool(data, "v") {
		t.Fatal("expected true")
	}
}

func TestJSONString_Present(t *testing.T) {
	data := map[string]any{"spec": map[string]any{"class": "nginx"}}
	if got := JSONString(data, "spec", "class"); got != "nginx" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONString_Missing(t *testing.T) {
	data := map[string]any{}
	if got := JSONString(data, "spec", "class"); got != "" {
		t.Fatal("expected empty")
	}
}

func TestCountList(t *testing.T) {
	data := map[string]any{"items": []any{1, 2, 3}}
	if got := CountList(data, "items"); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestConditionStatus_Found(t *testing.T) {
	data := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Available", "status": "True"},
				map[string]any{"type": "Progressing", "status": "False"},
			},
		},
	}
	if !ConditionStatus(data, "Available") {
		t.Fatal("expected Available=true")
	}
	if ConditionStatus(data, "Progressing") {
		t.Fatal("expected Progressing=false")
	}
}

func TestConditionStatus_NotFoundReturnsFalse(t *testing.T) {
	data := map[string]any{"status": map[string]any{"conditions": []any{}}}
	if ConditionStatus(data, "Anything") {
		t.Fatal("missing condition should be false")
	}
}

func TestBoolResult(t *testing.T) {
	r := BoolResult(true)
	if r.Value != true || r.Raw != "true" {
		t.Fatalf("got %+v", r)
	}
}

func TestStringResult(t *testing.T) {
	r := StringResult("hello")
	if r.Value != "hello" || r.Raw != "hello" {
		t.Fatalf("got %+v", r)
	}
}
```

- [ ] **Step 2: Implement helpers**

`internal/probes/helpers.go`:

```go
package probes

import (
	"fmt"
	"strings"
)

// JSONInt traverses nested map by path, returning the value as int.
// 0 for missing/nil/non-numeric.
func JSONInt(data map[string]any, path ...string) int {
	v := walk(data, path...)
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		// kubectl sometimes returns stringified ints in jsonpath mode.
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

// JSONBool accepts both actual bools and the "True"/"False" strings kubernetes
// uses in conditions.
func JSONBool(data map[string]any, path ...string) bool {
	switch x := walk(data, path...).(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "True")
	}
	return false
}

func JSONString(data map[string]any, path ...string) string {
	if s, ok := walk(data, path...).(string); ok {
		return s
	}
	return ""
}

func CountList(data map[string]any, path ...string) int {
	if l, ok := walk(data, path...).([]any); ok {
		return len(l)
	}
	return 0
}

// ConditionStatus finds the named condition in status.conditions and returns
// whether its status is "True". Missing condition returns false.
func ConditionStatus(data map[string]any, name string) bool {
	conds, _ := walk(data, "status", "conditions").([]any)
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if t, _ := m["type"].(string); t == name {
			s, _ := m["status"].(string)
			return strings.EqualFold(s, "True")
		}
	}
	return false
}

// MaxRestartCount returns the highest restartCount across all containers in
// all pods in a pod list.
func MaxRestartCount(data map[string]any) int {
	items, _ := data["items"].([]any)
	maxVal := 0
	for _, item := range items {
		pod, _ := item.(map[string]any)
		status, _ := pod["status"].(map[string]any)
		containers, _ := status["containerStatuses"].([]any)
		for _, c := range containers {
			cs, _ := c.(map[string]any)
			if v, ok := cs["restartCount"].(float64); ok && int(v) > maxVal {
				maxVal = int(v)
			}
		}
	}
	return maxVal
}

// CountEndpointAddresses counts addresses across all subsets of an Endpoints resource.
func CountEndpointAddresses(data map[string]any) int {
	subsets, _ := data["subsets"].([]any)
	count := 0
	for _, s := range subsets {
		subset, _ := s.(map[string]any)
		addrs, _ := subset["addresses"].([]any)
		count += len(addrs)
	}
	return count
}

func IntResult(v int) Result    { return Result{Value: v, Raw: fmt.Sprintf("%d", v), Status: "ok"} }
func BoolResult(v bool) Result  { return Result{Value: v, Raw: fmt.Sprintf("%t", v), Status: "ok"} }
func StringResult(v string) Result { return Result{Value: v, Raw: v, Status: "ok"} }

func walk(m map[string]any, path ...string) any {
	var cur any = m
	for _, k := range path {
		mp, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mp[k]
	}
	return cur
}
```

- [ ] **Step 3: Run — expect pass**

Run: `go test ./internal/probes/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/probes/helpers.go internal/probes/helpers_test.go
git commit -m "feat(probes): shared helpers for common kubectl JSON shapes"
```

## Task 1.6: Migrate deployment probes to registry

**Files:**
- Create: `internal/probes/deployment.go`
- Create: `internal/probes/deployment_test.go`

- [ ] **Step 1: Write failing tests**

```go
package probes

import (
	"context"
	"testing"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

type fakeClient struct {
	responses map[string]map[string]any
	err       error
}

func (f *fakeClient) GetJSON(_ context.Context, args ...string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := args[1] + "/" + args[2] // e.g. "deploy/nginx"
	return f.responses[key], nil
}

func TestDeployment_ReadyReplicas(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"deploy/nginx": {"status": map[string]any{"readyReplicas": float64(3)}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "deployment", Name: "nginx", Namespace: "default", Fact: "ready_replicas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 3 {
		t.Fatalf("expected 3, got %v", got.Value)
	}
}

func TestDeployment_ConditionAvailable(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"deploy/nginx": {"status": map[string]any{"conditions": []any{
			map[string]any{"type": "Available", "status": "True"},
		}}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "deployment", Name: "nginx", Namespace: "default", Fact: "condition_available",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != true {
		t.Fatalf("expected true, got %v", got.Value)
	}
}

func TestDeployment_NotFound(t *testing.T) {
	c := &fakeClient{err: kubeclient.ErrNotFound}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "deployment", Name: "missing", Namespace: "default", Fact: "ready_replicas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "not_found" {
		t.Fatalf("expected not_found, got %q", got.Status)
	}
}
```

- [ ] **Step 2: Run — expect failure (no deployment probes registered)**

- [ ] **Step 3: Implement**

`internal/probes/deployment.go`:

```go
package probes

import (
	"context"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

func init() {
	Default.Register("deployment", map[string]ProbeFn{
		"ready_replicas":       deployGet("readyReplicas"),
		"desired_replicas":     deploySpecReplicas,
		"updated_replicas":     deployGet("updatedReplicas"),
		"available_replicas":   deployGet("availableReplicas"),
		"unavailable_replicas": deployGet("unavailableReplicas"),
		"condition_available":  deployCond("Available"),
		"condition_progressing": deployCond("Progressing"),
		"restart_count":        deployRestartCount,
		"endpoints":            deployEndpoints,
	})
}

func deployGet(statusField string) ProbeFn {
	return func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
		data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "deploy", req.Name)
		if err != nil {
			return Result{}, err
		}
		return IntResult(JSONInt(data, "status", statusField)), nil
	}
}

func deploySpecReplicas(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "deploy", req.Name)
	if err != nil {
		return Result{}, err
	}
	return IntResult(JSONInt(data, "spec", "replicas")), nil
}

func deployCond(name string) ProbeFn {
	return func(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
		data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "deploy", req.Name)
		if err != nil {
			return Result{}, err
		}
		return BoolResult(ConditionStatus(data, name)), nil
	}
}

func deployRestartCount(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "pods", "-l", "app="+req.Name)
	if err != nil {
		return Result{}, err
	}
	return IntResult(MaxRestartCount(data)), nil
}

func deployEndpoints(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "endpoints", req.Name)
	if err != nil {
		return Result{}, err
	}
	return IntResult(CountEndpointAddresses(data)), nil
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/probes/... -run Deployment`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/probes/deployment.go internal/probes/deployment_test.go
git commit -m "feat(probes/deployment): migrate to registry — 9 facts via table dispatch"
```

## Task 1.7: Migrate ingress probes to registry

**Files:**
- Create: `internal/probes/ingress.go`
- Create: `internal/probes/ingress_test.go`

- [ ] **Step 1: Write tests** (mirror deployment test pattern for `upstream_count`, `backend_count`, `address_assigned`, `tls_configured`, `class`)

```go
package probes

import (
	"context"
	"testing"
)

func TestIngress_UpstreamCount(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"endpoints/myapp": {"subsets": []any{
			map[string]any{"addresses": []any{map[string]any{}, map[string]any{}}},
		}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "ingress", Name: "myapp", Namespace: "default", Fact: "upstream_count",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 2 {
		t.Fatalf("expected 2, got %v", got.Value)
	}
}

func TestIngress_AddressAssigned_True(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"ingress/myapp": {"status": map[string]any{"loadBalancer": map[string]any{
			"ingress": []any{map[string]any{"ip": "10.0.0.1"}},
		}}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "ingress", Name: "myapp", Namespace: "default", Fact: "address_assigned",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != true {
		t.Fatalf("expected true, got %v", got.Value)
	}
}

func TestIngress_Class(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"ingress/myapp": {"spec": map[string]any{"ingressClassName": "nginx"}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "ingress", Name: "myapp", Namespace: "default", Fact: "class",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "nginx" {
		t.Fatalf("expected nginx, got %v", got.Value)
	}
}

func TestIngress_BackendCount(t *testing.T) {
	c := &fakeClient{responses: map[string]map[string]any{
		"ingress/myapp": {"spec": map[string]any{"rules": []any{
			map[string]any{"http": map[string]any{"paths": []any{
				map[string]any{}, map[string]any{},
			}}},
			map[string]any{"http": map[string]any{"paths": []any{map[string]any{}}}},
		}}},
	}}
	got, err := Default.ProbeWith(context.Background(), c, Request{
		Type: "ingress", Name: "myapp", Namespace: "default", Fact: "backend_count",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != 3 {
		t.Fatalf("expected 3, got %v", got.Value)
	}
}
```

- [ ] **Step 2: Implement**

`internal/probes/ingress.go`:

```go
package probes

import (
	"context"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

func init() {
	Default.Register("ingress", map[string]ProbeFn{
		"upstream_count":   ingressUpstreamCount,
		"backend_count":    ingressBackendCount,
		"address_assigned": ingressAddressAssigned,
		"tls_configured":   ingressTLSConfigured,
		"class":            ingressClass,
	})
}

func ingressUpstreamCount(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "endpoints", req.Name)
	if err != nil {
		return Result{}, err
	}
	return IntResult(CountEndpointAddresses(data)), nil
}

func ingressBackendCount(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "ingress", req.Name)
	if err != nil {
		return Result{}, err
	}
	rules, _ := walk(data, "spec", "rules").([]any)
	total := 0
	for _, r := range rules {
		rm, _ := r.(map[string]any)
		if paths, ok := walk(rm, "http", "paths").([]any); ok {
			total += len(paths)
		}
	}
	return IntResult(total), nil
}

func ingressAddressAssigned(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "ingress", req.Name)
	if err != nil {
		return Result{}, err
	}
	lb, _ := walk(data, "status", "loadBalancer", "ingress").([]any)
	return BoolResult(len(lb) > 0), nil
}

func ingressTLSConfigured(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "ingress", req.Name)
	if err != nil {
		return Result{}, err
	}
	tls, _ := walk(data, "spec", "tls").([]any)
	return BoolResult(len(tls) > 0), nil
}

func ingressClass(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "ingress", req.Name)
	if err != nil {
		return Result{}, err
	}
	return StringResult(JSONString(data, "spec", "ingressClassName")), nil
}
```

- [ ] **Step 3: Run — expect pass**

Run: `go test ./internal/probes/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/probes/ingress.go internal/probes/ingress_test.go
git commit -m "feat(probes/ingress): migrate to registry — 5 facts"
```

## Task 1.8: Rewrite main.go as thin dispatcher

**Files:**
- Modify: `main.go`
- Modify: `main_test.go` (delete migrated helper tests; keep only flag parsing tests)

- [ ] **Step 1: Write test for flag parsing**

Replace contents of `main_test.go` with:

```go
package main

import "testing"

func TestParseArgs_Defaults(t *testing.T) {
	r, err := parseArgs([]string{"probe", "nginx", "ready_replicas"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != "deployment" || r.Namespace != "default" || r.Name != "nginx" || r.Fact != "ready_replicas" {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestParseArgs_ExplicitFlags(t *testing.T) {
	r, err := parseArgs([]string{"probe", "web", "ready", "--namespace", "prod", "--type", "pod"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Namespace != "prod" || r.Type != "pod" {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestParseArgs_TooFewArgs(t *testing.T) {
	if _, err := parseArgs([]string{"probe"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseArgs_UnknownSubcommand(t *testing.T) {
	if _, err := parseArgs([]string{"bogus"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestVersion_HasDefault(t *testing.T) {
	if Version == "" {
		t.Fatal("Version empty")
	}
}
```

- [ ] **Step 2: Rewrite `main.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"mgtt-provider-kubernetes/internal/kubeclient"
	"mgtt-provider-kubernetes/internal/probes"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(Version)
		return
	}

	req, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	res, err := probes.Default.Probe(context.Background(), req)
	if err != nil {
		exitCode := classifyExit(err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode)
	}
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
}

func parseArgs(args []string) (probes.Request, error) {
	if len(args) == 0 {
		return probes.Request{}, errors.New("usage: mgtt-provider-kubernetes probe <name> <fact> [--namespace NS] [--type TYPE]")
	}
	if args[0] != "probe" {
		return probes.Request{}, fmt.Errorf("unknown command: %s", args[0])
	}
	if len(args) < 3 {
		return probes.Request{}, errors.New("probe requires <name> and <fact>")
	}
	r := probes.Request{
		Name:      args[1],
		Fact:      args[2],
		Namespace: "default",
		Type:      "deployment",
	}
	for i := 3; i < len(args)-1; i++ {
		switch args[i] {
		case "--namespace":
			r.Namespace = args[i+1]
		case "--type":
			r.Type = args[i+1]
		}
	}
	return r, nil
}

func classifyExit(err error) int {
	switch {
	case errors.Is(err, probes.ErrUnknownType), errors.Is(err, probes.ErrUnknownFact):
		return 1
	case errors.Is(err, kubeclient.ErrEnv):
		return 2
	case errors.Is(err, kubeclient.ErrForbidden):
		return 3
	case errors.Is(err, kubeclient.ErrTransient):
		return 4
	case errors.Is(err, kubeclient.ErrProtocol):
		return 5
	}
	return 1
}
```

- [ ] **Step 3: Update go.mod module path**

Ensure `go.mod` has `module mgtt-provider-kubernetes` (already does).

- [ ] **Step 4: Run full suite**

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 5: Smoke-test the binary**

Run:

```bash
go build -o /tmp/k .
/tmp/k version
```

Expected: prints `dev` (or version from ldflags).

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "refactor: main.go dispatches via probes registry — 2-type shim replaced"
```

## Task 1.9: Add vocabulary↔runner parity test

**Files:**
- Create: `internal/probes/parity_test.go`

- [ ] **Step 1: Write the test**

```go
package probes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)
```

Wait — we haven't introduced YAML parsing dep yet. Use a minimal parser or just shell to find facts by regex. Avoid new deps: use regex.

Rewrite using regex:

```go
package probes

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// factRegex matches "  factname:" at column 2 under a top-level "facts:" block.
var factKey = regexp.MustCompile(`(?m)^  ([a-z_]+):\s*$`)
var topFacts = regexp.MustCompile(`(?m)^facts:\s*$`)

// declaredFacts parses a types/<type>.yaml file and returns the fact names
// declared under `facts:`. The format is stable and small — regex is
// sufficient and keeps this test dep-free.
func declaredFacts(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Find start of "facts:" block and end at the next top-level key.
	idx := topFacts.FindIndex(raw)
	if idx == nil {
		return nil, nil
	}
	block := raw[idx[1]:]
	// End block at next line starting at column 0 with a non-space.
	endLine := regexp.MustCompile(`(?m)^[a-z]`)
	if end := endLine.FindIndex(block); end != nil {
		block = block[:end[0]]
	}
	var out []string
	for _, m := range factKey.FindAllSubmatch(block, -1) {
		out = append(out, string(m[1]))
	}
	return out, nil
}

func TestParity_EveryDeclaredFactHasProbeFn(t *testing.T) {
	// Exemption list: types/facts intentionally declared in vocabulary but
	// not yet implemented. Every line MUST have an issue link or justification.
	exempt := map[string]bool{
		// "<type>/<fact>": true,
	}

	matches, err := filepath.Glob("../../types/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no type files found — run from internal/probes")
	}
	var missing []string
	for _, p := range matches {
		typ := strings.TrimSuffix(filepath.Base(p), ".yaml")
		facts, err := declaredFacts(p)
		if err != nil {
			t.Fatal(err)
		}
		registered := Default.types[typ]
		for _, f := range facts {
			if exempt[typ+"/"+f] {
				continue
			}
			if _, ok := registered[f]; !ok {
				missing = append(missing, typ+"/"+f)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("runner missing probes for declared facts:\n  %s\n\n"+
			"Either implement the probe in internal/probes/<type>.go, or add to the exempt map with justification.",
			strings.Join(missing, "\n  "))
	}
}
```

- [ ] **Step 2: Run — expect failure listing 35 types worth of missing facts**

Run: `go test ./internal/probes/ -run Parity`
Expected: FAIL with a big list. That's the point — it's the to-do list for Phases 4–8.

- [ ] **Step 3: Populate the exempt map with all current gaps**

Run the test, pipe output to a file, convert to exempt map entries. Format each as `"type/fact": true, // implement in phase N`. This lets us commit phase 1 green while documenting the debt.

```bash
go test ./internal/probes/ -run Parity 2>&1 | grep '^  ' | sed 's|^  |"|; s|$|": true, // phase 4-8|' > /tmp/exempt
```

Then paste into `parity_test.go`'s `exempt` map, sorted.

- [ ] **Step 4: Re-run — expect pass**

Run: `go test ./internal/probes/...`
Expected: PASS (the exempt list covers everything unimplemented).

- [ ] **Step 5: Commit**

```bash
git add internal/probes/parity_test.go
git commit -m "test(probes): parity — every declared fact must have a registered ProbeFn"
```

Going forward, Phases 4–8 will **remove entries** from the exempt map as they implement probes. The final PR of phase 8 should leave the exempt map empty.

---

# PHASE 2: Robustness Layer Polish

> ⛔ **DEPRECATED — DO NOT EXECUTE.** All robustness concerns in this phase (kubectl-absent, debug logging, timeouts, stderr classification) are provided by `sdk/provider/shell.Client` with a kubectl-specific `Classify` function. See the Gate at the top of this plan. The only remaining task is the ~15-line kubectl classifier, already captured in the revised Phase 1.

The kubeclient already has timeouts, size caps, and typed errors. Remaining gaps: kubectl-absent detection, debug logging, friendly startup error.

## Task 2.1: kubectl presence check with friendly error

**Files:**
- Modify: `internal/kubeclient/client.go`
- Modify: `internal/kubeclient/client_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestClient_GetJSON_KubectlNotOnPATH(t *testing.T) {
	c := &Client{Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return nil, nil, &exec.Error{Name: "kubectl", Err: exec.ErrNotFound}
	}}
	_, err := c.GetJSON(context.Background(), "get", "deploy", "x")
	if !errors.Is(err, ErrEnv) {
		t.Fatalf("expected ErrEnv, got %v", err)
	}
	if !strings.Contains(err.Error(), "kubectl") {
		t.Fatalf("error should mention kubectl: %v", err)
	}
}
```

- [ ] **Step 2: Update `classify`** to recognize `exec.ErrNotFound`:

```go
import execpkg "os/exec"

func classify(runErr error, stderr []byte) error {
	if errors.Is(runErr, execpkg.ErrNotFound) {
		return fmt.Errorf("%w: kubectl not found on PATH — install kubectl or set MGTT_PROVIDER_DEBUG=1 to diagnose", ErrEnv)
	}
	// ... existing logic
}
```

- [ ] **Step 3: Run — expect pass**

- [ ] **Step 4: Commit**

```bash
git add internal/kubeclient/
git commit -m "feat(kubeclient): friendly error when kubectl is not on PATH"
```

## Task 2.2: Debug logging via MGTT_PROVIDER_DEBUG

**Files:**
- Create: `internal/kubeclient/debug.go`
- Create: `internal/kubeclient/debug_test.go`
- Modify: `internal/kubeclient/client.go` (call debug at start/end of GetJSON)

- [ ] **Step 1: Write failing test**

```go
package kubeclient

import (
	"bytes"
	"testing"
)

func TestDebugf_WritesWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	d := &Debug{Enabled: true, W: &buf}
	d.Debugf("hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("expected log, got %q", buf.String())
	}
}

func TestDebugf_SilentWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	d := &Debug{Enabled: false, W: &buf}
	d.Debugf("hello")
	if buf.Len() != 0 {
		t.Fatalf("expected silence, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Implement**

`internal/kubeclient/debug.go`:

```go
package kubeclient

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Debug struct {
	Enabled bool
	W       io.Writer
}

func NewDebug() *Debug {
	return &Debug{
		Enabled: os.Getenv("MGTT_PROVIDER_DEBUG") == "1",
		W:       os.Stderr,
	}
}

func (d *Debug) Debugf(format string, args ...any) {
	if !d.Enabled {
		return
	}
	fmt.Fprintf(d.W, "[mgtt-k8s %s] "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
}
```

Wire into client: add `Debug *Debug` field. In `GetJSON`, log argv before and duration after.

- [ ] **Step 3: Run — expect pass**

- [ ] **Step 4: Commit**

```bash
git add internal/kubeclient/debug.go internal/kubeclient/debug_test.go internal/kubeclient/client.go
git commit -m "feat(kubeclient): MGTT_PROVIDER_DEBUG traces argv and timing to stderr"
```

---

# PHASE 3: Observability Subcommands

> ⛔ **DEPRECATED — DO NOT EXECUTE.** `version` is provided by `provider.Main` in the SDK. `doctor` is replaced by `mgtt provider validate [--live] kubernetes` in core. See the Gate at the top of this plan.

## Task 3.1: version subcommand (already wired in main.go)

Already done in Task 1.8 — verify with:

```bash
go build -ldflags "-X main.Version=2.0.0" -o /tmp/k .
/tmp/k version
```

Expected output: `2.0.0`.

No commit needed if the verification passes.

## Task 3.2: doctor subcommand

**Files:**
- Create: `internal/diag/doctor.go`
- Create: `internal/diag/doctor_test.go`
- Modify: `main.go`

- [ ] **Step 1: Write failing test**

```go
package diag

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

type fakeGet struct {
	err error
}

func (f *fakeGet) GetJSON(_ context.Context, args ...string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{"kind": "APIVersions"}, nil
}

func TestDoctor_AllGreen(t *testing.T) {
	var buf bytes.Buffer
	code := Doctor(context.Background(), &fakeGet{}, &buf)
	if code != 0 {
		t.Fatalf("expected 0, got %d; output:\n%s", code, buf.String())
	}
}

func TestDoctor_NoKubectl(t *testing.T) {
	var buf bytes.Buffer
	code := Doctor(context.Background(), &fakeGet{err: kubeclient.ErrEnv}, &buf)
	if code == 0 {
		t.Fatal("expected non-zero exit when env broken")
	}
}
```

- [ ] **Step 2: Implement**

`internal/diag/doctor.go`:

```go
// Package diag provides doctor/version subcommands — operator-facing
// diagnostics that do not exercise the probe contract.
package diag

import (
	"context"
	"errors"
	"fmt"
	"io"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

// Doctor runs a sequence of health checks and prints a report. Returns
// 0 on success, >0 on failure — same conventions as `probe`.
func Doctor(ctx context.Context, c kubeclient.Getter, w io.Writer) int {
	fmt.Fprintln(w, "mgtt-provider-kubernetes doctor")

	// Check 1: cluster reachable via kubectl.
	fmt.Fprint(w, "  cluster reachable... ")
	if _, err := c.GetJSON(ctx, "api-versions"); err != nil {
		if errors.Is(err, kubeclient.ErrEnv) {
			fmt.Fprintln(w, "FAIL — kubectl not on PATH")
			return 2
		}
		fmt.Fprintf(w, "FAIL — %v\n", err)
		return 4
	}
	fmt.Fprintln(w, "ok")

	// Check 2: RBAC can read deployments (spot check).
	fmt.Fprint(w, "  can get deployments... ")
	if _, err := c.GetJSON(ctx, "auth", "can-i", "get", "deployments"); err != nil {
		if errors.Is(err, kubeclient.ErrForbidden) {
			fmt.Fprintln(w, "FAIL — forbidden (cluster role lacks `get deployments`)")
			return 3
		}
		fmt.Fprintf(w, "WARN — %v\n", err)
	} else {
		fmt.Fprintln(w, "ok")
	}

	return 0
}
```

- [ ] **Step 3: Wire into main.go**

Add branch after `version`:

```go
if len(os.Args) > 1 && os.Args[1] == "doctor" {
	c := kubeclient.NewCaching(kubeclient.New())
	os.Exit(diag.Doctor(context.Background(), c, os.Stdout))
}
```

Add import `"mgtt-provider-kubernetes/internal/diag"`.

- [ ] **Step 4: Run — expect pass**

- [ ] **Step 5: Commit**

```bash
git add internal/diag/ main.go
git commit -m "feat(diag): doctor subcommand — cluster reachability + RBAC spot check"
```

---

# PHASE 4: Workloads Group (6 types — Tier 1 only per S4)

> ⚠️ **SCOPE ADJUSTED.** Per the triage at the top of this plan, implement Tier 1 types here: **deployment, statefulset, daemonset, pod** (Tier 1 from workloads) plus **service, endpoints, ingress** (Tier 1 from networking — merged into this phase to ship one coherent tier). Move **replicaset, cronjob, job** to a Tier 2 follow-up plan. Do not implement webhooks/CSI/leases/priorityclass in this cycle.

Each type follows the same shape as deployment (Task 1.6). For each type:
1. Write `<type>_test.go` with one test per fact.
2. Write `<type>.go` with `init()` registering ProbeFns.
3. Run tests.
4. Remove that type's entries from the `exempt` map in `parity_test.go`.
5. Commit.

**Types:** `statefulset`, `daemonset`, `replicaset`, `cronjob`, `job`, `pod`.

The concrete fact list per type lives in `types/<type>.yaml` — read it, then translate each `probe.cmd` into a kubectl argv + parser. Below is the template and one worked example.

## Task 4.1: Template — how to add a type

For every remaining type:

- [ ] **Step 1: Read `types/<type>.yaml` and list the facts.**

Run: `grep -E '^  [a-z_]+:' types/<type>.yaml | awk -F: '{print $1}' | sort -u`

- [ ] **Step 2: For each fact, identify the kubectl argv and JSON path.**

The YAML's `probe.cmd` gives the kubectl call; the `parse:` line indicates the return type. Convert to `c.GetJSON(ctx, ...)` + one of `JSONInt`/`JSONBool`/`JSONString`/`ConditionStatus`/`CountList`.

- [ ] **Step 3: Write the test file.**

One test per fact, using the `fakeClient` pattern from `deployment_test.go`.

- [ ] **Step 4: Write the implementation file.**

```go
package probes

import (
	"context"

	"mgtt-provider-kubernetes/internal/kubeclient"
)

func init() {
	Default.Register("<type>", map[string]ProbeFn{
		"<fact1>": <type><Fact1>,
		// ...
	})
}

func <type><Fact1>(ctx context.Context, c kubeclient.Getter, req Request) (Result, error) {
	data, err := c.GetJSON(ctx, "-n", req.Namespace, "get", "<kind>", req.Name)
	if err != nil {
		return Result{}, err
	}
	return IntResult(JSONInt(data, "status", "<field>")), nil
}
```

- [ ] **Step 5: Run `go test ./internal/probes/... -run <Type>` — expect pass.**

- [ ] **Step 6: Remove the type's entries from `parity_test.go` exempt map.**

- [ ] **Step 7: Run full parity test** — expect pass (no new missing facts).

- [ ] **Step 8: Commit.**

```bash
git add internal/probes/<type>.go internal/probes/<type>_test.go internal/probes/parity_test.go
git commit -m "feat(probes/<type>): implement N facts via registry"
```

## Task 4.2: Implement `statefulset`

Apply the template. Facts per `types/statefulset.yaml`. Example skeleton:

- Facts: `ready_replicas`, `desired_replicas`, `current_replicas`, `updated_replicas`, `restart_count`, `condition_available` (read the actual file to confirm).
- Resource kind for kubectl: `statefulset`.

Full implementation lives in `internal/probes/statefulset.go`; test file mirrors the deployment pattern.

Commit: `feat(probes/statefulset): implement N facts`

## Task 4.3: Implement `daemonset`

Apply template. Kind: `daemonset`. Fact translation per `types/daemonset.yaml`.

Commit.

## Task 4.4: Implement `replicaset`

Apply template. Kind: `replicaset`.

Commit.

## Task 4.5: Implement `cronjob`

Apply template. Kind: `cronjob`. Note: includes facts like `last_successful_time`, which may need a time-typed helper — if so, extend `helpers.go` with `StringResult` (already exists) or a `TimeResult` if needed.

Commit.

## Task 4.6: Implement `job`

Apply template. Kind: `job`.

Commit.

## Task 4.7: Implement `pod`

Apply template. Kind: `pod`. Facts: `phase`, `ready`, `restart_count`, `scheduled`, `containers_ready`, `oom_killed` (per `types/pod.yaml`). `oom_killed` reads `status.containerStatuses[*].lastState.terminated.reason` — add a helper `AnyContainerReason` if the expression recurs.

Commit.

---

# PHASE 5: Networking Group (4 types) — **Tier 2, DEFERRED**

> ⛔ **DEFERRED.** Networking Tier 1 types (service, endpoints, ingress) are implemented in Phase 4. Remaining networking types (`networkpolicy`, `ingressclass`) are Tier 2 — move to a follow-up plan.

**Types:** `service`, `endpoints`, `networkpolicy`, `ingressclass`.

Same template as Phase 4. One task per type.

## Task 5.1–5.4: service / endpoints / networkpolicy / ingressclass

For each: read `types/<type>.yaml`, translate facts, write tests + impl, remove exempt entries, commit.

Notable helper additions likely needed:
- `service`: selector presence (`spec.selector` non-empty)
- `networkpolicy`: ingress/egress rule counts — reuse `CountList`
- `endpoints`: subsets count, addresses count — already have `CountEndpointAddresses`

---

# PHASE 6: Storage Group (5 types) — **Tier 1: pvc, node (from scheduling), hpa (from scheduling) only**

> ⚠️ **SCOPE ADJUSTED.** Implement only `pvc` here (Tier 1). Move `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment` to Tier 2 / Tier 3 per the triage. Also implement `node` and `hpa` here (they are Tier 1 from the scheduling group — merged for coherence).

**Types:** `pvc`, `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment`.

## Task 6.1–6.5: One per type

Notable helper additions:
- `pvc`/`pv`: `capacity` facts may return quantity strings ("5Gi") — if the vocabulary wants an int, add a `ParseQuantityBytes` helper now (pure function, unit-testable, no deps).
- `storageclass`: `provisioner` is string.

Ensure `ParseQuantityBytes` handles Ki/Mi/Gi/Ti and K/M/G/T (decimal) and returns bytes. Add to `helpers.go` with tests covering each suffix and malformed input returning 0.

---

# PHASE 7: Scheduling & Cluster Group (7 types) — **DEFERRED**

> ⛔ **DEFERRED.** `node` and `hpa` (Tier 1) were moved to Phase 6 for coherence. Remaining scheduling types (`pdb`, `resourcequota`, `limitrange`, `priorityclass`, `lease`) are Tier 2 or Tier 3 — do not implement in this cycle.

**Types:** `hpa`, `pdb`, `node`, `resourcequota`, `limitrange`, `priorityclass`, `lease`.

## Task 7.1–7.7: One per type

Notable helper additions:
- `node`: conditions `Ready`/`MemoryPressure`/`DiskPressure`/`PIDPressure` — use existing `ConditionStatus`.
- `resourcequota` / `limitrange`: usage vs hard limits — may need a `ResourceMap` helper.
- `hpa`: `current_replicas`, `desired_replicas`, `target_cpu_utilization` — straightforward int facts.

---

# PHASE 8: Prerequisites, RBAC, Webhooks, Extensibility (15 types) — **DEFERRED**

> ⛔ **DEFERRED.** All 15 types in this phase are Tier 2 or Tier 3 per the triage at the top of this plan. Do not implement in this cycle. The vocabulary YAMLs stay; `mgtt provider validate` marks them permanently exempt with a pointer to the triage rationale.

## Task 8.1–8.5: Prerequisites group

`namespace`, `serviceaccount`, `secret`, `configmap`, `operator`.
- `operator` probing is context-dependent: its facts likely include CRD presence; reuse the pattern from custom_resource below.

## Task 8.6–8.9: RBAC group

`role`, `clusterrole`, `rolebinding`, `clusterrolebinding`.

These are mostly existence + rule count. Straightforward.

## Task 8.10–8.11: Webhooks group

`validatingwebhookconfiguration`, `mutatingwebhookconfiguration`.

Facts: `webhook_count`, `failure_policy`, etc. — read the YAML for the authoritative list.

## Task 8.12–8.15: Extensibility group

`customresourcedefinition`, `priorityclass` (already in Phase 7 — confirm placement), `lease` (Phase 7), `custom_resource`.

`custom_resource` is special: it probes arbitrary CRD instances. The probe signature needs an extra param (the CRD group/version/kind). Options:

**Option A (recommended):** encode group/version/kind into `req.Name` via a convention like `gvk:apps.example.com/v1/MyKind/instance-name`. Parse in the probe.

**Option B:** add an optional `--crd-kind`, `--crd-group` flags to the runner. More invasive.

Pick A for v0. Document in `PROBE_CONTRACT.md`.

---

# PHASE 9: Integration Test Expansion

## Task 9.1: Add fixtures for each type group

**Files:**
- Create: `test/integration/fixtures/workloads.yaml`
- Create: `test/integration/fixtures/networking.yaml`
- Create: `test/integration/fixtures/storage.yaml`

- [ ] **Step 1: Write `workloads.yaml`**

Declare a StatefulSet, a DaemonSet, a Pod, a Job, a CronJob — each minimal enough to reach a known state quickly on kind.

- [ ] **Step 2–3**: same for networking and storage.

- [ ] **Step 4: Update `integration_test.go` to apply fixtures and add one test per type group asserting at least one fact returns a sensible value.**

- [ ] **Step 5: Run locally**

```bash
kind create cluster --name mgtt-provider-kubernetes-it
go test -tags=integration -count=1 ./test/integration/...
```

- [ ] **Step 6: Commit**

```bash
git add test/integration/
git commit -m "test(integration): cover workloads/networking/storage groups on kind"
```

## Task 9.2: Add ErrNotFound integration test

- [ ] **Step 1**: Probe a deployment that doesn't exist; assert `status: not_found`, exit 0, `value: null`.

- [ ] **Step 2**: Commit.

---

# PHASE 10: Install Hook Hardening

## Task 10.1: Detect Go absence + version gate

**Files:**
- Modify: `hooks/install.sh`

- [ ] **Step 1: Rewrite**

```bash
#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "error: 'go' is not installed. Install Go 1.21+ from https://go.dev/dl/" >&2
  exit 2
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/^go//')
MIN="1.21"
if [ "$(printf '%s\n%s\n' "$MIN" "$GO_VERSION" | sort -V | head -1)" != "$MIN" ]; then
  echo "error: go ${GO_VERSION} is too old; need ${MIN}+" >&2
  exit 2
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "warning: kubectl not on PATH — required at probe time, not install time" >&2
fi

mkdir -p bin
VERSION=$(cat VERSION)
go build -ldflags "-X main.Version=${VERSION}" -o bin/mgtt-provider-kubernetes .
echo "✓ built bin/mgtt-provider-kubernetes ${VERSION}"
```

- [ ] **Step 2: Smoke-test by running** — `bash hooks/install.sh`. Verify it still builds, with the new friendly messages.

- [ ] **Step 3: Commit**

```bash
git add hooks/install.sh
git commit -m "chore(install): gate on go version, warn on missing kubectl"
```

---

# PHASE 11: Release Automation

## Task 11.1: Add CHANGELOG.md

**Files:**
- Create: `CHANGELOG.md`

Use Keep-a-Changelog format. Seed with `## [Unreleased]` capturing everything in this plan, and a `## [2.0.0] — 2026-04-14` historical entry (the `b708e14` initial 37-type release).

- [ ] **Step 1: Write file, commit.**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG in keep-a-changelog format"
```

## Task 11.2: Add release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write**

```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        goos: [linux, darwin]
        goarch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          VERSION=$(cat VERSION)
          out="mgtt-provider-kubernetes-${{ matrix.goos }}-${{ matrix.goarch }}"
          go build -ldflags "-X main.Version=${VERSION}" -o "$out" .
          tar -czf "$out.tar.gz" "$out"
      - uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.goos }}-${{ matrix.goarch }}
          path: '*.tar.gz'

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
      - uses: softprops/action-gh-release@v2
        with:
          files: '**/*.tar.gz'
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: tag-triggered release builds — cross-platform binaries attached to GH release"
```

---

# PHASE 12: Final Sweep

## Task 12.1: Assert exempt map is empty

- [ ] **Step 1: Run parity test**

Run: `go test ./internal/probes/ -run Parity -v`
Expected: PASS. Inspect `parity_test.go` — the `exempt` map should be empty. If not, implement or justify each remaining entry.

## Task 12.2: README update

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Remove the "only 2 types implemented" caveat.** Replace with:

```markdown
## Runtime coverage

All 37 vocabulary types are now probed at runtime. Each type definition in `types/<type>.yaml` is backed by a Go ProbeFn in `internal/probes/<type>.go`. The parity test (`internal/probes/parity_test.go`) fails CI if coverage regresses.
```

- [ ] **Step 2: Add subcommand reference** (`version`, `doctor`).

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: README reflects full type coverage + new subcommands"
```

## Task 12.3: Tag v2.1.0

- [ ] **Step 1: Update VERSION**

```
2.1.0
```

- [ ] **Step 2: Update `manifest.yaml` meta.version to 2.1.0.**

- [ ] **Step 3: Update CHANGELOG** — move `[Unreleased]` contents to `[2.1.0] — 2026-04-15`.

- [ ] **Step 4: Commit**

```bash
git add VERSION manifest.yaml CHANGELOG.md
git commit -m "release: v2.1.0 — full runtime coverage, robustness, diagnostics"
```

- [ ] **Step 5: Tag and push**

```bash
git tag v2.1.0
git push origin main --tags
```

Release workflow builds cross-platform binaries and attaches to the GH release.

---

# Cross-Cutting Concerns Checklist

Run through these before declaring complete:

- [ ] **Security:** no provider code writes to the cluster. `grep -rE '(apply|create|delete|patch|edit|scale|replace)' internal/ main.go` should return only comments or test fixtures.
- [ ] **No new runtime deps:** `go list -m all` should show only standard library.
- [ ] **Race-free:** `go test -race ./...` passes.
- [ ] **Lint clean:** `gofmt -s -d .` empty, `go vet ./...` clean.
- [ ] **Binary size reasonable:** `ls -lh bin/mgtt-provider-kubernetes` should be <10 MB.
- [ ] **Debug output never corrupts stdout:** grep `os.Stderr` in debug/doctor code, not `os.Stdout`.
- [ ] **Every probe respects `req.Namespace`:** `grep -L 'req.Namespace' internal/probes/*.go` should be empty (except cluster-scoped types — document the exceptions inline with a one-liner).
- [ ] **Integration tests exercise at least one fact per type group.**
- [ ] **CHANGELOG, README, PROBE_CONTRACT.md all describe the same behavior.**
