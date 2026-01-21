package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Known skill repositories indexed by skills.sh
var knownSkillRepos = []string{
	"vercel-labs/agent-skills",
	"anthropics/anthropic-cookbook",
}

// SkillsSHSource provides access to the SKILLS.sh registry via GitHub.
// Skills.sh is a GitHub-based skill browser, so we search GitHub directly.
type SkillsSHSource struct {
	client      *http.Client
	enabled     bool
	githubToken string
}

// skillsshRepoContent represents GitHub API contents response
type skillsshRepoContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
	URL         string `json:"url"`
}

// skillsshFileContent represents GitHub file content response
type skillsshFileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// NewSkillsSHSource creates a new SKILLS.sh source.
func NewSkillsSHSource(githubToken string) *SkillsSHSource {
	return &SkillsSHSource{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		enabled:     true,
		githubToken: githubToken,
	}
}

// Name returns the source type.
func (s *SkillsSHSource) Name() SourceType {
	return SourceTypeSkillsSH
}

// Enabled returns whether this source is enabled.
func (s *SkillsSHSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *SkillsSHSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds skills from known skills.sh repositories on GitHub.
func (s *SkillsSHSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	var allSkills []*ExternalSkill
	query := strings.ToLower(opts.Query)

	// Search through known skill repositories
	for _, repoPath := range knownSkillRepos {
		skills, err := s.listSkillsFromRepo(ctx, repoPath)
		if err != nil {
			continue // Skip repos that fail
		}

		// Filter by query
		for _, skill := range skills {
			if query == "" ||
				strings.Contains(strings.ToLower(skill.Name), query) ||
				strings.Contains(strings.ToLower(skill.Description), query) ||
				strings.Contains(strings.ToLower(skill.Slug), query) {
				allSkills = append(allSkills, skill)
			}
		}
	}

	// Apply pagination
	total := len(allSkills)
	start := (opts.Page - 1) * opts.PerPage
	end := start + opts.PerPage

	if start >= total {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			Source:  SourceTypeSkillsSH,
		}, nil
	}

	if end > total {
		end = total
	}

	return &SearchResult{
		Skills:  allSkills[start:end],
		Total:   total,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeSkillsSH,
	}, nil
}

// listSkillsFromRepo lists all skills from a GitHub repository
func (s *SkillsSHSource) listSkillsFromRepo(ctx context.Context, repoPath string) ([]*ExternalSkill, error) {
	var skills []*ExternalSkill

	// Try common skill directory structures
	skillDirs := []string{"skills", ".", "packages"}

	for _, dir := range skillDirs {
		dirSkills, err := s.listSkillsFromDir(ctx, repoPath, dir)
		if err == nil && len(dirSkills) > 0 {
			skills = append(skills, dirSkills...)
		}
	}

	return skills, nil
}

// listSkillsFromDir lists skills from a specific directory in a repo
func (s *SkillsSHSource) listSkillsFromDir(ctx context.Context, repoPath, dir string) ([]*ExternalSkill, error) {
	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repoPath, dir)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.githubToken != "" {
		req.Header.Set("Authorization", "token "+s.githubToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var contents []skillsshRepoContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	var skills []*ExternalSkill
	parts := strings.Split(repoPath, "/")
	owner, repo := parts[0], parts[1]

	for _, item := range contents {
		if item.Type == "dir" {
			// Check if this directory contains a SKILL.md
			skillPath := fmt.Sprintf("%s/%s/SKILL.md", dir, item.Name)
			if dir == "." {
				skillPath = fmt.Sprintf("%s/SKILL.md", item.Name)
			}

			// Try to get skill info
			skill, err := s.getSkillInfo(ctx, repoPath, skillPath, item.Name, owner, repo)
			if err == nil && skill != nil {
				skills = append(skills, skill)
			}
		} else if item.Name == "SKILL.md" && dir == "." {
			// Root level SKILL.md
			skill, err := s.getSkillInfo(ctx, repoPath, "SKILL.md", repo, owner, repo)
			if err == nil && skill != nil {
				skills = append(skills, skill)
			}
		}
	}

	return skills, nil
}

// getSkillInfo retrieves skill metadata from a SKILL.md file
func (s *SkillsSHSource) getSkillInfo(ctx context.Context, repoPath, filePath, skillName, owner, repo string) (*ExternalSkill, error) {
	reqURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repoPath, filePath)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.githubToken != "" {
		req.Header.Set("Authorization", "token "+s.githubToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var fileContent skillsshFileContent
	if err := json.NewDecoder(resp.Body).Decode(&fileContent); err != nil {
		return nil, err
	}

	content, err := base64.StdEncoding.DecodeString(fileContent.Content)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter for name and description
	name, description := s.parseFrontmatter(string(content), skillName)

	// Build skills.sh URL
	skillSlug := skillName
	skillsshURL := fmt.Sprintf("https://skills.sh/%s/%s/%s", owner, repo, skillSlug)

	return &ExternalSkill{
		ID:          fmt.Sprintf("%s/%s", repoPath, filePath),
		Slug:        skillSlug,
		Name:        name,
		Description: description,
		Content:     string(content),
		Source:      SourceTypeSkillsSH,
		SourceURL:   skillsshURL,
		ContentURL:  fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", repoPath, filePath),
		RepoOwner:   owner,
		RepoName:    repo,
	}, nil
}

// parseFrontmatter extracts name and description from SKILL.md frontmatter
func (s *SkillsSHSource) parseFrontmatter(content, defaultName string) (string, string) {
	name := defaultName
	description := ""

	if !strings.HasPrefix(content, "---") {
		return name, description
	}

	end := strings.Index(content[3:], "---")
	if end == -1 {
		return name, description
	}

	frontmatter := content[3 : end+3]
	lines := strings.Split(frontmatter, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, "\"'")
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			description = strings.Trim(description, "\"'")
		}
	}

	return name, description
}

// GetSkill retrieves a specific skill from SKILLS.sh via GitHub.
func (s *SkillsSHSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	// ID format: owner/repo/path/to/SKILL.md
	parts := strings.SplitN(id, "/", 3)
	if len(parts) < 3 {
		return nil, nil
	}

	owner, repo := parts[0], parts[1]
	filePath := parts[2]

	// Extract skill name from path
	skillName := strings.TrimSuffix(filePath, "/SKILL.md")
	if idx := strings.LastIndex(skillName, "/"); idx != -1 {
		skillName = skillName[idx+1:]
	}

	return s.getSkillInfo(ctx, fmt.Sprintf("%s/%s", owner, repo), filePath, skillName, owner, repo)
}

// GetContent fetches the full content for a SKILLS.sh skill.
func (s *SkillsSHSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if skill.Content != "" {
		return skill.Content, nil
	}

	if skill.ContentURL == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", skill.ContentURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "SkillMD/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var content []byte
	content = make([]byte, 0, 64*1024)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if err != nil {
			break
		}
		if len(content) > 1024*1024 { // 1MB limit
			break
		}
	}

	return string(content), nil
}
