package core

import "testing"

func TestCompileAndDecision(t *testing.T) {
	tools := map[string]Effect{"host_status": EffectObserve, "restart_service": EffectRepair}
	profiles := map[string]Profile{
		"base": {Allow: []Rule{{Effect: EffectObserve, Tool: "host_status", Resource: "host"}}},
		"ops": {
			Includes: []string{"base"},
			Allow:    []Rule{{Effect: EffectRepair, Tool: "restart_service", Resource: "nginx.service"}, {Effect: EffectRepair, Tool: "restart_service", Resource: "hostlens-gateway.service"}},
			Deny:     []Rule{{Effect: EffectRepair, Tool: "restart_service", Resource: "hostlens*.service"}},
		},
		"dormant": {Allow: []Rule{{Effect: EffectRepair, Tool: "restart_service", Resource: "example.service"}}},
	}
	policy, err := Compile(profiles, []string{"ops"}, tools)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.Allows(EffectObserve, "host_status", "host") || !policy.Allows(EffectRepair, "restart_service", "nginx.service") {
		t.Fatal("active grants missing")
	}
	if policy.Allows(EffectRepair, "restart_service", "hostlens-gateway.service") || policy.Allows(EffectRepair, "restart_service", "nginx.socket") {
		t.Fatal("denial or resource pattern bypassed")
	}
	if !policy.Advertises(EffectRepair, "restart_service") || policy.Advertises(EffectObserve, "restart_service") {
		t.Fatal("discovery effect mismatch")
	}
	explanation := policy.Explain(EffectRepair, "restart_service", "hostlens-gateway.service")
	if explanation.Allowed || len(explanation.Grants) != 1 || len(explanation.Denials) != 1 || explanation.Allowed != policy.Allows(EffectRepair, "restart_service", "hostlens-gateway.service") {
		t.Fatalf("explanation disagrees with policy: %+v", explanation)
	}
}

func TestCompileRejectsInvalidActiveProfiles(t *testing.T) {
	tools := map[string]Effect{"host_status": EffectObserve, "restart_service": EffectRepair}
	cases := []struct {
		name     string
		profiles map[string]Profile
		active   []string
	}{
		{"missing", nil, []string{"missing"}},
		{"cycle", map[string]Profile{"a": {Includes: []string{"b"}}, "b": {Includes: []string{"a"}}}, []string{"a"}},
		{"unknown tool", map[string]Profile{"a": {Allow: []Rule{{Effect: EffectObserve, Tool: "other", Resource: "host"}}}}, []string{"a"}},
		{"bad pattern", map[string]Profile{"a": {Allow: []Rule{{Effect: EffectObserve, Tool: "host_status", Resource: "["}}}}, []string{"a"}},
		{"unsafe name", map[string]Profile{"../a": {}}, []string{"../a"}},
		{"broad repair", map[string]Profile{"a": {Allow: []Rule{{Effect: EffectRepair, Tool: "restart_service", Resource: "*"}}}}, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compile(tc.profiles, tc.active, tools); err == nil {
				t.Fatal("invalid active profile accepted")
			}
		})
	}
	// An inactive invalid profile cannot grant access or break startup.
	if _, err := Compile(map[string]Profile{"bad": {Allow: []Rule{{Tool: "other"}}}}, nil, tools); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryHidesFullyDeniedTool(t *testing.T) {
	tools := map[string]Effect{"service_status": EffectObserve}
	profile := Profile{Allow: []Rule{{Effect: EffectObserve, Tool: "service_status", Resource: "*"}}, Deny: []Rule{{Effect: EffectObserve, Tool: "service_status", Resource: "*"}}}
	policy, err := Compile(map[string]Profile{"all": profile}, []string{"all"}, tools)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Advertises(EffectObserve, "service_status") || policy.Allows(EffectObserve, "service_status", "any.service") {
		t.Fatal("full denial did not hide tool")
	}
}
