package linux

import (
	"context"
	"errors"
	"io"
	"net/mail"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
)

func (c *Collector) auditUpdates(ctx context.Context, r *contract.Result) {
	r.Source = "local package metadata"
	r.Data["scope"] = "cached repository metadata only; no refresh, upgrade or vulnerability assessment"
	metadata := []map[string]any{}

	for _, dir := range []string{"/var/lib/apt/lists", "/var/lib/pacman/sync"} {
		if !c.auditSystemContinue(ctx, r, dir) {
			break
		}
		d, err := c.auditDirectory(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			code := auditErrorCode(err)
			auditIssue(r, code, dir, "repository metadata directory unavailable")
			continue
		}
		entries, err := d.ReadDir(257)
		d.Close()
		if err != nil && err != io.EOF {
			auditIssue(r, "collection_failed", dir, "repository metadata enumeration failed")
			continue
		}
		if len(entries) > 256 {
			entries = entries[:256]
			r.Truncated = true
			auditIssue(r, "inspection_limit", dir, "repository metadata entry limit reached")
		}
		for _, entry := range entries {
			if !c.auditSystemContinue(ctx, r, dir) {
				break
			}
			name := entry.Name()
			apt := dir == "/var/lib/apt/lists"
			if entry.IsDir() || apt && !(strings.HasSuffix(name, "_InRelease") || strings.HasSuffix(name, "_Release")) || !apt && !strings.HasSuffix(name, ".db") {
				continue
			}
			source := path.Join(dir, name)
			f, err := openObservation(c.Root, source, c.Policy, true)
			if err != nil {
				code := auditErrorCode(err)
				auditIssue(r, code, source, "repository metadata source unavailable")
				continue
			}
			st, err := f.Stat()
			if err != nil {
				f.Close()
				auditIssue(r, "collection_failed", source, "metadata timestamp unavailable")
				continue
			}
			row := map[string]any{"source": source, "modified_at": st.ModTime().UTC(), "age_seconds": r.ObservedAt.Sub(st.ModTime()).Seconds()}
			if apt {
				b, err := c.readAuditBounded(f)
				f.Close()
				if err != nil {
					auditIssue(r, auditErrorCode(err), source, "repository metadata read failed or exceeds remaining inspection budget")
					r.Truncated = true
					break
				}
				for line := range strings.SplitSeq(string(b), "\n") {
					key, value, ok := strings.Cut(line, ":")
					if !ok || (key != "Date" && key != "Valid-Until") {
						continue
					}
					date, err := mail.ParseDate(strings.TrimSpace(value))
					if err != nil {
						auditIssue(r, "malformed_source", source, "invalid repository metadata date")
						continue
					}
					if key == "Date" {
						row["published_at"] = date.UTC()
					} else {
						row["valid_until"] = date.UTC()
						row["expired"] = r.ObservedAt.After(date)
					}
				}
				if row["published_at"] == nil {
					auditIssue(r, "malformed_source", source, "repository metadata lacks publication date")
				}
				if expired, _ := row["expired"].(bool); expired {
					auditIssue(r, "stale_metadata", source, "cached repository metadata has expired")
				}
			} else {
				f.Close()
			}
			if st.ModTime().After(r.ObservedAt.Add(time.Minute)) {
				auditIssue(r, "malformed_source", source, "metadata timestamp is in the future")
			}
			metadata = append(metadata, row)
		}
	}
	r.Data["repository_metadata"] = metadata
	if len(metadata) == 0 {
		r.Error = true
		auditIssue(r, "source_unavailable", "updates", "no supported local repository metadata was observed")
	}
	auditIssue(r, "evidence_unavailable", "updates", "cache timestamps do not prove successful refresh or signature verification; upgrade candidates, enabled repository configuration and reboot requirements are not established")
}
