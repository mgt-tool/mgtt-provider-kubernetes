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
func Classify(stderr string, runErr error) error {
	if runErr == nil {
		return nil
	}
	// Binary missing: delegate to the SDK default (backend-agnostic case).
	if errors.Is(runErr, exec.ErrNotFound) {
		return shell.EnvOnlyClassify(stderr, runErr)
	}
	first := firstLine(stderr)
	switch {
	case strings.Contains(stderr, "NotFound"):
		return fmt.Errorf("%w: %s", provider.ErrNotFound, first)
	case strings.Contains(stderr, "Forbidden"), strings.Contains(stderr, "forbidden"):
		return fmt.Errorf("%w: %s", provider.ErrForbidden, first)
	case strings.Contains(stderr, "Unable to connect"),
		strings.Contains(stderr, "i/o timeout"),
		strings.Contains(stderr, "context deadline exceeded"),
		strings.Contains(stderr, "connection refused"):
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
