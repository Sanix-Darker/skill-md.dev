package semver

import (
	"fmt"
	"regexp"
	"strings"
)

// Constraint represents a version constraint like "^1.0.0" or ">=2.0.0 <3.0.0".
type Constraint struct {
	raw      string
	checks   []constraintCheck
	original string
}

type constraintCheck struct {
	op      string
	version *Version
}

var constraintRegex = regexp.MustCompile(`^([<>=!~^]{0,2})\s*(v?\d+(?:\.\d+)?(?:\.\d+)?(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?)$`)

// ParseConstraint parses a version constraint string.
// Supported formats:
//   - "1.0.0" - exact version
//   - "^1.0.0" - compatible (>=1.0.0 <2.0.0)
//   - "~1.0.0" - approximately (~1.0.0 means >=1.0.0 <1.1.0)
//   - ">1.0.0" - greater than
//   - ">=1.0.0" - greater than or equal
//   - "<1.0.0" - less than
//   - "<=1.0.0" - less than or equal
//   - ">=1.0.0 <2.0.0" - range (multiple constraints)
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return &Constraint{raw: s, original: s}, nil
	}

	c := &Constraint{raw: s, original: s}

	// Split by space for multiple constraints
	parts := strings.Fields(s)
	for _, part := range parts {
		check, err := parseConstraintPart(part)
		if err != nil {
			return nil, err
		}
		c.checks = append(c.checks, check...)
	}

	return c, nil
}

func parseConstraintPart(s string) ([]constraintCheck, error) {
	s = strings.TrimSpace(s)

	// Handle caret (^) - compatible with
	if strings.HasPrefix(s, "^") {
		v, err := Parse(strings.TrimPrefix(s, "^"))
		if err != nil {
			return nil, fmt.Errorf("invalid caret constraint: %s", s)
		}
		// ^1.2.3 means >=1.2.3 <2.0.0
		upper := &Version{Major: v.Major + 1}
		return []constraintCheck{
			{op: ">=", version: v},
			{op: "<", version: upper},
		}, nil
	}

	// Handle tilde (~) - approximately
	if strings.HasPrefix(s, "~") {
		v, err := Parse(strings.TrimPrefix(s, "~"))
		if err != nil {
			return nil, fmt.Errorf("invalid tilde constraint: %s", s)
		}
		// ~1.2.3 means >=1.2.3 <1.3.0
		upper := &Version{Major: v.Major, Minor: v.Minor + 1}
		return []constraintCheck{
			{op: ">=", version: v},
			{op: "<", version: upper},
		}, nil
	}

	// Handle comparison operators
	matches := constraintRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid constraint: %s", s)
	}

	op := matches[1]
	if op == "" {
		op = "="
	}

	versionStr := matches[2]
	v, err := parsePartialVersion(versionStr)
	if err != nil {
		return nil, err
	}

	return []constraintCheck{{op: op, version: v}}, nil
}

func parsePartialVersion(s string) (*Version, error) {
	// Handle partial versions like "1" or "1.2"
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")

	major := 0
	minor := 0
	patch := 0

	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		// Parse patch, handling prerelease
		patchPart := parts[2]
		if idx := strings.Index(patchPart, "-"); idx >= 0 {
			fmt.Sscanf(patchPart[:idx], "%d", &patch)
		} else {
			fmt.Sscanf(patchPart, "%d", &patch)
		}
	}

	return Parse(fmt.Sprintf("%d.%d.%d", major, minor, patch))
}

// Check returns true if the version satisfies the constraint.
func (c *Constraint) Check(v *Version) bool {
	if c == nil || len(c.checks) == 0 {
		return true // No constraint = any version
	}

	for _, check := range c.checks {
		if !checkVersion(check, v) {
			return false
		}
	}
	return true
}

func checkVersion(check constraintCheck, v *Version) bool {
	cmp := v.Compare(check.version)
	switch check.op {
	case "=", "==":
		return cmp == 0
	case "!=":
		return cmp != 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	default:
		return cmp == 0
	}
}

// String returns the original constraint string.
func (c *Constraint) String() string {
	return c.original
}

// FindBestMatch finds the highest version that satisfies the constraint.
func (c *Constraint) FindBestMatch(versions []*Version) *Version {
	var matches []*Version
	for _, v := range versions {
		if c.Check(v) {
			matches = append(matches, v)
		}
	}
	return Latest(matches)
}

// IsExact returns true if this is an exact version constraint.
func (c *Constraint) IsExact() bool {
	if len(c.checks) == 1 && c.checks[0].op == "=" {
		return true
	}
	return false
}

// MinVersion returns the minimum version that could satisfy this constraint.
func (c *Constraint) MinVersion() *Version {
	for _, check := range c.checks {
		if check.op == ">=" || check.op == "=" {
			return check.version
		}
	}
	return nil
}
