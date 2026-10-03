package app

import (
	"strings"
	"testing"
)

func TestTokenLifetimeMustBeExplicit(t *testing.T) {
	for _, args := range [][]string{{"token", "create", "--name", "example"}, {"token", "rotate", "--id", "old", "--overlap-until", "2027-01-01T00:00:00Z"}} {
		err := Main(args)
		if err == nil || !strings.Contains(err.Error(), "expires") {
			t.Fatalf("%v: expected required expiry, got %v", args, err)
		}
	}
}
