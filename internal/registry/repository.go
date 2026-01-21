package registry

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// Repository handles database operations for skills.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new skill.
func (r *Repository) Create(s *skill.StoredSkill) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Slug == "" {
		s.Slug = r.generateSlug(s.Name)
	}
	s.ContentHash = r.hashContent(s.Content)
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt

	_, err := r.db.Exec(`
		INSERT INTO skills (id, slug, name, version, description, content, content_hash, source_format, view_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Slug, s.Name, s.Version, s.Description, s.Content, s.ContentHash, s.SourceFormat, s.ViewCount, s.CreatedAt, s.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create skill: %w", err)
	}

	// Add tags
	if len(s.Tags) > 0 {
		if err := r.setTags(s.ID, s.Tags); err != nil {
			return fmt.Errorf("failed to set tags: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a skill by ID.
func (r *Repository) GetByID(id string) (*skill.StoredSkill, error) {
	s := &skill.StoredSkill{}
	err := r.db.QueryRow(`
		SELECT id, slug, name, version, description, content, content_hash, source_format, view_count, created_at, updated_at
		FROM skills WHERE id = ?
	`, id).Scan(&s.ID, &s.Slug, &s.Name, &s.Version, &s.Description, &s.Content, &s.ContentHash, &s.SourceFormat, &s.ViewCount, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}

	s.Tags, _ = r.getTags(s.ID)
	return s, nil
}

// GetBySlug retrieves a skill by slug.
func (r *Repository) GetBySlug(slug string) (*skill.StoredSkill, error) {
	s := &skill.StoredSkill{}
	err := r.db.QueryRow(`
		SELECT id, slug, name, version, description, content, content_hash, source_format, view_count, created_at, updated_at
		FROM skills WHERE slug = ?
	`, slug).Scan(&s.ID, &s.Slug, &s.Name, &s.Version, &s.Description, &s.Content, &s.ContentHash, &s.SourceFormat, &s.ViewCount, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}

	s.Tags, _ = r.getTags(s.ID)
	return s, nil
}

// Update updates an existing skill.
func (r *Repository) Update(s *skill.StoredSkill) error {
	s.ContentHash = r.hashContent(s.Content)
	s.UpdatedAt = time.Now()

	_, err := r.db.Exec(`
		UPDATE skills SET name = ?, version = ?, description = ?, content = ?, content_hash = ?, source_format = ?, updated_at = ?
		WHERE id = ?
	`, s.Name, s.Version, s.Description, s.Content, s.ContentHash, s.SourceFormat, s.UpdatedAt, s.ID)

	if err != nil {
		return fmt.Errorf("failed to update skill: %w", err)
	}

	if err := r.setTags(s.ID, s.Tags); err != nil {
		return fmt.Errorf("failed to update tags: %w", err)
	}

	return nil
}

// Delete removes a skill by ID.
func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM skills WHERE id = ?", id)
	return err
}

// List retrieves skills with pagination.
func (r *Repository) List(offset, limit int) ([]*skill.StoredSkill, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count skills: %w", err)
	}

	// Get skills
	rows, err := r.db.Query(`
		SELECT id, slug, name, version, description, content, content_hash, source_format, view_count, created_at, updated_at
		FROM skills ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list skills: %w", err)
	}
	defer rows.Close()

	var skills []*skill.StoredSkill
	for rows.Next() {
		s := &skill.StoredSkill{}
		err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Version, &s.Description, &s.Content, &s.ContentHash, &s.SourceFormat, &s.ViewCount, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan skill: %w", err)
		}
		s.Tags, _ = r.getTags(s.ID)
		skills = append(skills, s)
	}

	return skills, total, nil
}

