package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConvertHandler_Convert_HTMXRendersWithoutMergeFields(t *testing.T) {
	application := setupTestApp(t)
	handler := NewConvertHandler(application)

	content := `openapi: "3.1.0"
info:
  title: F1 API
  version: "1.0.0"
paths:
  /drivers:
    get:
      summary: List drivers
`

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("content", content); err != nil {
		t.Fatalf("failed to add content field: %v", err)
	}
	if err := writer.WriteField("format", "openapi"); err != nil {
		t.Fatalf("failed to add format field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/convert", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	handler.Convert(w, req)

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
	if strings.Contains(string(body), "Internal Server Error") {
		t.Fatalf("expected partial response, got server error body: %s", string(body))
	}
	if !strings.Contains(string(body), "F1 API") {
		t.Fatalf("expected converted skill in response, got: %s", string(body))
	}
}
