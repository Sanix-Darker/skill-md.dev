package deps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"
)

// Lockfile represents a skill.lock file with resolved dependencies.
type Lockfile struct {
	Version     int             `json:"lockfile_version"`
	GeneratedAt string          `json:"generated_at"`
	SkillName   string          `json:"skill_name"`
	SkillHash   string          `json:"skill_hash"`
	Packages    []LockedPackage `json:"packages"`
}

// LockedPackage represents a locked dependency.
type LockedPackage struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	ContentHash string   `json:"content_hash"`
	Source      string   `json:"source,omitempty"`
	Requires    []string `json:"requires,omitempty"`
}

// NewLockfile creates a new lockfile from resolution results.
func NewLockfile(skillName, skillContent string, result *ResolutionResult) *Lockfile {
	lf := &Lockfile{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SkillName:   skillName,
		SkillHash:   hashContent(skillContent),
		Packages:    make([]LockedPackage, 0, len(result.Resolved)),
	}

	for _, resolved := range result.Resolved {
		pkg := LockedPackage{
			Name:        resolved.Name,
			Version:     resolved.Version.String(),
			ContentHash: resolved.ContentHash,
			Source:      resolved.Source,
			Requires:    resolved.Dependencies,
		}
		lf.Packages = append(lf.Packages, pkg)
	}

	return lf
}

// Write writes the lockfile to disk.
func (lf *Lockfile) Write(path string) error {
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadLockfile reads a lockfile from disk.
func ReadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, err
	}

	return &lf, nil
}

// IsStale checks if the lockfile is outdated compared to the skill content.
func (lf *Lockfile) IsStale(skillContent string) bool {
	return lf.SkillHash != hashContent(skillContent)
}

// GetPackage returns a locked package by name.
func (lf *Lockfile) GetPackage(name string) *LockedPackage {
	for i := range lf.Packages {
		if lf.Packages[i].Name == name {
			return &lf.Packages[i]
		}
	}
	return nil
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16]) // First 16 bytes for shorter hash
}
