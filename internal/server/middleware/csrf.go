package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// CSRFTokenKey is the context key for the CSRF token
type CSRFTokenKey struct{}

// CSRFConfig holds CSRF middleware configuration
type CSRFConfig struct {
	Secret        []byte
	CookieName    string
	HeaderName    string
	FormFieldName string
	CookiePath    string
	Secure        bool
	SameSite      http.SameSite
	MaxAge        int
}

// DefaultCSRFConfig returns sensible defaults for CSRF protection
func DefaultCSRFConfig() CSRFConfig {
	secret := make([]byte, 32)
	rand.Read(secret)
	return CSRFConfig{
		Secret:        secret,
		CookieName:    "_csrf",
		HeaderName:    "X-CSRF-Token",
		FormFieldName: "csrf_token",
		CookiePath:    "/",
		Secure:        true,
		SameSite:      http.SameSiteStrictMode,
		MaxAge:        3600 * 12, // 12 hours
	}
}

// CSRFProtection provides CSRF protection middleware
type CSRFProtection struct {
	config CSRFConfig
	tokens sync.Map // In-memory token store for validation
}

// NewCSRFProtection creates a new CSRF protection middleware
func NewCSRFProtection(config CSRFConfig) *CSRFProtection {
	return &CSRFProtection{
		config: config,
	}
}

// generateToken creates a new CSRF token
func (c *CSRFProtection) generateToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	// Create HMAC signature
	h := hmac.New(sha256.New, c.config.Secret)
	h.Write(tokenBytes)
	sig := h.Sum(nil)

	// Combine token and signature
	combined := append(tokenBytes, sig...)
	return base64.URLEncoding.EncodeToString(combined), nil
}

// validateToken validates a CSRF token
func (c *CSRFProtection) validateToken(token string) bool {
	if token == "" {
		return false
	}

	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return false
	}

	if len(decoded) != 64 { // 32 bytes token + 32 bytes signature
		return false
	}

	tokenBytes := decoded[:32]
	providedSig := decoded[32:]

	// Recompute signature
	h := hmac.New(sha256.New, c.config.Secret)
	h.Write(tokenBytes)
	expectedSig := h.Sum(nil)

	return hmac.Equal(providedSig, expectedSig)
}

// Protect returns the CSRF protection middleware
func (c *CSRFProtection) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF check for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			// Generate token for the response
			token, err := c.generateToken()
			if err != nil {
				http.Error(w, "Failed to generate CSRF token", http.StatusInternalServerError)
				return
			}

			// Set cookie
			http.SetCookie(w, &http.Cookie{
				Name:     c.config.CookieName,
				Value:    token,
				Path:     c.config.CookiePath,
				MaxAge:   c.config.MaxAge,
				Secure:   c.config.Secure,
				HttpOnly: true,
				SameSite: c.config.SameSite,
			})

			// Make token available to templates
			w.Header().Set("X-CSRF-Token", token)

			next.ServeHTTP(w, r)
			return
		}

		// For unsafe methods, validate the token
		cookie, err := r.Cookie(c.config.CookieName)
		if err != nil {
			http.Error(w, "CSRF token missing", http.StatusForbidden)
			return
		}

		// Get token from header or form
		token := r.Header.Get(c.config.HeaderName)
		if token == "" {
			token = r.FormValue(c.config.FormFieldName)
		}

		// Validate that submitted token matches cookie
		if token != cookie.Value || !c.validateToken(token) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		// Token is valid, refresh it
		newToken, err := c.generateToken()
		if err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:     c.config.CookieName,
				Value:    newToken,
				Path:     c.config.CookiePath,
				MaxAge:   c.config.MaxAge,
				Secure:   c.config.Secure,
				HttpOnly: true,
				SameSite: c.config.SameSite,
			})
			w.Header().Set("X-CSRF-Token", newToken)
		}

		next.ServeHTTP(w, r)
	})
}

// GetToken returns the CSRF token from the request context or generates a new one
func (c *CSRFProtection) GetToken(r *http.Request) string {
	if cookie, err := r.Cookie(c.config.CookieName); err == nil {
		return cookie.Value
	}
	token, _ := c.generateToken()
	return token
}

// Token cleanup goroutine (if using in-memory storage)
func (c *CSRFProtection) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			now := time.Now()
			c.tokens.Range(func(key, value interface{}) bool {
				if expiry, ok := value.(time.Time); ok && now.After(expiry) {
					c.tokens.Delete(key)
				}
				return true
			})
		}
	}()
}
