package policy

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/bmatcuk/doublestar/v4"
)

const LinuxEvaluator = "linux-no-links-v3"

var unitRE = regexp.MustCompile(`^[A-Za-z0-9_@:.\\-]+\.(service|socket|timer|target|mount|automount|path|slice|scope|device|swap)$`)

func ValidLinuxUnit(s string) bool {
	return s == "-.mount" || len(s) <= 256 && unitRE.MatchString(s) && !strings.HasPrefix(s, "-")
}

func validLinuxPattern(category, s string) bool {
	if len(s) > 4096 {
		return false
	}
	if category == "audit" {
		switch s {
		case "*", "processes", "network", "accounts", "storage", "updates", "security", "services", "hostlens", "paths":
			return true
		}
		return false
	}
	if category == "files" {
		return strings.HasPrefix(s, "/") && path.Clean(s) == s && doublestar.ValidatePattern(s)
	}
	if category == "docker" {
		return validDockerPattern(s)
	}
	return !strings.ContainsAny(s, "/ =\n\r") && doublestar.ValidatePattern(s) && s != ""
}

// dockerCollections and dockerItems define the canonical Docker resource
// forms. Collection forms grant one inventory or aggregate observation; item
// forms scope stable-ID and name matching for one resource class. Stats and
// logs remain separate authorities from container metadata.
var dockerCollections = map[string]bool{
	"daemon": true, "containers": true, "images": true,
	"volumes": true, "networks": true, "disk_usage": true,
}
var dockerItems = map[string]bool{
	"container": true, "stats": true, "logs": true,
	"image": true, "volume": true, "network": true,
}

func validDockerPattern(s string) bool {
	if s == "*" {
		return true
	}
	if dockerCollections[s] {
		return true
	}
	kind, selector, ok := strings.Cut(s, "/")
	if !ok || !dockerItems[kind] || selector == "" || strings.ContainsAny(selector, " =\n\r") || len(selector) > 4096 {
		return false
	}
	// The kind boundary is the only slash; nested or traversing selectors
	// never match a stable identity or name.
	if strings.Contains(selector, "/") || strings.Contains(selector, "..") {
		return false
	}
	return doublestar.ValidatePattern(selector)
}

// DockerKinds enumerates the collection and item classes that active allow
// rules in the docker category cover. "*" grants every class.
func DockerKinds(rules []string) []string {
	seen := map[string]bool{}
	for _, pattern := range rules {
		if pattern == "*" {
			for kind := range dockerCollections {
				seen[kind] = true
			}
			for kind := range dockerItems {
				seen[kind] = true
			}
			continue
		}
		kind, _, ok := strings.Cut(pattern, "/")
		if ok && dockerItems[kind] {
			seen[kind] = true
		} else if dockerCollections[pattern] {
			seen[pattern] = true
		}
	}
	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

func linuxMatches(r Rule, target string) bool {
	if r.Literal {
		return r.Pattern == target
	}
	if r.Category == "docker" && r.Pattern == "*" {
		// The bare wildcard grants every Docker resource class; glob
		// semantics would stop at the kind boundary.
		return true
	}
	matched, _ := doublestar.Match(r.Pattern, target)
	return matched
}

func linuxBuiltinExempt(r Rule) bool {
	return r.Mandatory && !r.Literal && (r.Pattern == "/proc/**" || r.Pattern == "/sys/**" || r.Pattern == "/dev/**")
}

// CompileLinux binds native source grammar and exclusions to shared provenance
// and denial evaluation. It is also used by tools running on other build hosts.
func CompileLinux(c config.Config, source string, defs map[string]Definition) (*Policy, error) {
	var mandatory []Rule
	for _, name := range []string{source, c.TokenStore, c.TokenStore + ".lock", c.Server.TLS.KeyFile} {
		if name != "" {
			mandatory = append(mandatory, Rule{Category: "files", Pattern: name, Deny: true, Source: "internal", Mandatory: true, Literal: true})
		}
	}
	for _, definition := range defs {
		mandatory = append(mandatory, Rule{Category: "files", Pattern: definition.Source, Deny: true, Source: "internal", Mandatory: true, Literal: true})
	}
	for _, pattern := range []string{"/proc/**", "/sys/**", "/dev/**"} {
		mandatory = append(mandatory, Rule{Category: "files", Pattern: pattern, Deny: true, Source: "internal", Mandatory: true})
	}
	native := semantics{
		identity: LinuxEvaluator,
		categories: func(r config.Rules) map[string][]string {
			return map[string][]string{"files": r.Files, "journal": r.Journal, "audit": r.Audit, "docker": r.Docker}
		},
		validPattern:  validLinuxPattern,
		matches:       linuxMatches,
		builtinExempt: linuxBuiltinExempt,
	}
	return compile(c.Profile, source, defs, native, mandatory)
}
