package policy

import (
	"path"
	"regexp"
	"strings"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/bmatcuk/doublestar/v4"
)

const LinuxEvaluator = "linux-no-links-v2"

var unitRE = regexp.MustCompile(`^[A-Za-z0-9_@:.\\-]+\.(service|socket|timer|target|mount|automount|path|slice|scope)$`)

func ValidLinuxUnit(s string) bool {
	return len(s) <= 256 && unitRE.MatchString(s) && !strings.HasPrefix(s, "-")
}

func validLinuxPattern(category, s string) bool {
	if len(s) > 4096 {
		return false
	}
	if category == "files" {
		return strings.HasPrefix(s, "/") && path.Clean(s) == s && doublestar.ValidatePattern(s)
	}
	return !strings.ContainsAny(s, "/ =\n\r") && doublestar.ValidatePattern(s) && s != ""
}

func linuxMatches(r Rule, target string) bool {
	if r.Literal {
		return r.Pattern == target
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
			return map[string][]string{"files": r.Files, "journal": r.Journal}
		},
		validPattern:  validLinuxPattern,
		matches:       linuxMatches,
		builtinExempt: linuxBuiltinExempt,
	}
	return compile(c.Profile, source, defs, native, mandatory)
}
