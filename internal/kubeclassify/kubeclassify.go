// Package kubeclassify maps kubectl stderr phrasing to the provider SDK's
// sentinel errors. This is the one place in the provider that encodes
// kubectl-specific vocabulary — everything else consumes the SDK's
// backend-agnostic helpers.
package kubeclassify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// Classify is a shell.ClassifyFn for kubectl. Pass to shell.Client.Classify.
//
// Real kubectl stderr varies by version, resource, and API group. This
// classifier tries to catch the common shapes across recent (1.24+) kubectl
// plus the "the server doesn't have a resource type" branch that fires when
// a CRD is not installed or a resource name is misspelled.
func Classify(stderr string, runErr error) error {
	if runErr == nil {
		return nil
	}
	// Binary missing: delegate to the SDK default (backend-agnostic case).
	if errors.Is(runErr, exec.ErrNotFound) {
		return shell.EnvOnlyClassify(stderr, runErr)
	}
	first := firstLine(stderr)
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(stderr, "NotFound"),
		strings.Contains(lower, "the server doesn't have a resource type"),
		strings.Contains(lower, "the server could not find the requested resource"):
		return fmt.Errorf("%w: %s", provider.ErrNotFound, first)

	case strings.Contains(stderr, "Forbidden"),
		strings.Contains(lower, "forbidden"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "you must be logged in"),
		strings.Contains(lower, "cannot list resource"),
		strings.Contains(lower, "cannot get resource"):
		return fmt.Errorf("%w: %s", provider.ErrForbidden, first)

	case strings.Contains(stderr, "Unable to connect"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "connection reset"),
		strings.Contains(lower, "tls handshake timeout"),
		strings.Contains(lower, "server returned http status 5"):
		return fmt.Errorf("%w: %s", provider.ErrTransient, first)
	}
	return fmt.Errorf("%w: %s", provider.ErrEnv, first)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
