package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// clientIP returns the address the trusted proxy chain saw. Google's external
// Application Load Balancer appends "<client-ip>, <lb-ip>" to
// X-Forwarded-For, so with hops=2 the client is the second entry from the
// right. Entries further left are supplied by the client and can be forged.
// hops=0 ignores X-Forwarded-For entirely (direct connections, LOCAL_MODE).
func clientIP(r *http.Request, hops int) string {
	if hops > 0 {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) >= hops {
				if ip := strings.TrimSpace(parts[len(parts)-hops]); net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// securityHeaders applies to every response. The UI's script and connect
// restrictions are in the CSP <meta> tag SvelteKit generates with script
// hashes; frame-ancestors only works as a header, so it lives here.
func securityHeaders(https bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "no-referrer")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'self'")
		hd.Set("Cross-Origin-Opener-Policy", "same-origin")
		hd.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		if https {
			hd.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		h.ServeHTTP(w, r)
	})
}

// logRequests emits one structured log line per request, picked up by the
// ops module's log-based metrics. Only the path is logged; no route carries a
// token in its path, and query strings are never logged.
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(sw, r)
		path := r.URL.Path
		if strings.HasPrefix(path, "/_local/") {
			path = "/_local/…"
		}
		slog.Info("http",
			"method", r.Method,
			"path", path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
