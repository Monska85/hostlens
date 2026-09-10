package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Monska85/hostlens/internal/config"
)

type Rule struct {
	Category  string   `json:"category"`
	Pattern   string   `json:"pattern"`
	Deny      bool     `json:"deny"`
	Source    string   `json:"source"`
	Profile   string   `json:"profile,omitempty"`
	Chain     []string `json:"chain,omitempty"`
	Inactive  bool     `json:"inactive,omitempty"`
	Mandatory bool     `json:"mandatory,omitempty"`
	Literal   bool     `json:"literal,omitempty"`
}
type semantics struct {
	identity      string
	categories    func(config.Rules) map[string][]string
	validPattern  func(string, string) bool
	matches       func(Rule, string) bool
	builtinExempt func(Rule) bool
}

type Policy struct {
	Rules     []Rule `json:"rules"`
	semantics semantics
}
type Definition struct {
	Profile config.Profile
	Source  string
}

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func ValidName(s string) bool { return nameRE.MatchString(s) }
func compile(profile config.Profile, source string, defs map[string]Definition, native semantics, mandatory []Rule) (*Policy, error) {
	p := &Policy{semantics: native}
	active := map[string]bool{}
	add := func(pr config.Profile, src, name string, chain []string, inactive bool) error {
		for _, set := range []struct {
			r    config.Rules
			deny bool
		}{{pr.Allow, false}, {pr.Deny, true}} {
			for cat, patterns := range native.categories(set.r) {
				for _, pat := range patterns {
					if !native.validPattern(cat, pat) {
						return fmt.Errorf("%s: invalid %s pattern %q", src, cat, pat)
					}
					if len(p.Rules) >= 10000 {
						return fmt.Errorf("resolved policy exceeds 10000 provenance rules")
					}
					p.Rules = append(p.Rules, Rule{cat, pat, set.deny, src, name, append([]string{}, chain...), inactive, false, false})
				}
			}
		}
		return nil
	}
	visits := 0
	var visit func(string, []string) error
	visit = func(name string, chain []string) error {
		visits++
		if visits > 10000 {
			return fmt.Errorf("profile traversal exceeds 10000 visits")
		}
		if !ValidName(name) {
			return fmt.Errorf("unsafe profile reference %q", name)
		}
		for _, n := range chain {
			if n == name {
				return fmt.Errorf("profile cycle: %s", strings.Join(append(chain, name), " -> "))
			}
		}
		d, ok := defs[name]
		if !ok {
			return fmt.Errorf("missing profile %q", name)
		}
		if len(chain) > 64 {
			return fmt.Errorf("profile include depth exceeds 64")
		}
		active[name] = true
		chain = append(append([]string{}, chain...), name)
		if e := add(d.Profile, d.Source, name, chain, false); e != nil {
			return e
		}
		for _, n := range d.Profile.Profiles {
			if e := visit(n, chain); e != nil {
				return e
			}
		}
		return nil
	}
	if e := add(profile, source, "", nil, false); e != nil {
		return nil, e
	}
	for _, n := range profile.Profiles {
		if e := visit(n, nil); e != nil {
			return nil, e
		}
	}
	for name, d := range defs {
		if !active[name] {
			if e := add(d.Profile, d.Source, name, []string{name}, true); e != nil {
				return nil, e
			}
		}
	}
	p.Rules = append(p.Rules, mandatory...)
	return p, nil
}
func (p *Policy) Matches(category, target string) []Rule {
	var out []Rule
	matched := make(map[struct {
		pattern string
		literal bool
	}]bool)
	for _, r := range p.Rules {
		if r.Category != category {
			continue
		}
		key := struct {
			pattern string
			literal bool
		}{r.Pattern, r.Literal}
		yes, known := matched[key]
		if !known {
			yes = p.semantics.matches(r, target)
			matched[key] = yes
		}
		if yes {
			out = append(out, r)
		}
	}
	return out
}
func (p *Policy) Allowed(category, target string, builtin bool) bool {
	allow := builtin
	var previousPattern string
	var previousLiteral, known, matched bool
	for _, r := range p.Rules {
		if r.Inactive || r.Category != category || (allow && !r.Deny) {
			continue
		}
		if builtin && p.semantics.builtinExempt(r) {
			continue
		}
		// Repeated provenance often shares a pattern. Reuse only this call's
		// last match, without allocating or retaining mutable rule state.
		if !known || r.Pattern != previousPattern || r.Literal != previousLiteral {
			matched = p.semantics.matches(r, target)
			previousPattern, previousLiteral, known = r.Pattern, r.Literal, true
		}
		if !matched {
			continue
		}
		if r.Deny {
			return false
		}
		allow = true
	}
	return allow
}
func (p *Policy) Fingerprint() string {
	seen := map[string]bool{}
	for _, r := range p.Rules {
		if r.Inactive {
			continue
		}
		b, _ := json.Marshal([]any{r.Category, r.Pattern, r.Deny, r.Mandatory, r.Literal})
		seen[string(b)] = true
	}
	keys := []string{p.semantics.identity}
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b, _ := json.Marshal(keys)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
