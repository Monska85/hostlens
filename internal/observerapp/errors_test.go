package observerapp

import (
	"testing"
)

func TestNotFoundMarksMissingResources(t *testing.T) {
	t.Parallel()

	response := notFound(errEngineNotFound)
	if !response.Failed || response.Issue != "not_found" {
		t.Fatalf("missing resource must carry the not_found issue: %+v", response)
	}
	other := notFound(errCeiling)
	if !other.Failed || other.Issue != "" {
		t.Fatalf("non-not-found errors must stay generic failures: %+v", other)
	}
}
