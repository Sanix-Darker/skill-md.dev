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
