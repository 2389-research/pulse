// ABOUTME: Tests for core data models and section helpers.
// ABOUTME: Covers validation, title/key conversion, and constructors.
package models

import (
	"testing"
)

func TestIsValidSection(t *testing.T) {
	tests := []struct {
		section string
		valid   bool
	}{
		{"feelings", true},
		{"project_notes", true},
		{"user_context", true},
		{"technical_insights", true},
		{"world_knowledge", true},
		{"invalid", false},
		{"", false},
		{"FEELINGS", false},
	}
	for _, tt := range tests {
		if got := IsValidSection(tt.section); got != tt.valid {
			t.Errorf("IsValidSection(%q) = %v, want %v", tt.section, got, tt.valid)
		}
	}
}

func TestSectionTitleRoundtrip(t *testing.T) {
	for _, s := range GetValidSections() {
		title := SectionTitle(s)
		key := SectionKey(title)
		if key != s {
			t.Errorf("roundtrip failed: %q -> %q -> %q", s, title, key)
		}
	}
}

func TestGetValidSectionsReturnsCopy(t *testing.T) {
	a := GetValidSections()
	b := GetValidSections()
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("GetValidSections returned shared slice, not a copy")
	}
}
