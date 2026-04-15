# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [2.2.0] — 2026-04-16

Runtime coverage expands to 20 of 37 vocabulary types (Tier 1 + Tier 2).

### Added

- **Runtime probes** for 10 new Tier-2 types:
  - Workloads: `replicaset`, `cronjob`, `job`
  - Networking: `networkpolicy`, `ingressclass`
  - Scaling: `pdb`
  - Prerequisites: `namespace`, `configmap`, `secret`, `serviceaccount`
- **`AgeSeconds(rfc3339)`** helper in `internal/probes/helpers.go` — parses creation timestamps and status timestamps (cronjob `lastScheduleTime`/`lastSuccessfulTime`, configmap/secret `age`).
- **`CountMapKeys`** helper — counts `data`/`binaryData` keys without emitting values.
- **`Exists(kind, scoped)`** helper — maps kubectl NotFound to `bool:false` so every Tier-2 type can expose an `exists` fact without surfacing errors to operators.
- **Secret probe guardrail**: a unit test asserts `Raw` never contains secret content — metadata-only contract locked by test.

### Deferred (Tier 3 — next cycle)

`role`, `clusterrole`, `rolebinding`, `clusterrolebinding`, `validatingwebhookconfiguration`, `mutatingwebhookconfiguration`, `customresourcedefinition`, `custom_resource`, `operator`, `priorityclass`, `lease`, `csidriver`, `volumeattachment`, `persistentvolume`, `storageclass`, `resourcequota`, `limitrange`.

## [2.1.0] — 2026-04-16

Consumes the mgtt SDK; expands runtime coverage to 10 Tier-1 types.

### Added

- **Runtime probes** for 8 new types (in addition to pre-existing `deployment` + `ingress`):
  `statefulset`, `daemonset`, `pod`, `service`, `endpoints`, `pvc`, `node`, `hpa`.
  Chosen per S4 triage: the highest-signal set for workload-path troubleshooting.
- **`internal/kubeclassify/`** — maps kubectl stderr phrasing (`NotFound`, `Forbidden`,
  `Unable to connect`, etc.) to the SDK's sentinel errors. The single place kubectl
  vocabulary lives in this repo.
- **CHANGELOG** and **VERSION** files.
- **GitHub Actions CI** — lint + unit on every push, integration (kind) on PRs to main.
- **Install hook hardening** — gates on Go 1.21+, warns if kubectl is absent at install time.

### Changed

- **Runtime replaced by SDK.** The old 187-line `main.go` shrinks to 13 lines; all
  plumbing (argv parsing, timeouts, size caps, exit codes, `status:not_found`
  translation, `version` subcommand) moves to `github.com/mgt-tool/mgtt/sdk/provider`.
- **Module path** → `github.com/mgt-tool/mgtt-provider-kubernetes`.
- **`requires: mgtt`** → `>=0.1.0` (SDK availability gate).

### Deferred

- Tier 2: `replicaset`, `cronjob`, `job`, `networkpolicy`, `ingressclass`, `pdb`,
  `configmap`, `secret`, `namespace`, `serviceaccount`, `role`, `clusterrole`,
  `rolebinding`, `clusterrolebinding`, `persistentvolume`, `storageclass`,
  `resourcequota`, `limitrange`.
- Tier 3: `csidriver`, `volumeattachment`, `validatingwebhookconfiguration`,
  `mutatingwebhookconfiguration`, `customresourcedefinition`, `custom_resource`,
  `operator`, `priorityclass`, `lease`.

Vocabulary for all deferred types remains in `types/*.yaml` so the engine can still reason about them; only runtime probing is deferred.

## [2.0.0] — 2026-04-14

Initial 37-type vocabulary release. Runtime probes covered `deployment` + `ingress`.
