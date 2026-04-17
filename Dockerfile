# Multi-stage build for mgtt-provider-kubernetes.
# The provider binary shells out to `kubectl`, so the runtime image must
# include it. The `bitnami/kubectl` image is a kubectl-only distroless-ish
# image; we stage its binary onto a minimal alpine runtime so apk remains
# available for future additions if needed.
#
# NOTE: mgtt's image runner (as of mgtt v0.1.x) does not forward env or
# mounts into the container, so `KUBECONFIG`/service-account tokens are not
# automatically visible inside. Operators using `--image` must run mgtt in
# an environment where the container can reach the cluster (e.g., on an
# in-cluster pod with a service account, or manually wrap with `docker run
# -v $HOME/.kube:/root/.kube -e KUBECONFIG=/root/.kube/config`). Git install
# has no such limitation.

FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/provider .

FROM alpine:3.20
ARG KUBECTL_VERSION=v1.30.5
RUN apk add --no-cache ca-certificates curl \
 && curl -sSL -o /usr/local/bin/kubectl \
      "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
 && chmod +x /usr/local/bin/kubectl \
 && kubectl version --client
COPY --from=build /out/provider /usr/local/bin/provider
COPY manifest.yaml /manifest.yaml
COPY types /types
ENTRYPOINT ["/usr/local/bin/provider"]
