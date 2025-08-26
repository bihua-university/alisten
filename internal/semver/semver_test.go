package semver

import (
	"testing"
)

func TestParseWithVPrefix(t *testing.T) {
	result := Parse("v1.2.3")
	expected := Version{Major: 1, Minor: 2, Patch: 3}
	if result != expected {
		t.Errorf("Parse(\"v1.2.3\") = %+v, expected %+v", result, expected)
	}
}

func TestParseWithoutVPrefix(t *testing.T) {
	result := Parse("1.2.3")
	expected := Version{Major: 1, Minor: 2, Patch: 3}
	if result != expected {
		t.Errorf("Parse(\"1.2.3\") = %+v, expected %+v", result, expected)
	}
}

func TestParseInvalid(t *testing.T) {
	result := Parse("1.2")
	expected := Version{}
	if result != expected {
		t.Errorf("Parse(\"1.2\") = %+v, expected %+v", result, expected)
	}
}

func TestGreaterEqual(t *testing.T) {
	testCases := []struct {
		greater string
		lesser  string
	}{
		{"v2.0.0", "v1.9.9"},
		{"v1.3.0", "v1.2.9"},
		{"v1.2.4", "v1.2.3"},
		{"v1.2.3", "v1.2.2"},
		{"v1.2.3", "v1.2.3"},
	}

	for _, tc := range testCases {
		v1 := Parse(tc.greater)
		v2 := Parse(tc.lesser)

		if !v1.GreaterEqual(v2) {
			t.Errorf("Expected %s to be greater or equal to %s, but it was not", tc.greater, tc.lesser)
		}
	}
}

func TestString(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3}
	expected := "v1.2.3"
	if v.String() != expected {
		t.Errorf("String() = %s, want %s", v.String(), expected)
	}
}
