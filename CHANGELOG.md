# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [2.3.0] — 2026-04-16

**Full 37-of-37 vocabulary coverage.** All types declared in `types/*.yaml` now have runtime probe implementations.

### Added

- **Runtime probes** for the 17 Tier-3 types:
  - RBAC: `role`, `clusterrole`, `rolebinding`, `clusterrolebinding`
  - Webhooks: `validatingwebhookconfiguration`, `mutatingwebhookconfiguration`
  - Extensibility: `customresourcedefinition`, `custom_resource`, `operator`, `priorityclass`, `lease`
  - Storage (cluster-scoped): `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment`
  - Quotas: `resourcequota`, `limitrange`
- **Dangling-binding detection**: `rolebinding` and `clusterrolebinding` expose a `role_ref_exists` fact that
  probes whether the referenced Role/ClusterRole is still present. A binding pointing at a missing role is a
  silent RBAC failure that's hard to find via `kubectl describe`.
- **Webhook backend reachability**: `validatingwebhookconfiguration` and `mutatingwebhookconfiguration`
  expose a `service_available` fact that resolves `clientConfig.service.{name, namespace}` for every
  webhook and probes the target Service. URL-based hooks are treated as reachable (we can't probe
  arbitrary external URLs from read-only kubectl).
- **Generic CRD instance probing**: `custom_resource` accepts either `--resource <plural.group>` or
  `--kind <Kind> [--api <group/version>]` via `Command.Extra` and probes conditions on any CRD instance
  without needing a per-CRD provider.
- **Composite operator type**: `operator` composes existing primitives (deployment health, CRD presence,
  webhook backend reachability) into a single-component view of an operator deployment. Accepts optional
  `--crd` and `--webhook` flags.
- **CSI driver node-readiness**: `csidriver.node_count` returns the number of CSINode objects that list
  this driver — a proxy for "how many nodes can mount volumes provisioned by this CSI".

### Tests

- `internal/probes/tier3_test.go` exercises the non-trivial composite probes (rolebinding→role resolution,
  webhook→service resolution, custom_resource flag parsing, CSIDriver node counting) using a
  `multiFakeKubectl` router that returns distinct responses per kubectl subcommand.

### Coverage summary

| Tier | Types | Status |
|---|---|---|
| Tier 1 (v2.1.0) | deployment, ingress, pod, service, endpoints, statefulset, daemonset, pvc, node, hpa | ✅ |
| Tier 2 (v2.2.0) | replicaset, cronjob, job, networkpolicy, ingressclass, pdb, namespace, configmap, secret, serviceaccount | ✅ |
| Tier 3 (v2.3.0) | role, clusterrole, rolebinding, clusterrolebinding, validatingwebhookconfiguration, mutatingwebhookconfiguration, customresourcedefinition, custom_resource, operator, priorityclass, lease, persistentvolume, storageclass, csidriver, volumeattachment, resourcequota, limitrange | ✅ |

**37 / 37.**

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
