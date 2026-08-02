// Package semver provides semantic versioning parsing, comparison, and manipulation.
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// semverRegex matches semantic version strings.
var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// Parse parses a semantic version string.
func Parse(s string) (*Version, error) {
	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid semantic version: %s", s)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// MustParse parses a version string and panics on error.
func MustParse(s string) *Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// IsValid returns true if the string is a valid semantic version.
func IsValid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// String returns the string representation of the version.
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInt(v.Patch, other.Patch)
	}

	// Prerelease versions have lower precedence
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		return comparePrereleases(v.Prerelease, other.Prerelease)
	}

	return 0
}

// LessThan returns true if v < other.
func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other.
func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other.
func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

// BumpType represents the type of version bump.
type BumpType string

const (
	BumpMajor BumpType = "major"
	BumpMinor BumpType = "minor"
	BumpPatch BumpType = "patch"
)

// Bump increments the version by the specified type.
func (v *Version) Bump(bumpType BumpType) *Version {
	newV := &Version{
		Major: v.Major,
		Minor: v.Minor,
		Patch: v.Patch,
	}

	switch bumpType {
	case BumpMajor:
		newV.Major++
		newV.Minor = 0
		newV.Patch = 0
	case BumpMinor:
		newV.Minor++
		newV.Patch = 0
	case BumpPatch:
		newV.Patch++
	}

	// Clear prerelease and build on bump
	newV.Prerelease = ""
	newV.Build = ""

	return newV
}

// BumpMajor returns a new version with major incremented.
func (v *Version) BumpMajor() *Version {
	return v.Bump(BumpMajor)
}

// BumpMinor returns a new version with minor incremented.
func (v *Version) BumpMinor() *Version {
	return v.Bump(BumpMinor)
}

// BumpPatch returns a new version with patch incremented.
func (v *Version) BumpPatch() *Version {
	return v.Bump(BumpPatch)
}

// WithPrerelease returns a copy with the given prerelease.
func (v *Version) WithPrerelease(pre string) *Version {
	return &Version{
		Major:      v.Major,
		Minor:      v.Minor,
		Patch:      v.Patch,
		Prerelease: pre,
		Build:      v.Build,
	}
}

// WithBuild returns a copy with the given build metadata.
func (v *Version) WithBuild(build string) *Version {
	return &Version{
		Major:      v.Major,
		Minor:      v.Minor,
		Patch:      v.Patch,
		Prerelease: v.Prerelease,
		Build:      build,
	}
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func comparePrereleases(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aNum, aErr := strconv.Atoi(aParts[i])
		bNum, bErr := strconv.Atoi(bParts[i])

		// Both numeric
		if aErr == nil && bErr == nil {
			if aNum != bNum {
				return compareInt(aNum, bNum)
			}
			continue
		}

		// Numeric < alphanumeric
		if aErr == nil {
			return -1
		}
		if bErr == nil {
			return 1
		}

		// Both alphanumeric
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}

	// More parts = higher precedence
	return compareInt(len(aParts), len(bParts))
}

// Sort sorts a slice of versions in ascending order.
func Sort(versions []*Version) {
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[j].LessThan(versions[i]) {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
}

// Latest returns the highest version from a slice.
func Latest(versions []*Version) *Version {
	if len(versions) == 0 {
		return nil
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.GreaterThan(latest) {
			latest = v
		}
	}
	return latest
}
