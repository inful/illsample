package model

import "testing"

// TestSettings_AdapterSequence checks that the read 1 adapter is
// found under either of the names Illumina uses: "Adapter" and, in
// the BCL Convert dialect, "AdapterRead1".
func TestSettings_AdapterSequence(t *testing.T) {
	var nilSettings *Settings
	if got := nilSettings.AdapterSequence(); got != "" {
		t.Errorf("nil settings: AdapterSequence() = %q, want empty", got)
	}
	s := &Settings{Adapter: "AGAT"}
	if got := s.AdapterSequence(); got != "AGAT" {
		t.Errorf("AdapterSequence() = %q, want AGAT", got)
	}
	s = &Settings{Extra: map[string]string{"AdapterRead1": "CTGT"}}
	if got := s.AdapterSequence(); got != "CTGT" {
		t.Errorf("AdapterSequence() = %q, want CTGT", got)
	}
	// The typed field wins.
	s = &Settings{Adapter: "AGAT", Extra: map[string]string{"AdapterRead1": "CTGT"}}
	if got := s.AdapterSequence(); got != "AGAT" {
		t.Errorf("AdapterSequence() = %q, want AGAT", got)
	}
}
