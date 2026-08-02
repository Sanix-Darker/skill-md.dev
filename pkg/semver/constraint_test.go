package semver

import (
	"testing"
)

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		input   string
		version string
		want    bool
	}{
		// Exact version
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"1.0.0", "2.0.0", false},

		// Caret (^) - compatible with
		{"^1.0.0", "1.0.0", true},
		{"^1.0.0", "1.5.0", true},
		{"^1.0.0", "1.9.9", true},
		{"^1.0.0", "2.0.0", false},
		{"^1.2.3", "1.2.3", true},
		{"^1.2.3", "1.3.0", true},
		{"^1.2.3", "1.2.2", false},

		// Tilde (~) - approximately
		{"~1.2.3", "1.2.3", true},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"~1.2.0", "1.2.5", true},
		{"~1.2.0", "1.3.0", false},

		// Greater than
		{">1.0.0", "1.0.1", true},
		{">1.0.0", "2.0.0", true},
		{">1.0.0", "1.0.0", false},
		{">1.0.0", "0.9.0", false},

		// Greater than or equal
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "1.0.1", true},
		{">=1.0.0", "0.9.9", false},

		// Less than
		{"<2.0.0", "1.0.0", true},
		{"<2.0.0", "1.9.9", true},
		{"<2.0.0", "2.0.0", false},
		{"<2.0.0", "2.0.1", false},

		// Less than or equal
		{"<=2.0.0", "2.0.0", true},
		{"<=2.0.0", "1.9.9", true},
		{"<=2.0.0", "2.0.1", false},

		// Range
		{">=1.0.0 <2.0.0", "1.0.0", true},
		{">=1.0.0 <2.0.0", "1.5.0", true},
		{">=1.0.0 <2.0.0", "2.0.0", false},
		{">=1.0.0 <2.0.0", "0.9.0", false},

		// Wildcard
		{"*", "1.0.0", true},
		{"*", "999.0.0", true},
		{"", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+tt.version, func(t *testing.T) {
			c, err := ParseConstraint(tt.input)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.input, err)
			}

			v := MustParse(tt.version)
			if got := c.Check(v); got != tt.want {
				t.Errorf("Constraint(%q).Check(%q) = %v, want %v", tt.input, tt.version, got, tt.want)
			}
		})
	}
}

func TestConstraintInvalid(t *testing.T) {
	tests := []string{
		"^invalid",
		"~not-a-version",
		">>>1.0.0",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := ParseConstraint(tt)
			if err == nil {
				t.Errorf("ParseConstraint(%q) should return error", tt)
			}
		})
	}
}

func TestFindBestMatch(t *testing.T) {
	versions := []*Version{
		MustParse("1.0.0"),
		MustParse("1.1.0"),
		MustParse("1.2.0"),
		MustParse("2.0.0"),
		MustParse("2.1.0"),
	}

	tests := []struct {
		constraint string
		want       string
	}{
		{"^1.0.0", "1.2.0"},
		{"~1.0.0", "1.0.0"},
		{">=2.0.0", "2.1.0"},
		{"<2.0.0", "1.2.0"},
		{">=1.0.0 <1.2.0", "1.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			c, _ := ParseConstraint(tt.constraint)
			got := c.FindBestMatch(versions)
			if got == nil {
				t.Fatalf("FindBestMatch(%q) returned nil", tt.constraint)
			}
			if got.String() != tt.want {
				t.Errorf("FindBestMatch(%q) = %q, want %q", tt.constraint, got.String(), tt.want)
			}
		})
	}
}

func TestConstraintString(t *testing.T) {
	inputs := []string{"^1.0.0", "~2.3.4", ">=1.0.0 <2.0.0", "*"}

	for _, input := range inputs {
		c, err := ParseConstraint(input)
		if err != nil {
			t.Fatalf("ParseConstraint(%q) error = %v", input, err)
		}
		if c.String() != input {
			t.Errorf("Constraint(%q).String() = %q", input, c.String())
		}
	}
}

func TestIsExact(t *testing.T) {
	tests := []struct {
		constraint string
		want       bool
	}{
		{"1.0.0", true},
		{"^1.0.0", false},
		{">=1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			c, _ := ParseConstraint(tt.constraint)
			if got := c.IsExact(); got != tt.want {
				t.Errorf("IsExact(%q) = %v, want %v", tt.constraint, got, tt.want)
			}
		})
	}
}
