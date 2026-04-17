# Tier-2 Runtime Coverage — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand runtime probe coverage from the Tier-1 10 types (shipped in v2.1.0) to the Tier-2 10 types, bringing kubernetes-provider coverage to 20 of 37 vocabulary types.

**Architecture:** Identical to Tier-1. Each type is one file under `internal/probes/<type>.go`, wired via `registerX(r, c)` from `helpers.go:Register`. All plumbing stays in the mgtt SDK; kubectl stderr classification stays in `internal/kubeclassify/`. No architectural changes.

**Tech Stack:** Go 1.25+, `github.com/mgt-tool/mgtt/sdk/provider`, `github.com/mgt-tool/mgtt/sdk/provider/shell`, kubectl.

---

## Scope

**In scope — Tier 2 (10 types):**

| Group | Types |
|---|---|
| Workloads | `replicaset`, `cronjob`, `job` |
| Networking | `networkpolicy`, `ingressclass` |
| Scaling | `pdb` |
| Prerequisites | `namespace`, `configmap`, `secret`, `serviceaccount` |

**Explicitly NOT in scope (deferred to Tier 3 — separate future plan):**

RBAC (`role`, `clusterrole`, `rolebinding`, `clusterrolebinding`), webhooks, CSI, leases, priorityclass, extensibility (`customresourcedefinition`, `custom_resource`, `operator`), cluster-quota (`resourcequota`, `limitrange`, `persistentvolume`, `storageclass`). Rationale: lower troubleshooting signal; engine can still reason about them via the vocabulary YAMLs without a runtime probe.

## Approach

Copy the Tier-1 patterns verbatim:

- One `internal/probes/<type>.go` per type.
- Reuse `helpers.go:JSONInt`, `JSONBool`, `JSONString`, `ConditionStatus`, `CountList`, `MaxRestartCount`, etc. Extend helpers only if a Tier-2 type needs a shape not yet seen (e.g. `cronjob.last_successful_time` wants an `age_seconds` helper that parses RFC3339 and subtracts from `time.Now()`).
- Register each type in `helpers.go:Register` alongside the Tier-1 registrations.
- Unit tests per fact using the same fake-client pattern as `deployment_test.go`.
- No changes to `main.go`, `manifest.yaml`, or the SDK.

Commit per type — 10 commits total — so the history is bisectable if one type regresses.

---

## Task Template (apply once per type)

For each type in Tier 2:

**Files:**
- Create: `internal/probes/<type>.go`
- Create: `internal/probes/<type>_test.go`
- Modify: `internal/probes/helpers.go` — append `register<Type>(r, c)` to `Register`.

- [ ] **Step 1: Read the vocabulary YAML.** Confirm the declared facts and their types.

  ```bash
  grep -E '^  [a-z_]+:$' types/<type>.yaml
  ```

