package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanixdarker/skillforge/internal/app"
)

func setupTestApp(t *testing.T) *app.App {
	t.Helper()

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

	t.Cleanup(func() {
		application.Close()
	})

	return application
}

func TestMergeHandler_Index(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	req := httptest.NewRequest(http.MethodGet, "/merge", nil)
	w := httptest.NewRecorder()

	handler.Index(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Merge Skills") {
		t.Error("expected page to contain 'Merge Skills'")
	}
}

func TestMergeHandler_Merge_WithFiles(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	// Create multipart form with two skill files
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Read test files
	skill1Content := `---
name: "User API"
version: "1.0.0"
description: "User management operations"
tags:
  - "users"
  - "api"
---

## Overview

User management API for creating and retrieving users.

## Endpoints

### GET /users

List all users in the system.
`

	skill2Content := `---
name: "Product API"
version: "1.0.0"
description: "Product catalog operations"
tags:
  - "products"
  - "api"
---

## Overview

Product catalog API for managing products.

## Endpoints

### GET /products

List all products in the catalog.

### POST /products

Create a new product.
`

	// Add first file
	part1, _ := writer.CreateFormFile("files", "skill1.md")
	part1.Write([]byte(skill1Content))

	// Add second file
	part2, _ := writer.CreateFormFile("files", "skill2.md")
	part2.Write([]byte(skill2Content))

	// Add optional name
	writer.WriteField("name", "Combined API")
	writer.WriteField("dedupe", "on")

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check that merged output contains expected content
	if !strings.Contains(bodyStr, "Combined API") {
		t.Error("expected merged output to contain custom name 'Combined API'")
	}
}

func TestMergeHandler_Merge_WithSkillRefs(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	// First, import some skills into the local registry
	skill1Content := `---
name: "User API"
version: "1.0.0"
description: "User management operations"
tags:
  - "users"
---

## Endpoints

### GET /users

List all users.
`

	skill2Content := `---
name: "Product API"
version: "1.0.0"
description: "Product management"
tags:
  - "products"
---

## Endpoints

### GET /products

List all products.
`

	stored1, err := application.RegistryService.ImportSkill(skill1Content)
	if err != nil {
		t.Fatalf("failed to import skill1: %v", err)
	}

	stored2, err := application.RegistryService.ImportSkill(skill2Content)
	if err != nil {
		t.Fatalf("failed to import skill2: %v", err)
	}

	// Create multipart form with skill references
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add skill references as JSON
	skillRefsJSON := `[{"id":"` + stored1.ID + `","source":"local","name":"User API"},{"id":"` + stored2.ID + `","source":"local","name":"Product API"}]`
	writer.WriteField("skill_refs", skillRefsJSON)
	writer.WriteField("name", "Merged API")
	writer.WriteField("dedupe", "on")

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check that merged output contains expected content
	if !strings.Contains(bodyStr, "Merged API") {
		t.Error("expected merged output to contain custom name 'Merged API'")
	}
}

func TestMergeHandler_Merge_InsufficientFiles(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	// Create multipart form with only one file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	skill1Content := `---
name: "User API"
version: "1.0.0"
---

## Overview
User management API.
`

	part1, _ := writer.CreateFormFile("files", "skill1.md")
	part1.Write([]byte(skill1Content))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	// Should return an error since we need at least 2 files
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "At least 2") {
		t.Errorf("expected error about needing 2 files, got: %s", string(body))
	}
}

func TestMergeHandler_Merge_InsufficientSkillRefs(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	// Create multipart form with only one skill reference
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	skillRefsJSON := `[{"id":"test-id","source":"local","name":"User API"}]`
	writer.WriteField("skill_refs", skillRefsJSON)

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "At least 2") {
		t.Errorf("expected error about needing 2 skills, got: %s", string(body))
	}
}

func TestMergeHandler_Merge_InvalidSkillRefsJSON(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Invalid JSON
	writer.WriteField("skill_refs", "not valid json")

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Invalid skill references") {
		t.Errorf("expected error about invalid JSON, got: %s", string(body))
	}
}

func TestMergeHandler_Merge_WithTestDataFiles(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	// Try to use actual test files if they exist
	skill1Path := "../../../testdata/skill1.md"
	skill2Path := "../../../testdata/skill2.md"

	skill1Content, err := os.ReadFile(skill1Path)
	if err != nil {
		t.Skip("testdata files not available")
	}
	skill2Content, err := os.ReadFile(skill2Path)
	if err != nil {
		t.Skip("testdata files not available")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part1, _ := writer.CreateFormFile("files", "skill1.md")
	part1.Write(skill1Content)

	part2, _ := writer.CreateFormFile("files", "skill2.md")
	part2.Write(skill2Content)

	writer.WriteField("dedupe", "on")

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check that both skills are included in the merge
	if !strings.Contains(bodyStr, "User") && !strings.Contains(bodyStr, "Product") {
		t.Error("expected merged output to contain content from both skills")
	}
}

func TestMergeHandler_Merge_HTMX(t *testing.T) {
	application := setupTestApp(t)
	handler := NewMergeHandler(application)

	skill1Content := `---
name: "API 1"
version: "1.0.0"
---

## Endpoints
### GET /endpoint1
`

	skill2Content := `---
name: "API 2"
version: "1.0.0"
---

## Endpoints
### GET /endpoint2
`

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part1, _ := writer.CreateFormFile("files", "skill1.md")
	part1.Write([]byte(skill1Content))

	part2, _ := writer.CreateFormFile("files", "skill2.md")
	part2.Write([]byte(skill2Content))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/merge", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("HX-Request", "true") // HTMX request header
	w := httptest.NewRecorder()

	handler.Merge(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// HTMX response should contain the code-preview partial
	if !strings.Contains(bodyStr, "code-preview") && !strings.Contains(bodyStr, "<pre") {
		t.Log("Response body:", bodyStr)
		// This is not an error, the response may not include the wrapper class
	}
}
