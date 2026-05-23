package service

import "testing"

func TestAntigravityMetadataMatchesCPA(t *testing.T) {
	load := antigravityLoadCodeAssistMetadata()
	if len(load) != 1 || load["ideType"] != "ANTIGRAVITY" {
		t.Fatalf("loadCodeAssist metadata = %#v", load)
	}

	control := antigravityControlPlaneMetadata()
	if control["ide_type"] != "ANTIGRAVITY" || control["ide_name"] != "antigravity" || control["ide_version"] == "" {
		t.Fatalf("control plane metadata = %#v", control)
	}
}

func TestExtractAntigravityProjectIDMatchesCPA(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{name: "cloudaicompanionProject", in: map[string]any{"cloudaicompanionProject": "p1"}, want: "p1"},
		{name: "projectId", in: map[string]any{"projectId": "p2"}, want: "p2"},
		{name: "project string", in: map[string]any{"project": "p3"}, want: "p3"},
		{name: "project id", in: map[string]any{"project": map[string]any{"id": "p4"}}, want: "p4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractAntigravityProjectID(tc.in); got != tc.want {
				t.Fatalf("extractAntigravityProjectID() = %q, want %q", got, tc.want)
			}
		})
	}
}


func TestExtractAntigravityTierIDMatchesCPA(t *testing.T) {
	loadResp := map[string]any{
		"allowedTiers": []any{
			map[string]any{"id": "paid-tier", "isDefault": false},
			map[string]any{"id": "free-tier-2", "isDefault": true},
		},
	}
	if got := extractAntigravityTierID(loadResp); got != "free-tier-2" {
		t.Fatalf("default tier = %q, want free-tier-2", got)
	}

	current := map[string]any{"currentTier": map[string]any{"id": "current-tier"}}
	if got := extractAntigravityTierID(current); got != "current-tier" {
		t.Fatalf("current tier = %q, want current-tier", got)
	}

	if got := extractAntigravityTierID(map[string]any{}); got != "free-tier" {
		t.Fatalf("fallback tier = %q, want free-tier", got)
	}
}
