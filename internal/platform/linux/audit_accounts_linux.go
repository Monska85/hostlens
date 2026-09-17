package linux

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/contract"
)

func (c *Collector) auditAccounts(ctx context.Context, r *contract.Result, a contract.PageArgs) bool {
	p := r.Data.(*contract.AccountPage)
	r.Source = "/etc/passwd,/etc/group"
	p.Scope = "local account files; excludes directory services, effective sudo/SSH authorization and password state"
	p.SnapshotConsistent = false
	rows := []contract.AccountRow{}
	successfulSources := 0
	remainingMembers := auditMaxEntries
	for _, path := range []string{"/etc/passwd", "/etc/group"} {
		if len(rows) >= auditMaxEntries {
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
			if len(rows) >= auditMaxEntries {
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
			if want == 7 {
				gid, err := strconv.ParseUint(fields[3], 10, 32)
				if err != nil {
					malformed = true
					continue
				}
				uid := uint32(id)
				accountGID := uint32(gid)
				home := fields[5]
				shell := fields[6]
				rows = append(rows, contract.AccountRow{Name: fields[0], Kind: "account", UID: &uid, GID: &accountGID, Home: &home, Shell: &shell})
			} else {
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
				groupGID := uint32(id)
				rows = append(rows, contract.AccountRow{Name: fields[0], Kind: "group", GID: &groupGID, Members: members})
			}
		}
		if malformed {
			auditIssue(r, "malformed_source", path, "invalid local identity records omitted")
		}
	}
	if successfulSources == 0 {
		r.Error = true
	}
	window, next, ok := pageWindow(r, rows, a, c.Config.Limits.PageSize)
	if ok {
		p.Items = window
		r.NextOffset = next
	}
	auditIssue(r, "evidence_unavailable", "accounts", "password lock/expiry state, external identities and effective sudo/SSH authorization are not collected; approve selected configuration sources for investigation")
	return ok
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
