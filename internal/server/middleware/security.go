package middleware

import (
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// safeFilenameRegex matches characters safe for filenames.
var safeFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SecurityHeaders adds standard security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filter in browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Prevent caching of sensitive data
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")

		// HTTP Strict Transport Security
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content Security Policy
		// Note: Using 'unsafe-inline' and 'unsafe-eval' for HTMX and inline event handler compatibility
		// The codebase uses onclick handlers extensively, which requires this permissive policy
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com; "+
				"font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self' https://cdn.jsdelivr.net https://huggingface.co https://cdn-lfs.huggingface.co; "+
				"worker-src 'self' blob: https://cdn.jsdelivr.net; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		// Permissions Policy
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// SanitizeFilename sanitizes a string for use in Content-Disposition header.
// This prevents header injection attacks.
func SanitizeFilename(s string) string {
	// Replace any unsafe characters with underscore
	safe := safeFilenameRegex.ReplaceAllString(s, "_")

	// Ensure reasonable length
	if len(safe) > 100 {
		safe = safe[:100]
	}

	// Ensure not empty
	if safe == "" {
		safe = "download"
	}

	return safe
}

// ValidateURL checks if a URL is safe to fetch (SSRF protection).
// Returns an error message if the URL is not safe, empty string if OK.
func ValidateURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "invalid URL format"
	}

	// Only allow http and https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return "only http and https URLs are allowed"
	}

	// Block requests to private/internal networks
	host := u.Hostname()

	// Check if it's an IP address
	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return "requests to private IP addresses are not allowed"
		}
	} else {
		// It's a hostname - resolve and check
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, resolvedIP := range ips {
				if isPrivateIP(resolvedIP) {
					return "requests to private/internal hosts are not allowed"
				}
			}
		}
	}

	// Block common internal hostnames
	lowerHost := strings.ToLower(host)
	blockedHosts := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"::1",
		"metadata.google.internal",
		"169.254.169.254", // AWS/GCP metadata endpoint
	}
	for _, blocked := range blockedHosts {
		if lowerHost == blocked {
			return "requests to internal hosts are not allowed"
		}
	}

	return ""
}

// isPrivateIP checks if an IP address is private, loopback, or link-local.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check loopback
	if ip.IsLoopback() {
		return true
	}

	// Check private ranges
	if ip.IsPrivate() {
		return true
	}

	// Check link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check unspecified (0.0.0.0 or ::)
	if ip.IsUnspecified() {
		return true
	}

	// Additional check for IPv4 link-local (169.254.x.x)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}

	return false
}
