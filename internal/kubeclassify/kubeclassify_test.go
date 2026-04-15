package kubeclassify

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/mgt-tool/mgtt/sdk/provider"
)

func TestClassify_NotFound(t *testing.T) {
	err := Classify(`Error from server (NotFound): deployments.apps "x" not found`,
		errors.New("exit status 1"))
	if !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestClassify_Forbidden(t *testing.T) {
	err := Classify(`Error from server (Forbidden): pods is forbidden: User "x"`,
		errors.New("exit status 1"))
	if !errors.Is(err, provider.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestClassify_Transient(t *testing.T) {
	cases := []string{
		"Unable to connect to the server: dial tcp ...",
		"i/o timeout",
		"context deadline exceeded",
		"connection refused",
		"dial tcp: lookup kubernetes.default.svc: no such host",
		"TLS handshake timeout",
		"connection reset by peer",
		"Server returned HTTP status 503 Service Unavailable",
	}
	for _, msg := range cases {
		err := Classify(msg, errors.New("exit status 1"))
		if !errors.Is(err, provider.ErrTransient) {
			t.Errorf("stderr %q: want ErrTransient, got %v", msg, err)
		}
	}
}

func TestClassify_NotFoundVariants(t *testing.T) {
	cases := []string{
		`Error from server (NotFound): deployments.apps "x" not found`,
		`error: the server doesn't have a resource type "widgets"`,
		`Error from server (NotFound): the server could not find the requested resource`,
	}
	for _, msg := range cases {
		err := Classify(msg, errors.New("exit status 1"))
		if !errors.Is(err, provider.ErrNotFound) {
			t.Errorf("stderr %q: want ErrNotFound, got %v", msg, err)
		}
	}
}

func TestClassify_ForbiddenVariants(t *testing.T) {
	cases := []string{
		`Error from server (Forbidden): pods is forbidden: User "x"`,
		`error: You must be logged in to the server (Unauthorized)`,
		`Error from server (Forbidden): User "x" cannot list resource "nodes"`,
	}
	for _, msg := range cases {
		err := Classify(msg, errors.New("exit status 1"))
		if !errors.Is(err, provider.ErrForbidden) {
			t.Errorf("stderr %q: want ErrForbidden, got %v", msg, err)
		}
	}
}

func TestClassify_BinaryMissingFallsThroughToEnv(t *testing.T) {
	err := Classify("", &exec.Error{Name: "kubectl", Err: exec.ErrNotFound})
	if !errors.Is(err, provider.ErrEnv) {
		t.Fatalf("want ErrEnv, got %v", err)
	}
}

func TestClassify_FallsThroughToEnv(t *testing.T) {
	err := Classify("some unexpected message", errors.New("exit status 1"))
	if !errors.Is(err, provider.ErrEnv) {
		t.Fatalf("want ErrEnv fallthrough, got %v", err)
	}
}

func TestClassify_NoErrorReturnsNil(t *testing.T) {
	if err := Classify("whatever", nil); err != nil {
		t.Fatalf("nil runErr must yield nil, got %v", err)
	}
}
