package sources

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// FederatedSource orchestrates searches across multiple sources.
type FederatedSource struct {
	sources     map[SourceType]Source
	cache       *SearchCache
	skillCache  *SkillCache
	rateLimiter *RateLimiter
	logger      *slog.Logger
}

// FederatedResult contains combined results from multiple sources.
type FederatedResult struct {
	Skills      []*ExternalSkill
	Total       int
	BySource    map[SourceType]int
	SearchTime  time.Duration
	SourceTimes map[SourceType]time.Duration
}

// NewFederatedSource creates a new federated source manager.
func NewFederatedSource(logger *slog.Logger) *FederatedSource {
	return &FederatedSource{
		sources:     make(map[SourceType]Source),
		cache:       NewSearchCache(),
		skillCache:  NewSkillCache(),
		rateLimiter: NewRateLimiter(),
		logger:      logger,
	}
}

// RegisterSource adds a source to the federation.
func (f *FederatedSource) RegisterSource(source Source) {
	f.sources[source.Name()] = source
}

// GetSource returns a specific source by type.
func (f *FederatedSource) GetSource(sourceType SourceType) (Source, bool) {
	source, ok := f.sources[sourceType]
	return source, ok
}

// EnabledSources returns the list of enabled source types.
func (f *FederatedSource) EnabledSources() []SourceType {
	var enabled []SourceType
	for sourceType, source := range f.sources {
		if source.Enabled() {
			enabled = append(enabled, sourceType)
		}
	}
	return enabled
}

// Search performs a federated search across all enabled sources.
func (f *FederatedSource) Search(ctx context.Context, opts SearchOptions) (*FederatedResult, error) {
	return f.SearchSources(ctx, opts, nil)
}

// SearchSources performs a federated search across specified sources.
// If sources is nil, searches all enabled sources.
func (f *FederatedSource) SearchSources(ctx context.Context, opts SearchOptions, sources []SourceType) (*FederatedResult, error) {
	start := time.Now()

	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	// Determine which sources to search
	var toSearch []Source
	if sources == nil {
		for _, source := range f.sources {
			if source.Enabled() {
				toSearch = append(toSearch, source)
			}
		}
	} else {
		for _, sourceType := range sources {
			if source, ok := f.sources[sourceType]; ok && source.Enabled() {
				toSearch = append(toSearch, source)
			}
		}
	}

	if len(toSearch) == 0 {
		return &FederatedResult{
			Skills:      []*ExternalSkill{},
			BySource:    make(map[SourceType]int),
			SourceTimes: make(map[SourceType]time.Duration),
		}, nil
	}

	// Search all sources in parallel
	var wg sync.WaitGroup
	resultCh := make(chan *SearchResult, len(toSearch))
	errCh := make(chan error, len(toSearch))

	for _, source := range toSearch {
		wg.Add(1)
		go func(src Source) {
			defer wg.Done()

			sourceType := src.Name()

			// Check cache first
			if cached, ok := f.cache.GetSearchResult(sourceType, opts.Query, opts.Page); ok {
				resultCh <- cached
				return
			}

			// Rate limit
			if err := f.rateLimiter.Wait(ctx, sourceType); err != nil {
				f.logger.Warn("rate limit wait cancelled", "source", sourceType, "error", err)
				return
			}

			// Perform search
			searchStart := time.Now()
			result, err := src.Search(ctx, opts)
			if err != nil {
				f.logger.Error("source search failed", "source", sourceType, "error", err)
				errCh <- err
				return
			}

			result.SearchTime = time.Since(searchStart)
			result.Source = sourceType

			// Cache results (skip for local source)
			if sourceType != SourceTypeLocal {
				f.cache.SetSearchResult(sourceType, opts.Query, opts.Page, result)
			}

			resultCh <- result
		}(source)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	// Collect results
	fedResult := &FederatedResult{
		Skills:      make([]*ExternalSkill, 0),
		BySource:    make(map[SourceType]int),
		SourceTimes: make(map[SourceType]time.Duration),
	}

	for result := range resultCh {
		fedResult.Skills = append(fedResult.Skills, result.Skills...)
		fedResult.Total += result.Total
		fedResult.BySource[result.Source] = result.Total
		fedResult.SourceTimes[result.Source] = result.SearchTime
	}

	// Sort results by relevance (stars for external, then by name)
	sort.Slice(fedResult.Skills, func(i, j int) bool {
		// Local results first
		if fedResult.Skills[i].Source == SourceTypeLocal && fedResult.Skills[j].Source != SourceTypeLocal {
			return true
		}
		if fedResult.Skills[i].Source != SourceTypeLocal && fedResult.Skills[j].Source == SourceTypeLocal {
			return false
		}
		// Then by stars
		if fedResult.Skills[i].Stars != fedResult.Skills[j].Stars {
			return fedResult.Skills[i].Stars > fedResult.Skills[j].Stars
		}
		// Then by name
		return fedResult.Skills[i].Name < fedResult.Skills[j].Name
	})

	fedResult.SearchTime = time.Since(start)
	return fedResult, nil
}

// SearchSource searches a specific source only.
func (f *FederatedSource) SearchSource(ctx context.Context, sourceType SourceType, opts SearchOptions) (*SearchResult, error) {
	source, ok := f.sources[sourceType]
	if !ok {
		return &SearchResult{Skills: []*ExternalSkill{}}, nil
	}

	if !source.Enabled() {
		return &SearchResult{Skills: []*ExternalSkill{}}, nil
	}

	// Check cache first
	if cached, ok := f.cache.GetSearchResult(sourceType, opts.Query, opts.Page); ok {
		return cached, nil
	}

	// Rate limit
	if err := f.rateLimiter.Wait(ctx, sourceType); err != nil {
		return nil, err
	}

	// Perform search
	start := time.Now()
	result, err := source.Search(ctx, opts)
	if err != nil {
		return nil, err
	}

	result.SearchTime = time.Since(start)
	result.Source = sourceType

	// Cache results (skip for local source)
	if sourceType != SourceTypeLocal {
		f.cache.SetSearchResult(sourceType, opts.Query, opts.Page, result)
	}

	return result, nil
}

// GetSkill retrieves a skill from a specific source.
func (f *FederatedSource) GetSkill(ctx context.Context, sourceType SourceType, id string) (*ExternalSkill, error) {
	// Check cache first
	if cached, ok := f.skillCache.GetSkill(sourceType, id); ok {
		return cached, nil
	}

	source, ok := f.sources[sourceType]
	if !ok {
		return nil, nil
	}

	// Rate limit
	if err := f.rateLimiter.Wait(ctx, sourceType); err != nil {
		return nil, err
	}

	skill, err := source.GetSkill(ctx, id)
	if err != nil {
		return nil, err
	}

	if skill != nil && sourceType != SourceTypeLocal {
		f.skillCache.SetSkill(sourceType, id, skill)
	}

	return skill, nil
}

// GetContent fetches the full content for an external skill.
func (f *FederatedSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if skill.Content != "" {
		return skill.Content, nil
	}

	source, ok := f.sources[skill.Source]
	if !ok {
		return "", nil
	}

	// Rate limit
	if err := f.rateLimiter.Wait(ctx, skill.Source); err != nil {
		return "", err
	}

	return source.GetContent(ctx, skill)
}

// ClearCache clears all caches.
func (f *FederatedSource) ClearCache() {
	f.cache.Clear()
	f.skillCache.Clear()
}
