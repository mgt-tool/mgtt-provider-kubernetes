package probes

import "testing"

func TestJSONInt_Normal(t *testing.T) {
	data := map[string]any{"status": map[string]any{"readyReplicas": float64(3)}}
	if got := JSONInt(data, "status", "readyReplicas"); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestJSONInt_Missing(t *testing.T) {
	if got := JSONInt(map[string]any{}, "x", "y"); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestJSONBool_TrueString(t *testing.T) {
	data := map[string]any{"status": map[string]any{"v": "True"}}
	if !JSONBool(data, "status", "v") {
		t.Fatal("expected true")
	}
}

func TestJSONBool_FalseString(t *testing.T) {
	data := map[string]any{"v": "False"}
	if JSONBool(data, "v") {
		t.Fatal("expected false")
	}
}

func TestJSONBool_NativeBool(t *testing.T) {
	if !JSONBool(map[string]any{"v": true}, "v") {
		t.Fatal("expected true")
	}
}

func TestJSONString(t *testing.T) {
	data := map[string]any{"spec": map[string]any{"class": "nginx"}}
	if got := JSONString(data, "spec", "class"); got != "nginx" {
		t.Fatalf("got %q", got)
	}
}

func TestCountList(t *testing.T) {
	data := map[string]any{"items": []any{1, 2, 3}}
	if CountList(data, "items") != 3 {
		t.Fatal("expected 3")
	}
}

func TestConditionStatus(t *testing.T) {
	data := map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Available", "status": "True"},
		map[string]any{"type": "Progressing", "status": "False"},
	}}}
	if !ConditionStatus(data, "Available") {
		t.Fatal("want Available=true")
	}
	if ConditionStatus(data, "Progressing") {
		t.Fatal("want Progressing=false")
	}
	if ConditionStatus(data, "Missing") {
		t.Fatal("missing condition must be false")
	}
}

func TestMaxRestartCount(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"status": map[string]any{"containerStatuses": []any{
			map[string]any{"restartCount": float64(5)},
			map[string]any{"restartCount": float64(2)},
		}}},
		map[string]any{"status": map[string]any{"containerStatuses": []any{
			map[string]any{"restartCount": float64(12)},
		}}},
	}}
	if got := MaxRestartCount(data); got != 12 {
		t.Fatalf("got %d", got)
	}
}

func TestMaxRestartCount_Empty(t *testing.T) {
	if MaxRestartCount(map[string]any{}) != 0 {
		t.Fatal("expected 0")
	}
}

func TestCountEndpointAddresses(t *testing.T) {
	data := map[string]any{"subsets": []any{
		map[string]any{"addresses": []any{map[string]any{}, map[string]any{}}},
		map[string]any{"addresses": []any{map[string]any{}}},
	}}
	if got := CountEndpointAddresses(data); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestAnyContainerReason(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"status": map[string]any{"containerStatuses": []any{
			map[string]any{"lastState": map[string]any{
				"terminated": map[string]any{"reason": "OOMKilled"},
			}},
		}}},
	}}
	if !AnyContainerReason(data, "OOMKilled") {
		t.Fatal("expected OOMKilled true")
	}
	if AnyContainerReason(data, "Error") {
		t.Fatal("expected Error false")
	}
}
