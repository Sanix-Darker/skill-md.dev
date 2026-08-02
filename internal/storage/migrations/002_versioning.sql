-- Migration 002: Skill versioning support

-- Skill versions table for version history
CREATE TABLE IF NOT EXISTS skill_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id TEXT NOT NULL,
    version TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    changelog TEXT,
    published_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(skill_id, version),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- Index for efficient version lookups
CREATE INDEX IF NOT EXISTS idx_skill_versions_skill_id ON skill_versions(skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_versions_version ON skill_versions(version);
CREATE INDEX IF NOT EXISTS idx_skill_versions_published_at ON skill_versions(published_at DESC);

-- Add version_count to skills for quick access
-- Note: This would require ALTER TABLE which SQLite has limited support for
-- For now, we track this via a view

-- View for skill with latest version info
CREATE VIEW IF NOT EXISTS skills_with_versions AS
SELECT
    s.*,
    sv.version as latest_version,
    sv.published_at as latest_published_at,
    (SELECT COUNT(*) FROM skill_versions WHERE skill_id = s.id) as version_count
FROM skills s
LEFT JOIN skill_versions sv ON sv.skill_id = s.id
    AND sv.published_at = (
        SELECT MAX(published_at)
        FROM skill_versions
        WHERE skill_id = s.id
    );
