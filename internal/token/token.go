package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
)

type Record struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Roles   []string  `json:"roles"`
	Expires time.Time `json:"expires"`
	Status  string    `json:"status"`
	Hash    string    `json:"hash,omitempty"`
}
type Verifier interface {
	Verify(string) (Record, error)
}

func Random(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func RolesOK(roles []string) bool {
	if len(roles) == 0 {
		return false
	}
	for _, r := range roles {
		if !slices.Contains([]string{"health", "inspect", "diagnostics", "metrics"}, r) {
			return false
		}
	}
	return true
}
func Allows(roles []string, tool string) bool {
	level := 0
	for _, r := range roles {
		switch r {
		case "health":
			level = max(level, 1)
		case "inspect":
			level = max(level, 2)
		case "diagnostics":
			level = 3
		}
	}
	if contract.AuditDomain(tool) != "" {
		return level >= 3
	}
	switch tool {
	case "get_os_info", "get_inventory", "get_health_snapshot":
		return level >= 1
	case "list_services", "get_service_status", "list_packages":
		return level >= 2
	case "read_config", "query_logs":
		return level >= 3
	}
	return false
}