- [ ] **Step 2: Write tests first.** One test per fact. Use the fake-client pattern:

  ```go
  func TestX_Fact(t *testing.T) {
      c := fakeKubectl(map[string]map[string]any{
          "<kind>/<name>": { /* minimal JSON shape */ },
      })
      r := provider.NewRegistry()
      register<Type>(r, c)
      got, err := r.Probe(ctx, provider.Request{Type: "<type>", Name: "...", Fact: "..."})
      // assertions
  }
  ```

  If no `fakeKubectl` helper exists yet (Tier-1 tests didn't need one because helpers are tested directly), create one in `helpers_test.go` that satisfies `*shell.Client` by injecting `Exec`.

- [ ] **Step 3: Run — expect FAIL** (function not yet registered).

- [ ] **Step 4: Implement `internal/probes/<type>.go`.** Follow the pattern of `deployment.go` or `pod.go` depending on whether the resource is namespaced (all Tier-2 workloads/prereqs are) or cluster-scoped (none in Tier 2).

- [ ] **Step 5: Wire `register<Type>(r, c)` into `helpers.go:Register`.**

- [ ] **Step 6: Run — expect PASS.**

  ```bash
  go test ./internal/probes/... -count=1
  ```

- [ ] **Step 7: Commit.**

  ```bash
  git add internal/probes/<type>.go internal/probes/<type>_test.go internal/probes/helpers.go
  git commit -m "feat(probes/<type>): implement N facts via registry"
  ```

---

## Per-Type Notes

### `replicaset`

Kind: `replicaset`. Mostly mirrors deployment's status fields. Facts: `ready_replicas`, `desired_replicas`, `available_replicas`, `restart_count`. Get pod restart count via `-l app=<name>` same as deployment — if the vocabulary uses a different label selector, read from the YAML.

### `cronjob`

Kind: `cronjob`. Facts likely include `last_schedule_time`, `last_successful_time`, `active_count`, `suspend`. Time fields are RFC3339 strings — for "age in seconds" add:

```go
// helpers.go
func AgeSeconds(rfc3339 string) int {
    t, err := time.Parse(time.RFC3339, rfc3339)
    if err != nil || t.IsZero() {
        return 0
    }
    return int(time.Since(t).Seconds())
}
```

Register once in `helpers.go`; reuse from other time-based facts.

### `job`

Kind: `job`. Facts: `active`, `succeeded`, `failed`, `completion_time`. Straightforward int + timestamp.

### `networkpolicy`

Kind: `networkpolicy`. Facts (low-signal — may all be cheap counts): `ingress_rule_count`, `egress_rule_count`, `pod_selector_empty`. Use `CountList` on `spec.ingress`, `spec.egress`, and check whether `spec.podSelector.matchLabels` is non-empty.

### `ingressclass`

Kind: `ingressclass` (cluster-scoped — drop the `-n` flag). Facts: `controller`, `is_default`. `is_default` reads the `ingressclass.kubernetes.io/is-default-class` annotation.

### `pdb`

Kind: `pdb`. Facts: `min_available`, `max_unavailable`, `disruptions_allowed`, `current_healthy`, `desired_healthy`. Mix of int spec fields and int status fields.

### `namespace`

Kind: `namespace` (cluster-scoped). Facts: `phase` (`Active`/`Terminating`), `age_seconds`. Simple string + time arithmetic.

### `configmap` / `secret`

Same shape. Facts: `key_count` (len of `data`), `age_seconds`. **Do NOT expose secret values** — only metadata. The provider is read-only, but even reads should avoid exfiltrating secret contents to the trace output. If a fact is declared that would include a value, document why it's declared but not implemented (leave it in the exempt map per `mgtt provider validate`).

### `serviceaccount`

Kind: `serviceaccount`. Facts: `secret_count`, `automount_token`. Straightforward.

---

## Verification

After all 10 types land:

- [ ] `go test -race ./...` passes.
- [ ] `go vet ./...` clean.
- [ ] `gofmt -s -d .` empty.
- [ ] Binary size `<10 MB` — no new runtime deps.
- [ ] `grep -rni --include='*.go' --exclude-dir=testdata -E '\b(kubectl|kubernetes)\b' internal/probes/` returns only kubectl call-sites. No provider logic names other backends.
- [ ] `mgtt provider validate kubernetes` from a fresh install passes static checks.
- [ ] Release: bump VERSION to `2.2.0`, update CHANGELOG with Tier-2 additions, tag `v2.2.0`.

## Non-goals

- No Tier 3 types. RBAC, webhooks, CSI, etc. stay deferred — their probes are low-signal for operators and not worth the implementation cost in this cycle.
- No architectural changes. The SDK contract, `internal/kubeclassify/`, and `main.go` stay untouched.
- No CI changes — existing lint/unit/integration workflow already covers Tier-2 probes.
- No integration-test fixture expansion in this plan. Live validation against a real kind cluster can be added as a separate follow-up once the Tier-2 types settle.
