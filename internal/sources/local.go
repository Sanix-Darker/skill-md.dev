package sources

import (
	"context"

	"github.com/sanixdarker/skill-md/internal/registry"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// LocalSource adapts the local registry to the Source interface.
type LocalSource struct {
	registry *registry.Service
	enabled  bool
}

// NewLocalSource creates a new local source adapter.
func NewLocalSource(registryService *registry.Service) *LocalSource {
	return &LocalSource{
		registry: registryService,
		enabled:  true,
	}
}

// Name returns the source type.
func (s *LocalSource) Name() SourceType {
	return SourceTypeLocal
}

// Enabled returns whether this source is enabled.
func (s *LocalSource) Enabled() bool {
	return s.enabled && s.registry != nil
}

// SetEnabled sets whether this source is enabled.
func (s *LocalSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds skills in the local registry.
func (s *LocalSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if s.registry == nil {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeLocal,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	var skills interface{}
	var total int
	var err error

	if opts.Query != "" {
		skills, total, err = s.registry.SearchSkills(opts.Query, opts.Page, opts.PerPage)
	} else if len(opts.Tags) > 0 {
		skills, total, err = s.registry.ListSkillsByTag(opts.Tags[0], opts.Page, opts.PerPage)
	} else {
		skills, total, err = s.registry.ListSkills(opts.Page, opts.PerPage)
	}

	if err != nil {
		return nil, err
	}

	// Convert to ExternalSkill format
	var externalSkills []*ExternalSkill
	if storedSkills, ok := skills.([]*skill.StoredSkill); ok {
		for _, sk := range storedSkills {
			externalSkills = append(externalSkills, s.convertToExternal(sk))
		}
	}

	return &SearchResult{
		Skills:  externalSkills,
		Total:   total,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeLocal,
	}, nil
}

// GetSkill retrieves a skill from the local registry.
func (s *LocalSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	if s.registry == nil {
		return nil, nil
	}

	skill, err := s.registry.GetSkill(id)
	if err != nil {
		return nil, err
	}

	if skill == nil {
		return nil, nil
	}

	return s.convertToExternal(skill), nil
}

// GetContent returns the content of a local skill.
func (s *LocalSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if s.registry == nil {
		return "", nil
	}

	stored, err := s.registry.GetSkill(skill.Slug)
	if err != nil {
		return "", err
	}

	if stored != nil {
		return stored.Content, nil
	}

	return "", nil
}

// convertToExternal converts a StoredSkill to ExternalSkill.
func (s *LocalSource) convertToExternal(sk *skill.StoredSkill) *ExternalSkill {
	return &ExternalSkill{
		ID:          sk.ID,
		Slug:        sk.Slug,
		Name:        sk.Name,
		Description: sk.Description,
		Content:     sk.Content,
		Tags:        sk.Tags,
		Source:      SourceTypeLocal,
		SourceURL:   "/skill/" + sk.Slug,
		Version:     sk.Version,
		UpdatedAt:   sk.UpdatedAt,
	}
}
