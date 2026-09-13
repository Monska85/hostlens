package observerapp

import (
	"strings"
	"testing"
)

func TestEngineErrorMessagePrefersDaemonPayload(t *testing.T) {
	if got := engineErrorMessage(strings.NewReader(`{"message":"no such container"}`)); got != "no such container" {
		t.Fatalf("daemon message lost: %q", got)
	}
}

func TestEngineErrorMessageFallsBackWithoutJSON(t *testing.T) {
	for _, body := range []string{"", "plain text refusal", `{"other":"field"}`, "{not json"} {
		if got := engineErrorMessage(strings.NewReader(body)); got != "engine response had no usable message" {
			t.Fatalf("fallback missing for %q: %q", body, got)
		}
	}
}

func TestEngineErrorMessageBoundsOversizedPayload(t *testing.T) {
	huge := `{"message":"` + string(make([]byte, 4096)) + `"}`
	if got := engineErrorMessage(strings.NewReader(huge)); got == "" {
		t.Fatal("bounded read returned nothing")
	}
}

func TestNotFoundMarksMissingResources(t *testing.T) {
	response := notFound(errEngineNotFound)
	if !response.Failed || response.Issue != "not_found" {
		t.Fatalf("missing resource must carry the not_found issue: %+v", response)
	}
	other := notFound(errCeiling)
	if !other.Failed || other.Issue != "" {
		t.Fatalf("non-not-found errors must stay generic failures: %+v", other)
	}
}
