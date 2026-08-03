package converter

import "testing"

func TestResolveRemoteInput_ExplicitFormatUsesFetchedContent(t *testing.T) {
	manager := NewManager()
	spec := []byte(`openapi: "3.1.0"
info:
  title: F1 API
  version: "1.0.0"
paths: {}
`)

	previousFetcher := remoteContentFetcher
	remoteContentFetcher = func(rawURL string) (*RemoteContent, error) {
		if rawURL != "https://example.com/openapi.yaml" {
			t.Fatalf("expected normalized URL, got %q", rawURL)
		}
		return &RemoteContent{
			Body:        spec,
			Filename:    "openapi.yaml",
			ContentType: "application/yaml",
		}, nil
	}
	t.Cleanup(func() {
		remoteContentFetcher = previousFetcher
	})

	input, err := ResolveRemoteInput(manager, "example.com/openapi.yaml", "openapi")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if input.Format != "openapi" {
		t.Fatalf("expected openapi format, got %q", input.Format)
	}
	if input.SourcePath != "openapi.yaml" {
		t.Fatalf("expected source path openapi.yaml, got %q", input.SourcePath)
	}
	if string(input.Content) != string(spec) {
		t.Fatal("expected fetched content to be used")
	}
}

func TestResolveRemoteInput_AutoDetectsStructuredRemoteContent(t *testing.T) {
	manager := NewManager()
	spec := []byte(`openapi: "3.1.0"
info:
  title: F1 API
  version: "1.0.0"
paths: {}
`)

	previousFetcher := remoteContentFetcher
	remoteContentFetcher = func(string) (*RemoteContent, error) {
		return &RemoteContent{
			Body:        spec,
			Filename:    "openapi.yaml",
			ContentType: "application/yaml",
		}, nil
	}
	t.Cleanup(func() {
		remoteContentFetcher = previousFetcher
	})

	input, err := ResolveRemoteInput(manager, "https://example.com/openapi.yaml", "auto")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if input.Format != "openapi" {
		t.Fatalf("expected openapi format, got %q", input.Format)
	}
	if input.SourcePath != "openapi.yaml" {
		t.Fatalf("expected source path openapi.yaml, got %q", input.SourcePath)
	}
}

func TestResolveRemoteInput_AutoFallsBackToURLForHTML(t *testing.T) {
	manager := NewManager()

	previousFetcher := remoteContentFetcher
	remoteContentFetcher = func(string) (*RemoteContent, error) {
		return &RemoteContent{
			Body:        []byte("<html><body>docs</body></html>"),
			Filename:    "index.html",
			ContentType: "text/html; charset=utf-8",
		}, nil
	}
	t.Cleanup(func() {
		remoteContentFetcher = previousFetcher
	})

	input, err := ResolveRemoteInput(manager, "https://docs.example.com", "auto")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if input.Format != "url" {
		t.Fatalf("expected url fallback, got %q", input.Format)
	}
	if input.SourcePath != "https://docs.example.com" {
		t.Fatalf("expected original URL as source path, got %q", input.SourcePath)
	}
	if string(input.Content) != "https://docs.example.com" {
		t.Fatal("expected URL converter input")
	}
}

func TestResolveRemoteInput_AutoKeepsPlainTextFilesAsText(t *testing.T) {
	manager := NewManager()

	previousFetcher := remoteContentFetcher
	remoteContentFetcher = func(string) (*RemoteContent, error) {
		return &RemoteContent{
			Body:        []byte("simple notes"),
			Filename:    "notes.txt",
			ContentType: "text/plain; charset=utf-8",
		}, nil
	}
	t.Cleanup(func() {
		remoteContentFetcher = previousFetcher
	})

	input, err := ResolveRemoteInput(manager, "https://example.com/notes.txt", "auto")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if input.Format != "text" {
		t.Fatalf("expected text format, got %q", input.Format)
	}
	if input.SourcePath != "notes.txt" {
		t.Fatalf("expected source path notes.txt, got %q", input.SourcePath)
	}
}
