package api

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SecurityHeaders sets headers every response should carry. The API only answers
// JSON, so the policy forbids loading or framing anything.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("Cache-Control", "no-store")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS allows cross-origin calls only from the listed origins. The app is served
// from the same origin as the API, so the default (no list) allows none.
func CORS(allowed []string) func(http.Handler) http.Handler {
	set := map[string]bool{}
	for _, o := range allowed {
		set[strings.TrimRight(strings.TrimSpace(o), "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if origin != "" && set[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SameOriginWrites refuses state-changing requests that a browser marks as coming
// from another site. The session cookie is SameSite=Lax; this is the second layer.
// Requests without Origin or Sec-Fetch-Site (scripts, curl) are not browser
// cross-site requests and pass; they still need a valid session.
func SameOriginWrites(allowed []string) func(http.Handler) http.Handler {
	set := map[string]bool{}
	for _, o := range allowed {
		set[strings.TrimRight(strings.TrimSpace(o), "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if fs := r.Header.Get("Sec-Fetch-Site"); fs == "cross-site" || fs == "same-site" {
				if origin := r.Header.Get("Origin"); !set[origin] {
					writeJSONError(w, http.StatusForbidden, "cross-site request refused")
					return
				}
			}
			if origin := r.Header.Get("Origin"); origin != "" && !set[origin] {
				u, err := url.Parse(origin)
				if err != nil || u.Host == "" || (u.Host != r.Host && u.Host != r.Header.Get("X-Forwarded-Host")) {
					writeJSONError(w, http.StatusForbidden, "cross-site request refused")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit allows burst requests at once per client address and refills one
// request every every. It is in memory, so with several pods each pod counts alone.
func RateLimit(burst int, every time.Duration, now func() time.Time) func(http.Handler) http.Handler {
	type bucket struct {
		tokens float64
		last   time.Time
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if i := strings.LastIndex(key, ":"); i > 0 {
				key = key[:i]
			}
			t := now()
			mu.Lock()
			b := buckets[key]
			if b == nil {
				if len(buckets) > 10000 { // never let the map grow without bound
					buckets = map[string]*bucket{}
				}
				b = &bucket{tokens: float64(burst), last: t}
				buckets[key] = b
			}
			b.tokens += float64(t.Sub(b.last)) / float64(every)
			if b.tokens > float64(burst) {
				b.tokens = float64(burst)
			}
			b.last = t
			ok := b.tokens >= 1
			if ok {
				b.tokens--
			}
			mu.Unlock()
			if !ok {
				w.Header().Set("Retry-After", "30")
				writeJSONError(w, http.StatusTooManyRequests, "too many requests, try again shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
