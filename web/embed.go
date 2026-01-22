// Package web provides embedded web assets.
package web

import (
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// htmlPolicy is the HTML sanitization policy for user content
var htmlPolicy = bluemonday.UGCPolicy()

//go:embed templates/* static/*
var content embed.FS

// StaticFS provides access to static files.
var StaticFS fs.FS

// templates holds parsed templates.
var templates *template.Template

func init() {
	var err error
	StaticFS, err = fs.Sub(content, "static")
	if err != nil {
		panic(err)
	}

	// Parse all templates
	templates = template.New("")

	// Add template functions
	templates.Funcs(template.FuncMap{
		"safe": func(s string) template.HTML {
			// Sanitize HTML to prevent XSS while allowing safe formatting
			return template.HTML(htmlPolicy.Sanitize(s))
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"join": strings.Join,
		"contains": strings.Contains,
	})

	// Parse layouts
	templates, err = templates.ParseFS(content, "templates/layouts/*.html")
	if err != nil {
		panic(err)
	}

	// Parse pages
	templates, err = templates.ParseFS(content, "templates/pages/*.html")
	if err != nil {
		panic(err)
	}

	// Parse partials
	templates, err = templates.ParseFS(content, "templates/partials/*.html")
	if err != nil {
		panic(err)
	}
}

// RenderPage renders a full page with the base layout.
func RenderPage(w io.Writer, name string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Page"] = name

	// Clone template for thread safety
	tmpl, err := templates.Clone()
	if err != nil {
		return err
	}

	return tmpl.ExecuteTemplate(w, "base.html", data)
}

// RenderPartial renders a partial template (for HTMX responses).
func RenderPartial(w io.Writer, name string, data interface{}) error {
	tmpl, err := templates.Clone()
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, name, data)
}

// ServeStatic returns an HTTP handler for static files.
func ServeStatic() http.Handler {
	return http.FileServer(http.FS(StaticFS))
}
