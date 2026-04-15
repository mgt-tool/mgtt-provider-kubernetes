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
	}
	for _, msg := range cases {
		err := Classify(msg, errors.New("exit status 1"))
		if !errors.Is(err, provider.ErrTransient) {
			t.Errorf("stderr %q: want ErrTransient, got %v", msg, err)
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
