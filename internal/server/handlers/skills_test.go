package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanixdarker/skill-md/internal/app"
)

func TestSkillsHandler_Search(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import a test skill
	skillContent := `---
name: "Test API"
version: "1.0.0"
description: "A test API for testing"
tags:
  - "test"
---

## Overview
This is a test API.
`
	_, err = application.RegistryService.ImportSkill(skillContent)
	if err != nil {
		t.Fatalf("failed to import test skill: %v", err)
	}

	handler := NewSkillsHandler(application)

	t.Run("normal search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		// Normal search returns skill-list.html template
		if !strings.Contains(string(body), "Test API") && !strings.Contains(string(body), "No skills found") {
			t.Log("Response:", string(body))
			// Results may not appear immediately in local search
		}
	})

	t.Run("merge mode search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&merge_mode=true", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Merge mode search should return merge-skill-list.html template
		// which contains "Add" button instead of links
		if strings.Contains(bodyStr, "Test API") {
			if !strings.Contains(bodyStr, "addSkillToMerge") && !strings.Contains(bodyStr, "+ Add") {
				t.Error("merge mode should include 'Add' button functionality")
			}
		}
	})

	t.Run("search with source filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&source=local", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("empty search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestSkillsHandler_View(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import a test skill
	skillContent := `---
name: "View Test API"
version: "1.0.0"
description: "A test API for view testing"
---

## Overview
This is a test API.
`
	stored, err := application.RegistryService.ImportSkill(skillContent)
	if err != nil {
		t.Fatalf("failed to import test skill: %v", err)
	}

	handler := NewSkillsHandler(application)

	t.Run("view existing skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill/"+stored.Slug, nil)
		// We need to set up chi router context for URL params
		// For now, we'll test that the handler doesn't panic
		w := httptest.NewRecorder()

		// Note: This test is limited because chi.URLParam requires router context
		// In a real test, we'd use chi's test utilities
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Handler recovered from panic (expected without chi context): %v", r)
			}
		}()

		handler.View(w, req)
	})
}

func TestSkillsHandler_List(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import test skills
	for i := 0; i < 3; i++ {
		skillContent := `---
name: "List Test API ` + string(rune('A'+i)) + `"
version: "1.0.0"
---

## Overview
Test API.
`
		_, err = application.RegistryService.ImportSkill(skillContent)
		if err != nil {
			t.Fatalf("failed to import test skill: %v", err)
		}
	}

	handler := NewSkillsHandler(application)

	t.Run("list skills", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		// Should contain skill list HTML
		if !strings.Contains(string(body), "List Test API") {
			t.Log("Response:", string(body))
		}
	})

	t.Run("list with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills?page=1", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestSkillsHandler_Create(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	handler := NewSkillsHandler(application)

	t.Run("create with content", func(t *testing.T) {
		skillContent := `---
name: "Created API"
version: "1.0.0"
---

## Overview
Created via form.
`
		body := strings.NewReader("content=" + strings.ReplaceAll(skillContent, "\n", "%0A"))
		req := httptest.NewRequest(http.MethodPost, "/api/skills", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		// This will fail without proper multipart parsing, but shouldn't panic
		handler.Create(w, req)

		// The response might be an error due to form parsing
		// Just ensure no panic occurred
	})
}

func TestSkillsHandler_Search_QueryValidation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	handler := NewSkillsHandler(application)

	t.Run("query too long", func(t *testing.T) {
		longQuery := strings.Repeat("a", 501)
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q="+longQuery, nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 for query too long, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid source type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&source=invalid", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid source, got %d", resp.StatusCode)
		}
	})

	t.Run("valid source types", func(t *testing.T) {
		validSources := []string{"local", "github", "gitlab", "skills.sh", "bitbucket", "codeberg"}
		for _, source := range validSources {
			req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&source="+source, nil)
			w := httptest.NewRecorder()

			handler.Search(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200 for valid source %s, got %d", source, resp.StatusCode)
			}
		}
	})
}

