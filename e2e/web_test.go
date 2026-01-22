package e2e

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestHomePageLoads(t *testing.T) {
	resp, err := http.Get(getTestURL("/"))
	if err != nil {
		t.Fatalf("failed to get home page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// Check for key content
	if !strings.Contains(string(body), "Skill MD") {
		t.Error("home page does not contain 'Skill MD'")
	}
}

func TestConvertPageLoads(t *testing.T) {
	resp, err := http.Get(getTestURL("/convert"))
	if err != nil {
		t.Fatalf("failed to get convert page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if !strings.Contains(string(body), "Convert") {
		t.Error("convert page does not contain 'Convert'")
	}
}

func TestMergePageLoads(t *testing.T) {
	resp, err := http.Get(getTestURL("/merge"))
	if err != nil {
		t.Fatalf("failed to get merge page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if !strings.Contains(string(body), "Merge") {
		t.Error("merge page does not contain 'Merge'")
	}
}

func TestSkillListEndpoint(t *testing.T) {
	resp, err := http.Get(getTestURL("/api/skills"))
	if err != nil {
		t.Fatalf("failed to get skill list endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSearchAPIEndpoint(t *testing.T) {
	// Test search API without query
	resp, err := http.Get(getTestURL("/api/skills/search"))
	if err != nil {
		t.Fatalf("failed to get search endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSearchAPIWithQuery(t *testing.T) {
	// Test search API with query
	resp, err := http.Get(getTestURL("/api/skills/search?q=test"))
	if err != nil {
		t.Fatalf("failed to get search endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSearchAPIWithSource(t *testing.T) {
	// Test search API with specific source
	resp, err := http.Get(getTestURL("/api/skills/search?q=test&source=local"))
	if err != nil {
		t.Fatalf("failed to get search endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestInvalidSourceReturnsError(t *testing.T) {
	// Test search API with invalid source
	resp, err := http.Get(getTestURL("/api/skills/search?source=invalid"))
	if err != nil {
		t.Fatalf("failed to get search endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid source, got %d", resp.StatusCode)
	}
}

func TestConvertAPIEndpoint(t *testing.T) {
	// Test convert API with sample content using multipart form
	content := `openapi: "3.0.0"
info:
  title: Test API
  version: "1.0.0"
paths:
  /test:
    get:
      summary: Test endpoint`

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("content", content)
	writer.WriteField("format", "openapi")
	writer.Close()

	resp, err := http.Post(
		getTestURL("/api/convert"),
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		t.Fatalf("failed to post to convert endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// Check for SKILL.md markers
	if !strings.Contains(string(respBody), "---") {
		t.Error("convert response does not contain SKILL.md frontmatter")
	}
}

func TestStaticAssetsServed(t *testing.T) {
	// Test that static files route works (may return 404 for non-existent file)
	resp, err := http.Get(getTestURL("/static/test.txt"))
	if err != nil {
		t.Fatalf("failed to get static endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Route should work, file may or may not exist
	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("static route returned server error: %d", resp.StatusCode)
	}
}
