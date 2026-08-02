// Package handlers provides HTTP handlers.
package handlers

import (
	"net/http"

	"github.com/sanixdarker/skillf/internal/app"
	"github.com/sanixdarker/skillf/web"
)

// HomeHandler handles home page requests.
type HomeHandler struct {
	app *app.App
}

// NewHomeHandler creates a new HomeHandler.
func NewHomeHandler(application *app.App) *HomeHandler {
	return &HomeHandler{app: application}
}

// Index renders the home page.
func (h *HomeHandler) Index(w http.ResponseWriter, r *http.Request) {
	// Get supported formats from converter manager
	formats := h.app.ConverterManager.SupportedFormats()
	formatCount := len(formats)

	data := map[string]interface{}{
		"Title":       "Skillf",
		"Description": "Convert technical specs to SKILL.md format for AI agents",
		"Formats":     formats,
		"FormatCount": formatCount,
	}

	if err := web.RenderPage(w, "home.html", data); err != nil {
		h.app.Logger.Error("failed to render home page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SSH renders the SSH workflow page.
func (h *HomeHandler) SSH(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "SSH TUI - Skillf",
	}

	if err := web.RenderPage(w, "ssh.html", data); err != nil {
		h.app.Logger.Error("failed to render ssh page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
