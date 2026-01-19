// Package registry provides skill registry functionality.
package registry

import (
	"fmt"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// Service provides business logic for the skill registry.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateSkill creates a new skill in the registry.
func (s *Service) CreateSkill(sk *skill.Skill) (*skill.StoredSkill, error) {
	stored := &skill.StoredSkill{
		Name:         sk.Frontmatter.Name,
		Version:      sk.Frontmatter.Version,
		Description:  sk.Frontmatter.Description,
		Content:      skill.Render(sk),
		SourceFormat: sk.Frontmatter.SourceType,
		Tags:         sk.Frontmatter.Tags,
	}

	if err := s.repo.Create(stored); err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}

	return stored, nil
}

// GetSkill retrieves a skill by ID or slug.
func (s *Service) GetSkill(idOrSlug string) (*skill.StoredSkill, error) {
	// Try by ID first
	stored, err := s.repo.GetByID(idOrSlug)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		return stored, nil
	}

	// Try by slug
	return s.repo.GetBySlug(idOrSlug)
}

// UpdateSkill updates an existing skill.
func (s *Service) UpdateSkill(id string, sk *skill.Skill) (*skill.StoredSkill, error) {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("skill not found: %s", id)
	}

	existing.Name = sk.Frontmatter.Name
	existing.Version = sk.Frontmatter.Version
	existing.Description = sk.Frontmatter.Description
	existing.Content = skill.Render(sk)
	existing.SourceFormat = sk.Frontmatter.SourceType
	existing.Tags = sk.Frontmatter.Tags

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update skill: %w", err)
	}

	return existing, nil
}

// DeleteSkill removes a skill from the registry.
func (s *Service) DeleteSkill(id string) error {
	return s.repo.Delete(id)
}

// ListSkills retrieves a paginated list of skills.
func (s *Service) ListSkills(page, pageSize int) ([]*skill.StoredSkill, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	return s.repo.List(offset, pageSize)
}

// SearchSkills searches for skills matching a query.
func (s *Service) SearchSkills(query string, page, pageSize int) ([]*skill.StoredSkill, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	return s.repo.Search(query, offset, pageSize)
}

// ListSkillsByTag retrieves skills with a specific tag.
func (s *Service) ListSkillsByTag(tag string, page, pageSize int) ([]*skill.StoredSkill, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	return s.repo.ListByTag(tag, offset, pageSize)
}

// ViewSkill retrieves a skill and increments its view count.
func (s *Service) ViewSkill(idOrSlug string) (*skill.StoredSkill, error) {
	stored, err := s.GetSkill(idOrSlug)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, nil
	}

	// Increment view count (ignore errors)
	s.repo.IncrementViewCount(stored.ID)
	stored.ViewCount++

	return stored, nil
}

// GetAllTags retrieves all tags with their usage counts.
func (s *Service) GetAllTags() (map[string]int, error) {
	return s.repo.GetAllTags()
}

// ImportSkill imports a SKILL.md file into the registry.
func (s *Service) ImportSkill(content string) (*skill.StoredSkill, error) {
	sk, err := skill.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill: %w", err)
	}

	return s.CreateSkill(sk)
}
