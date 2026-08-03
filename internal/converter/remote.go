package converter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"
)

const maxRemoteContentSize = 10 << 20

var remoteContentFetcher = FetchRemoteContent

type RemoteContent struct {
	Body        []byte
	Filename    string
	ContentType string
}

type RemoteInput struct {
	Content    []byte
	SourcePath string
	Format     string
}

func NormalizeRemoteURL(input string) string {
	normalized := strings.TrimSpace(input)
	if normalized == "" {
		return ""
	}
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return normalized
	}
	return "https://" + normalized
}

func ResolveRemoteInput(manager *Manager, rawURL, requestedFormat string) (*RemoteInput, error) {
	normalizedURL := NormalizeRemoteURL(rawURL)
	if normalizedURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	if requestedFormat == "url" {
		return &RemoteInput{
			Content:    []byte(normalizedURL),
			SourcePath: normalizedURL,
			Format:     "url",
		}, nil
	}

	remoteContent, err := remoteContentFetcher(normalizedURL)
	if err != nil {
		return nil, err
	}

	if requestedFormat != "" && requestedFormat != "auto" {
		return &RemoteInput{
			Content:    remoteContent.Body,
			SourcePath: remoteContent.Filename,
			Format:     requestedFormat,
		}, nil
	}

	detectedFormat := manager.DetectFormat(remoteContent.Filename, remoteContent.Body)
	if shouldUseRemoteContent(detectedFormat, remoteContent) {
		return &RemoteInput{
			Content:    remoteContent.Body,
			SourcePath: remoteContent.Filename,
			Format:     detectedFormat,
		}, nil
	}

	return &RemoteInput{
		Content:    []byte(normalizedURL),
		SourcePath: normalizedURL,
		Format:     "url",
	}, nil
}

func FetchRemoteContent(rawURL string) (*RemoteContent, error) {
	safeURL := validateURL(rawURL)
	if safeURL == "" {
		return nil, fmt.Errorf("invalid or unsafe URL")
	}

	req, err := http.NewRequest(http.MethodGet, safeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "skillf/1.0")
	req.Header.Set("Accept", "*/*")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch remote content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, maxRemoteContentSize+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read remote content: %w", err)
	}
	if len(body) > maxRemoteContentSize {
		return nil, fmt.Errorf("remote content too large")
	}

	return &RemoteContent{
		Body:        body,
		Filename:    remoteFilename(safeURL),
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

func shouldUseRemoteContent(format string, remoteContent *RemoteContent) bool {
	if format == "" {
		return false
	}
	if format != "text" {
		return true
	}

	contentType := strings.ToLower(remoteContent.ContentType)
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml") {
		return false
	}

	extension := strings.ToLower(path.Ext(remoteContent.Filename))
	if extension == ".html" || extension == ".htm" {
		return false
	}

	trimmed := bytes.TrimSpace(remoteContent.Body)
	if bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<!doctype html")) || bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<html")) {
		return false
	}

	return true
}

func remoteFilename(rawURL string) string {
	parsedURL, err := neturl.Parse(rawURL)
	if err != nil {
		return "remote"
	}

	filename := path.Base(parsedURL.Path)
	if filename == "" || filename == "." || filename == "/" {
		if host := parsedURL.Hostname(); host != "" {
			return host
		}
		return "remote"
	}

	return filename
}
