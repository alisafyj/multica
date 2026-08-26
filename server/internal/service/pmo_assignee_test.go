package service

import "testing"

func TestNormalizePMOOwnerEmail(t *testing.T) {
	tests := []struct {
		name, externalID, want string
	}{
		{"bare account", "yanmeichen", "yanmeichen@soyoung.com"},
		{"trim and lowercase email", " YanMeiChen@Soyoung.com ", "yanmeichen@soyoung.com"},
		{"safe punctuation", "yan.mei_chen-1", "yan.mei_chen-1@soyoung.com"},
		{"empty", "   ", ""},
		{"display name is not guessed", "严美辰", ""},
		{"spaces are invalid", "yan mei chen", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePMOOwnerEmail(tt.externalID); got != tt.want {
				t.Fatalf("normalizePMOOwnerEmail(%q) = %q, want %q", tt.externalID, got, tt.want)
			}
		})
	}
}

func TestMatchPMOAgentMappingsPrefersOwnedCodingAgentProvider(t *testing.T) {
	owners := map[string]*PMOExternalOwner{
		"fengyujie": {ExternalID: "fengyujie", DisplayName: "风尘（冯钰杰）"},
		"multi":     {ExternalID: "multi", DisplayName: "Multiple"},
		"missing":   {ExternalID: "missing", DisplayName: "Missing"},
	}
	memberEmailToUserID := map[string]string{
		"fengyujie@soyoung.com": "user-1",
		"multi@soyoung.com":     "user-2",
	}
	agents := []pmoAgentCandidate{
		{ID: "agent-1", OwnerID: "user-1", RuntimeBound: true, Provider: "codex"},
		{ID: "agent-unbound", OwnerID: "user-1", RuntimeBound: false},
		{ID: "agent-design", OwnerID: "user-2", RuntimeBound: true, Provider: "openclaw"},
		{ID: "agent-opencode", OwnerID: "user-2", RuntimeBound: true, Provider: "opencode"},
		{ID: "agent-claude", OwnerID: "user-2", RuntimeBound: true, Provider: "claude"},
		{ID: "agent-codex", OwnerID: "user-2", RuntimeBound: true, Provider: "codex"},
	}

	got := matchPMOAgentMappings(owners, memberEmailToUserID, agents, nil)

	if got["fengyujie"] != "agent-1" {
		t.Fatalf("unique mapping = %q", got["fengyujie"])
	}
	if got["multi"] != "agent-codex" {
		t.Fatalf("multi-agent owner mapping = %q, want codex agent", got["multi"])
	}
	if got["missing"] != "" {
		t.Fatalf("missing owner must stay unresolved: %#v", got)
	}
	if _, ok := got["fengyujie"]; !ok {
		t.Fatalf("unique owner should be present: %#v", got)
	}
}

func TestMatchPMOAgentMappingsExplicitAgentWinsOverAutomaticResolution(t *testing.T) {
	owners := map[string]*PMOExternalOwner{
		"yanmeichen": {ExternalID: "yanmeichen"},
	}
	memberEmailToUserID := map[string]string{"yanmeichen@soyoung.com": "user-1"}
	agents := []pmoAgentCandidate{
		{ID: "agent-auto", OwnerID: "user-1", RuntimeBound: true, Provider: "codex"},
		{ID: "agent-explicit", OwnerID: "user-2", RuntimeBound: true, Provider: "openclaw"},
	}

	got := matchPMOAgentMappings(owners, memberEmailToUserID, agents, map[string]string{
		"yanmeichen": "agent-explicit",
	})
	if got["yanmeichen"] != "agent-explicit" {
		t.Fatalf("explicit mapping = %q, want agent-explicit", got["yanmeichen"])
	}
}

func TestMatchPMOAgentMappingsDropsUnavailableExplicitAgent(t *testing.T) {
	owners := map[string]*PMOExternalOwner{
		"yanmeichen": {ExternalID: "yanmeichen"},
	}
	memberEmailToUserID := map[string]string{"yanmeichen@soyoung.com": "user-1"}
	agents := []pmoAgentCandidate{{ID: "agent-auto", OwnerID: "user-1", RuntimeBound: true, Provider: "codex"}}

	got := matchPMOAgentMappings(owners, memberEmailToUserID, agents, map[string]string{
		"yanmeichen": "agent-unavailable",
	})
	if got["yanmeichen"] != "" {
		t.Fatalf("unavailable explicit mapping must stay unresolved: %#v", got)
	}
}

func TestResolvePMOLegacyMemberMappingsUsesProviderFallbackOrder(t *testing.T) {
	got := resolvePMOLegacyMemberMappings(
		map[string]string{"fengyujie": "user-1"},
		[]pmoAgentCandidate{
			{ID: "agent-unbound", OwnerID: "user-1", RuntimeBound: false},
			{ID: "agent-opencode", OwnerID: "user-1", RuntimeBound: true, Provider: "opencode"},
			{ID: "agent-claude", OwnerID: "user-1", RuntimeBound: true, Provider: "claude"},
		},
	)

	if got["fengyujie"] != "agent-claude" {
		t.Fatalf("legacy multi-agent mapping = %q, want claude fallback", got["fengyujie"])
	}
}
