package validate

import "testing"

func TestChecks(t *testing.T) {
	for value, want := range map[string]bool{
		"20000000-0000-4000-8000-000000000001": true,
		"00000000-0000-0000-0000-000000000001": true,
		"ABCDEF00-0000-4000-8000-00000000000A": true,
		"20000000-0000-4000-8000-00000000000":  false,
		"not-a-uuid":                           false,
		"":                                     false,
	} {
		if UUID(value) != want {
			t.Errorf("UUID(%q) != %v", value, want)
		}
	}
	for value, want := range map[string]bool{"2026-11-10": true, "2026-02-30": false, "2026-1-10": false, "": false} {
		if Date(value) != want {
			t.Errorf("Date(%q) != %v", value, want)
		}
	}
	for value, want := range map[string]bool{"#FFAA00": true, "#ffaa00": true, "#FFF": false, "FFAA00": false} {
		if HexColor(value) != want {
			t.Errorf("HexColor(%q) != %v", value, want)
		}
	}
	if !OneOf("b", "a", "b") || OneOf("c", "a", "b") || OneOf("") {
		t.Error("OneOf")
	}
}
