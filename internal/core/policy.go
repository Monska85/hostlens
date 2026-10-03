package core

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

type Effect string

const (
	EffectObserve Effect = "observe"
	EffectRepair  Effect = "repair"
)

type Role string

const (
	RoleObserve Role = "observe"
	RoleRepair  Role = "repair"
)

// Rule names one tool and a target pattern. The pattern follows path.Match
// syntax; an exact name is the ordinary case, and "*" grants all targets.
type Rule struct {
	Effect   Effect `yaml:"effect" json:"effect"`
	Tool     string `yaml:"tool" json:"tool"`
	Resource string `yaml:"resource" json:"resource"`
}

type Profile struct {
	Includes []string `yaml:"includes,omitempty" json:"includes,omitempty"`
	Allow    []Rule   `yaml:"allow,omitempty" json:"allow,omitempty"`
	Deny     []Rule   `yaml:"deny,omitempty" json:"deny,omitempty"`
}

type Policy struct {
	allow map[ruleKey][]Rule
	deny  map[ruleKey][]Rule
}

type Explanation struct {
	Effect           Effect   `json:"effect"`
	Tool             string   `json:"tool"`
	Resource         string   `json:"resource"`
	Allowed          bool     `json:"allowed"`
	Scope            string   `json:"scope"`
	AdditionalChecks []string `json:"additional_mcp_checks"`
	Grants           []Rule   `json:"grants,omitempty"`
	Denials          []Rule   `json:"denials,omitempty"`
}

type ruleKey struct {
	effect Effect
	tool   string
}

func validRule(rule Rule, tools map[string]Effect) error {
	if rule.Effect != EffectObserve && rule.Effect != EffectRepair {
		return fmt.Errorf("invalid effect %q", rule.Effect)
	}
	if expected, ok := tools[rule.Tool]; !ok || expected != rule.Effect {
		return fmt.Errorf("unknown tool or effect %q/%q", rule.Tool, rule.Effect)
	}
	if rule.Resource == "" || strings.ContainsAny(rule.Resource, "\x00\n\r") {
		return errors.New("resource pattern is empty or contains control characters")
	}
	if _, err := path.Match(rule.Resource, "probe"); err != nil {
		return fmt.Errorf("invalid resource pattern: %w", err)
	}
	return nil
}

// Compile activates only the named profiles. It validates the entire active
// include graph before publishing a policy and never applies partial rules.
func Compile(profiles map[string]Profile, active []string, tools map[string]Effect) (Policy, error) {
	compiled := Policy{allow: make(map[ruleKey][]Rule), deny: make(map[ruleKey][]Rule)}
	state := make(map[string]uint8, len(active))
	var visit func(string) error
	visit = func(name string) error {
		if !ValidProfileName(name) {
			return fmt.Errorf("invalid profile name %q", name)
		}
		switch state[name] {
		case 1:
			return fmt.Errorf("profile include cycle at %q", name)
		case 2:
			return nil
		}
		profile, ok := profiles[name]
		if !ok {
			return fmt.Errorf("active profile %q missing", name)
		}
		state[name] = 1
		for _, include := range profile.Includes {
			if err := visit(include); err != nil {
				return err
			}
		}
		for _, rule := range profile.Allow {
			if err := validRule(rule, tools); err != nil {
				return fmt.Errorf("profile %q allow: %w", name, err)
			}
			if rule.Effect == EffectRepair && strings.ContainsAny(rule.Resource, "*?[") {
				return fmt.Errorf("profile %q: repair grants require an exact resource", name)
			}
			key := ruleKey{rule.Effect, rule.Tool}
			compiled.allow[key] = append(compiled.allow[key], rule)
		}
		for _, rule := range profile.Deny {
			if err := validRule(rule, tools); err != nil {
				return fmt.Errorf("profile %q deny: %w", name, err)
			}
			key := ruleKey{rule.Effect, rule.Tool}
			compiled.deny[key] = append(compiled.deny[key], rule)
		}
		state[name] = 2
		return nil
	}
	for _, name := range active {
		if err := visit(name); err != nil {
			return Policy{}, err
		}
	}
	return compiled, nil
}

func matches(rule Rule, effect Effect, tool, resource string) bool {
	if rule.Effect != effect || rule.Tool != tool {
		return false
	}
	if rule.Resource == resource {
		return true
	}
	if !strings.ContainsAny(rule.Resource, "*?[") {
		return false
	}
	match, _ := path.Match(rule.Resource, resource)
	return match
}

// Allows is the final profile decision for a concrete target. The caller must
// independently enforce authentication, role, read-only state, and capability.
func (p Policy) Allows(effect Effect, tool, resource string) bool {
	if resource == "" {
		return false
	}
	key := ruleKey{effect, tool}
	for _, rule := range p.deny[key] {
		if matches(rule, effect, tool, resource) {
			return false
		}
	}
	for _, rule := range p.allow[key] {
		if matches(rule, effect, tool, resource) {
			return true
		}
	}
	return false
}

// Explain uses the same rule matcher and deny precedence as Allows.
func (p Policy) Explain(effect Effect, tool, resource string) Explanation {
	checks := []string{"token_identity", "token_role", "capability"}
	if effect == EffectRepair {
		checks = append(checks, "read_only_mode")
	}
	result := Explanation{Effect: effect, Tool: tool, Resource: resource, Scope: "profile", AdditionalChecks: checks}
	if resource == "" {
		return result
	}
	key := ruleKey{effect, tool}
	for _, rule := range p.allow[key] {
		if matches(rule, effect, tool, resource) {
			result.Grants = append(result.Grants, rule)
		}
	}
	for _, rule := range p.deny[key] {
		if matches(rule, effect, tool, resource) {
			result.Denials = append(result.Denials, rule)
		}
	}
	result.Allowed = len(result.Grants) > 0 && len(result.Denials) == 0
	return result
}

// Advertises reports whether any active grant exists for a tool. Direct calls
// still evaluate the concrete target and every denial.
func (p Policy) Advertises(effect Effect, tool string) bool {
	key := ruleKey{effect, tool}
	for _, grant := range p.allow[key] {
		blocked := false
		for _, denial := range p.deny[key] {
			if denial.Resource == "*" || denial.Resource == grant.Resource || !strings.ContainsAny(grant.Resource, "*?[") && matches(denial, effect, tool, grant.Resource) {
				blocked = true
				break
			}
		}
		if !blocked {
			return true
		}
	}
	return false
}
