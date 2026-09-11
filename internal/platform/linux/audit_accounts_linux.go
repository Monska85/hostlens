package linux

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/contract"
)

func (c *Collector) auditAccounts(ctx context.Context, r *contract.Result, a contract.Args) {
	r.Source = "/etc/passwd,/etc/group"
	r.Data["scope"] = "local account files; excludes directory services, effective sudo/SSH authorization and password state"
	r.Data["snapshot_consistent"] = false
	items := []map[string]any{}
	successfulSources := 0
	remainingMembers := auditMaxEntries
	for _, path := range []string{"/etc/passwd", "/etc/group"} {
		if len(items) >= auditMaxEntries {
			r.Truncated = true
			auditIssue(r, "inspection_limit", path, "account record limit reached")
			break
		}
		if !c.auditSystemContinue(ctx, r, path) {
			break
		}
		b, ok := c.auditRead(r, path)
		if !ok {
			continue
		}
		successfulSources++
		malformed := false
		for line := range strings.SplitSeq(string(b), "\n") {
			if ctx.Err() != nil {
				r.Truncated = true
				auditIssue(r, "cancelled_or_timeout", path, "account collection interrupted")
				break
			}
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(items) >= auditMaxEntries {
				r.Truncated = true
				auditIssue(r, "inspection_limit", path, "account record limit reached")
				break
			}
			fields := strings.SplitN(line, ":", 8)
			want := 7
			if path == "/etc/group" {
				want = 4
			}
			if len(fields) != want || fields[0] == "" || !utf8.ValidString(line) {
				malformed = true
				continue
			}
			id, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				malformed = true
				continue
			}
			row := map[string]any{"name": fields[0]}
			if want == 7 {
				gid, err := strconv.ParseUint(fields[3], 10, 32)
				if err != nil {
					malformed = true
					continue
				}
				row["kind"] = "account"
				row["uid"] = id
				row["gid"] = gid
				row["home"] = fields[5]
				row["shell"] = fields[6]
			} else {
				row["kind"] = "group"
				row["gid"] = id
				members := []string{}
				if fields[3] != "" {
					members = strings.SplitN(fields[3], ",", remainingMembers+1)
					if len(members) > remainingMembers {
						members = members[:remainingMembers]
						r.Truncated = true
						auditIssue(r, "inspection_limit", path, "group membership limit reached")
					}
				}
				remainingMembers -= len(members)
				row["members"] = members
			}
			items = append(items, row)
		}
		if malformed {
			auditIssue(r, "malformed_source", path, "invalid local identity records omitted")
		}
	}
	if successfulSources == 0 {
		r.Error = true
	}
	page(r, items, a, c.Config.Limits.PageSize)
	auditIssue(r, "evidence_unavailable", "accounts", "password lock/expiry state, external identities and effective sudo/SSH authorization are not collected; approve selected configuration sources for investigation")
}

func (c *Collector) auditSystemContinue(ctx context.Context, r *contract.Result, source string) bool {
	if ctx.Err() != nil {
		r.Truncated = true
		auditIssue(r, "cancelled_or_timeout", source, "audit collection interrupted")
		return false
	}
	if c.auditBudgetExhausted() {
		r.Truncated = true
		auditIssue(r, "inspection_limit", source, "aggregate audit inspection limit reached")
		return false
	}
	return true
}
func (c *Collector) auditSystemRead(ctx context.Context, r *contract.Result, source string) ([]byte, bool) {
	if !c.auditSystemContinue(ctx, r, source) {
		return nil, false
	}
	return c.auditRead(r, source)
}