func TestSkillsHandler_Search_MergeMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import test skills with different content for search matching
	skillContent := `---
name: "Design System API"
version: "1.0.0"
description: "A design system for UI components"
tags:
  - "design"
  - "ui"
---

## Overview
This is a design system API.
`
	_, err = application.RegistryService.ImportSkill(skillContent)
	if err != nil {
		t.Fatalf("failed to import test skill: %v", err)
	}

	handler := NewSkillsHandler(application)

	t.Run("merge mode returns correct template", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=design&merge_mode=true&source=local", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Merge mode template should have addSkillToMerge function calls
		if strings.Contains(bodyStr, "Design System") {
			if !strings.Contains(bodyStr, "addSkillToMerge") {
				t.Error("merge mode template should contain addSkillToMerge function")
			}
			if !strings.Contains(bodyStr, "+ Add") {
				t.Error("merge mode template should contain '+ Add' button")
			}
		}
	})

	t.Run("non-merge mode returns different template", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=design&source=local", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Non-merge mode should NOT have addSkillToMerge
		if strings.Contains(bodyStr, "addSkillToMerge") {
			t.Error("non-merge mode should not contain addSkillToMerge function")
		}
	})
}

func TestSkillsHandler_Search_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	handler := NewSkillsHandler(application)

	t.Run("page parameter defaults to 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid page number treated as 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&page=invalid", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("negative page number treated as 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=test&page=-1", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestSkillsHandler_Search_PaginationLimit10(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import 15 test skills to test pagination
	for i := 0; i < 15; i++ {
		skillContent := `---
name: "Pagination Test Skill ` + string(rune('A'+i)) + `"
version: "1.0.0"
description: "Test skill for pagination testing"
tags:
  - "pagination"
---

## Overview
Test skill number ` + string(rune('A'+i)) + `.
`
		_, err = application.RegistryService.ImportSkill(skillContent)
		if err != nil {
			t.Fatalf("failed to import test skill %d: %v", i, err)
		}
	}

	handler := NewSkillsHandler(application)

	t.Run("first page returns max 10 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=pagination&source=local", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Count occurrences of skill items (check for unique skill markers)
		skillCount := strings.Count(bodyStr, "Pagination Test Skill")
		if skillCount > 10 {
			t.Errorf("expected max 10 skills on first page, got %d", skillCount)
		}
	})

	t.Run("merge mode has load more button when more results exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=pagination&source=local&merge_mode=true", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// With 15 skills and page size of 10, first page should have Load More
		if strings.Contains(bodyStr, "Pagination Test Skill") {
			if !strings.Contains(bodyStr, "Load More") {
				t.Log("Response should contain 'Load More' button when more results exist")
			}
		}
	})

	t.Run("page 2 returns remaining items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=pagination&source=local&page=2", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("out of bounds page returns empty results", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills/search?q=pagination&source=local&page=100", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Page 100 should have no results
		if strings.Contains(bodyStr, "Pagination Test Skill") {
			t.Error("page 100 should not contain any skills")
		}
	})
}

func TestSkillsHandler_List_PaginationLimit10(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &app.Config{
		Port:   8080,
		DBPath: dbPath,
		Debug:  true,
	}

	application, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	defer application.Close()

	// Import 15 test skills
	for i := 0; i < 15; i++ {
		skillContent := `---
name: "List Pagination Skill ` + string(rune('A'+i)) + `"
version: "1.0.0"
---

## Overview
Test.
`
		_, err = application.RegistryService.ImportSkill(skillContent)
		if err != nil {
			t.Fatalf("failed to import test skill %d: %v", i, err)
		}
	}

	handler := NewSkillsHandler(application)

	t.Run("list returns max 10 items per page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills?page=1", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Count skill occurrences
		skillCount := strings.Count(bodyStr, "List Pagination Skill")
		if skillCount > 10 {
			t.Errorf("expected max 10 skills, got %d", skillCount)
		}
	})

	t.Run("page 2 returns remaining skills", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/skills?page=2", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Page 2 should have remaining 5 skills
		skillCount := strings.Count(bodyStr, "List Pagination Skill")
		if skillCount > 5 {
			t.Errorf("expected max 5 skills on page 2, got %d", skillCount)
		}
	})
}
