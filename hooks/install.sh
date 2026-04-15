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
go build -ldflags "-X github.com/mgt-tool/mgtt/sdk/provider.Version=${VERSION}" -o bin/mgtt-provider-kubernetes .
echo "✓ built bin/mgtt-provider-kubernetes ${VERSION}"
