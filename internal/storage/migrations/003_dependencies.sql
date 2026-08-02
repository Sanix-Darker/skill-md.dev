-- Migration 003: Skill dependencies support

-- Skill dependencies table
CREATE TABLE IF NOT EXISTS skill_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id TEXT NOT NULL,
    dependency_name TEXT NOT NULL,
    version_constraint TEXT NOT NULL,
    resolved_version TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(skill_id, dependency_name),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- Skill lockfiles table for resolved dependencies
CREATE TABLE IF NOT EXISTS skill_locks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id TEXT NOT NULL,
    dependency_name TEXT NOT NULL,
    locked_version TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    source TEXT,
    locked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(skill_id, dependency_name),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_skill_dependencies_skill_id ON skill_dependencies(skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_dependencies_name ON skill_dependencies(dependency_name);
CREATE INDEX IF NOT EXISTS idx_skill_locks_skill_id ON skill_locks(skill_id);

-- View for skills with dependency counts
CREATE VIEW IF NOT EXISTS skills_with_deps AS
SELECT
    s.*,
    (SELECT COUNT(*) FROM skill_dependencies WHERE skill_id = s.id) as dependency_count,
    (SELECT COUNT(*) FROM skill_locks WHERE skill_id = s.id) as locked_dep_count
FROM skills s;