// Search performs full-text search.
func (r *Repository) Search(query string, offset, limit int) ([]*skill.StoredSkill, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM skills_fts WHERE skills_fts MATCH ?
	`, query).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Get matching skills
	rows, err := r.db.Query(`
		SELECT s.id, s.slug, s.name, s.version, s.description, s.content, s.content_hash, s.source_format, s.view_count, s.created_at, s.updated_at
		FROM skills s
		JOIN skills_fts fts ON s.rowid = fts.rowid
		WHERE skills_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search skills: %w", err)
	}
	defer rows.Close()

	var skills []*skill.StoredSkill
	for rows.Next() {
		s := &skill.StoredSkill{}
		err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Version, &s.Description, &s.Content, &s.ContentHash, &s.SourceFormat, &s.ViewCount, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan skill: %w", err)
		}
		s.Tags, _ = r.getTags(s.ID)
		skills = append(skills, s)
	}

	return skills, total, nil
}

// ListByTag retrieves skills with a specific tag.
func (r *Repository) ListByTag(tag string, offset, limit int) ([]*skill.StoredSkill, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM skills s
		JOIN skill_tags st ON s.id = st.skill_id
		JOIN tags t ON st.tag_id = t.id
		WHERE t.name = ?
	`, tag).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count skills by tag: %w", err)
	}

	// Get skills
	rows, err := r.db.Query(`
		SELECT s.id, s.slug, s.name, s.version, s.description, s.content, s.content_hash, s.source_format, s.view_count, s.created_at, s.updated_at
		FROM skills s
		JOIN skill_tags st ON s.id = st.skill_id
		JOIN tags t ON st.tag_id = t.id
		WHERE t.name = ?
		ORDER BY s.created_at DESC
		LIMIT ? OFFSET ?
	`, tag, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list skills by tag: %w", err)
	}
	defer rows.Close()

	var skills []*skill.StoredSkill
	for rows.Next() {
		s := &skill.StoredSkill{}
		err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Version, &s.Description, &s.Content, &s.ContentHash, &s.SourceFormat, &s.ViewCount, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan skill: %w", err)
		}
		s.Tags, _ = r.getTags(s.ID)
		skills = append(skills, s)
	}

	return skills, total, nil
}

// IncrementViewCount increments the view count for a skill.
func (r *Repository) IncrementViewCount(id string) error {
	_, err := r.db.Exec("UPDATE skills SET view_count = view_count + 1 WHERE id = ?", id)
	return err
}

// GetAllTags retrieves all tags with their usage counts.
func (r *Repository) GetAllTags() (map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT t.name, COUNT(st.skill_id) as count
		FROM tags t
		LEFT JOIN skill_tags st ON t.id = st.tag_id
		GROUP BY t.id
		ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer rows.Close()

	tags := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags[name] = count
	}

	return tags, nil
}

func (r *Repository) getTags(skillID string) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT t.name FROM tags t
		JOIN skill_tags st ON t.id = st.tag_id
		WHERE st.skill_id = ?
	`, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (r *Repository) setTags(skillID string, tags []string) error {
	// Remove existing tags
	_, err := r.db.Exec("DELETE FROM skill_tags WHERE skill_id = ?", skillID)
	if err != nil {
		return err
	}

	// Add new tags
	for _, tagName := range tags {
		// Get or create tag
		var tagID int64
		err := r.db.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err == sql.ErrNoRows {
			result, err := r.db.Exec("INSERT INTO tags (name) VALUES (?)", tagName)
			if err != nil {
				return err
			}
			tagID, _ = result.LastInsertId()
		} else if err != nil {
			return err
		}

		// Link skill to tag
		_, err = r.db.Exec("INSERT OR IGNORE INTO skill_tags (skill_id, tag_id) VALUES (?, ?)", skillID, tagID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)

	// Remove multiple consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")

	if slug == "" {
		slug = uuid.New().String()[:8]
	}

	// Check for uniqueness
	baseSlug := slug
	counter := 1
	for {
		var count int
		r.db.QueryRow("SELECT COUNT(*) FROM skills WHERE slug = ?", slug).Scan(&count)
		if count == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}

	return slug
}

func (r *Repository) hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
