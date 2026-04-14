# Integration tests

End-to-end tests that prove the provider works against a real Kubernetes cluster and is usable by `mgtt` running in docker.

## What they cover

1. **`TestProbeRunner_AgainstRealCluster`** — builds the provider binary, spins up a `kind` cluster, deploys an nginx Deployment, invokes the binary directly, and asserts the JSON it returns matches the cluster's actual `readyReplicas` / `desiredReplicas` / `restartCount`.

2. **`TestMgttDocker_ProviderInspect`** — stages the provider under a temp `$MGTT_HOME/providers/kubernetes/`, runs the `mgtt` docker image with that home bind-mounted, executes `mgtt provider inspect kubernetes`, and asserts the v2.0.0 vocabulary (37 types) is discovered. This proves `mgtt` can load a multi-file provider end-to-end.

3. **`TestMgttDocker_ModelValidate`** — validates a sample model that declares a `type: deployment` component, via `mgtt model validate` inside the docker image. Passing validation means `mgtt` loaded the kubernetes provider and resolved the type.

## Requirements

- `docker`, `kind`, `kubectl`, `go` on `$PATH`
- Network access to pull `ghcr.io/mgt-tool/mgtt:latest` (or override with `MGTT_IMAGE`)

## Run

```bash
go test -tags=integration ./test/integration/...
```

The test suite creates a kind cluster named `mgtt-provider-kubernetes-it` and reuses it across runs. Delete it manually when you're done:

```bash
kind delete cluster --name mgtt-provider-kubernetes-it
```

## Why these tests live here, not in mgtt

`mgtt` is a framework — it knows only about types, interfaces, and the provider protocol. It must not know about kubernetes specifically. Integration between this provider and `mgtt` is this repository's concern, so the end-to-end tests live here.
