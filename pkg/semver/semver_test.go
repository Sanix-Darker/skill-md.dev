package semver

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    *Version
		wantErr bool
	}{
		{"1.0.0", &Version{Major: 1, Minor: 0, Patch: 0}, false},
		{"0.1.0", &Version{Major: 0, Minor: 1, Patch: 0}, false},
		{"2.3.4", &Version{Major: 2, Minor: 3, Patch: 4}, false},
		{"v1.2.3", &Version{Major: 1, Minor: 2, Patch: 3}, false},
		{"1.2.3-alpha", &Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"}, false},
		{"1.2.3-alpha.1", &Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha.1"}, false},
		{"1.2.3+build", &Version{Major: 1, Minor: 2, Patch: 3, Build: "build"}, false},
		{"1.2.3-beta+build.123", &Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta", Build: "build.123"}, false},
		{"invalid", nil, true},
		{"1.2", nil, true},
		{"1", nil, true},
		{"", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.want != nil && got != nil {
				if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
					t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
				}
				if got.Prerelease != tt.want.Prerelease {
					t.Errorf("Parse(%q).Prerelease = %q, want %q", tt.input, got.Prerelease, tt.want.Prerelease)
				}
				if got.Build != tt.want.Build {
					t.Errorf("Parse(%q).Build = %q, want %q", tt.input, got.Build, tt.want.Build)
				}
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1.0.0", true},
		{"v2.3.4", true},
		{"1.2.3-alpha", true},
		{"invalid", false},
		{"1.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsValid(tt.input); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0-alpha", "1.0.0", -1},      // prerelease < release
		{"1.0.0", "1.0.0-alpha", 1},       // release > prerelease
		{"1.0.0-alpha", "1.0.0-beta", -1}, // alpha < beta
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a := MustParse(tt.a)
			b := MustParse(tt.b)
			if got := a.Compare(b); got != tt.want {
				t.Errorf("Compare(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestBump(t *testing.T) {
	tests := []struct {
		input    string
		bumpType BumpType
		want     string
	}{
		{"1.0.0", BumpPatch, "1.0.1"},
		{"1.0.0", BumpMinor, "1.1.0"},
		{"1.0.0", BumpMajor, "2.0.0"},
		{"1.2.3", BumpPatch, "1.2.4"},
		{"1.2.3", BumpMinor, "1.3.0"},
		{"1.2.3", BumpMajor, "2.0.0"},
		{"1.0.0-alpha", BumpPatch, "1.0.1"}, // clears prerelease
		{"0.0.0", BumpPatch, "0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input+"_"+string(tt.bumpType), func(t *testing.T) {
			v := MustParse(tt.input)
			got := v.Bump(tt.bumpType)
			if got.String() != tt.want {
				t.Errorf("Bump(%q, %s) = %q, want %q", tt.input, tt.bumpType, got.String(), tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		version *Version
		want    string
	}{
		{&Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{&Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"}, "1.2.3-alpha"},
		{&Version{Major: 1, Minor: 2, Patch: 3, Build: "build"}, "1.2.3+build"},
		{&Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1", Build: "123"}, "1.2.3-rc.1+123"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	versions := []*Version{
		MustParse("2.0.0"),
		MustParse("1.0.0"),
		MustParse("1.5.0"),
		MustParse("1.0.0-alpha"),
		MustParse("1.2.3"),
	}

	Sort(versions)

	expected := []string{"1.0.0-alpha", "1.0.0", "1.2.3", "1.5.0", "2.0.0"}
	for i, v := range versions {
		if v.String() != expected[i] {
			t.Errorf("Sort[%d] = %q, want %q", i, v.String(), expected[i])
		}
	}
}

func TestLatest(t *testing.T) {
	versions := []*Version{
		MustParse("1.0.0"),
		MustParse("2.0.0"),
		MustParse("1.5.0"),
	}

	latest := Latest(versions)
	if latest.String() != "2.0.0" {
		t.Errorf("Latest() = %q, want %q", latest.String(), "2.0.0")
	}

	// Empty slice
	if Latest(nil) != nil {
		t.Error("Latest(nil) should return nil")
	}
}
