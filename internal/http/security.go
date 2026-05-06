package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golangkanban/internal/view"
)

const (
	sessionCookieName = "gk_session"
	csrfFormField     = "_csrf"
	csrfHeaderName    = "X-CSRF-Token"
)

type csrfSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]string
}

func newCSRFSessionStore() *csrfSessionStore {
	return &csrfSessionStore{sessions: make(map[string]string)}
}

func (s *csrfSessionStore) create() (string, string, error) {
	sessionID, err := randomToken()
	if err != nil {
		return "", "", err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return "", "", err
	}
	s.mu.Lock()
	s.sessions[sessionID] = csrfToken
	s.mu.Unlock()
	return sessionID, csrfToken, nil
}

func (s *csrfSessionStore) token(sessionID string) (string, bool) {
	s.mu.RLock()
	token, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	return token, ok
}

func randomToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func securityMiddleware(store *csrfSessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		if safeMethod(r.Method) {
			token, ok := sessionTokenFromRequest(store, r)
			if !ok {
				var err error
				token, err = createSession(w, r, store)
				if err != nil {
					serverError(w, err)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(view.WithCSRFToken(r.Context(), token)))
			return
		}
		token, ok := validateUnsafeRequest(store, r)
		if !ok {
			writeForbidden(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(view.WithCSRFToken(r.Context(), token)))
	})
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy-Report-Only", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
}

func safeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func createSession(w http.ResponseWriter, r *http.Request, store *csrfSessionStore) (string, error) {
	sessionID, token, err := store.create()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   requestIsHTTPS(r),
	})
	return token, nil
}

func sessionTokenFromRequest(store *csrfSessionStore, r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return store.token(cookie.Value)
}

func validateUnsafeRequest(store *csrfSessionStore, r *http.Request) (string, bool) {
	expected, ok := sessionTokenFromRequest(store, r)
	if !ok || !validRequestOrigin(r) {
		return "", false
	}
	actual := r.Header.Get(csrfHeaderName)
	if actual == "" {
		actual = r.FormValue(csrfFormField)
	}
	if actual == "" {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return "", false
	}
	return expected, true
}

func validRequestOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return sameOrigin(r, origin)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return sameOrigin(r, referer)
	}
	return true
}

func sameOrigin(r *http.Request, value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, requestScheme(r)) && strings.EqualFold(parsed.Host, r.Host)
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
	}
	return "http"
}

func requestIsHTTPS(r *http.Request) bool {
	return requestScheme(r) == "https"
}

func writeForbidden(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}
	http.Error(w, "forbidden", http.StatusForbidden)
}
