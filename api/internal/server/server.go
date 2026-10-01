package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/time/rate"

	"github.com/bcollard/file-vault/api/internal/config"
	"github.com/bcollard/file-vault/api/internal/signer"
	"github.com/bcollard/file-vault/api/internal/store"
	"github.com/bcollard/file-vault/api/internal/token"
)

const (
	maxUploadBytes = 32 << 20 // 32 MiB — Cloud Run default request size limit.
	maxFilenameLen = 200
)

type Server struct {
	cfg    *config.Config
	store  *store.Store
	signer *signer.Signer
	gcs    *storage.Client
}

func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	st, err := store.New(ctx, cfg.ProjectID, cfg.FirestoreDatabase)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	sg, err := signer.New(ctx, cfg.BucketName, cfg.SignerServiceAccount)
	if err != nil {
		return nil, fmt.Errorf("signer: %w", err)
	}
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: %w", err)
	}
	return &Server{cfg: cfg, store: st, signer: sg, gcs: gcs}, nil
}

func (s *Server) Close() {
	_ = s.store.Close()
	_ = s.gcs.Close()
}

func (s *Server) Router() http.Handler {
	uploadLim := newRateLimiter(rate.Limit(float64(s.cfg.UploadRPM)/60.0), s.cfg.UploadBurst, 10*time.Minute)
	readLim := newRateLimiter(rate.Limit(float64(s.cfg.ReadRPM)/60.0), s.cfg.ReadBurst, 10*time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /api/shares", uploadLim.middleware(s.handleUpload))
	mux.HandleFunc("GET /d/{token}", readLim.middleware(s.handleDownloadConfirm))
	mux.HandleFunc("POST /d/{token}/confirm", readLim.middleware(s.handleDownloadBurn))
	mux.HandleFunc("GET /api/admin/{admin}", readLim.middleware(s.handleAdminGet))
	mux.HandleFunc("GET /api/admin/{admin}/tokens", readLim.middleware(s.handleAdminListTokens))
	mux.HandleFunc("POST /api/admin/{admin}/tokens", readLim.middleware(s.handleAdminIssue))
	mux.HandleFunc("POST /api/admin/{admin}/revoke", readLim.middleware(s.handleAdminRevoke))
	mux.HandleFunc("DELETE /api/admin/{admin}", readLim.middleware(s.handleAdminDelete))
	return logRequests(mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// POST /api/shares — single-file multipart upload.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		slog.Info("upload_rejected", "reason", "parse_form", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		slog.Info("upload_rejected", "reason", "missing_file_field", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := safeFilename(header.Filename)
	if filename == "" {
		slog.Info("upload_rejected", "reason", "invalid_filename")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ttlHours := clampInt(
		parseInt(r.FormValue("ttl_hours"), s.cfg.DefaultTokenTTLHours),
		s.cfg.MinTokenTTLHours, s.cfg.MaxTokenTTLHours,
	)
	maxUses := clampInt(
		parseInt(r.FormValue("max_uses"), s.cfg.DefaultMaxUses),
		1, s.cfg.MaxUsesPerToken,
	)

	dlToken, err := token.New()
	if err != nil {
		s.serverError(w, "token", err)
		return
	}
	adminToken, err := token.New()
	if err != nil {
		s.serverError(w, "token", err)
		return
	}
	objectPrefix, err := token.New()
	if err != nil {
		s.serverError(w, "object prefix", err)
		return
	}
	objectPath := objectPrefix + "/" + filename

	ctx := r.Context()
	obj := s.gcs.Bucket(s.cfg.BucketName).Object(objectPath)
	wc := obj.NewWriter(ctx)
	wc.ContentType = header.Header.Get("Content-Type")
	if _, err := io.Copy(wc, file); err != nil {
		_ = wc.Close()
		_ = obj.Delete(context.Background())
		s.serverError(w, "gcs upload", err)
		return
	}
	if err := wc.Close(); err != nil {
		_ = obj.Delete(context.Background())
		s.serverError(w, "gcs close", err)
		return
	}

	now := time.Now().UTC()
	share := store.Share{
		ObjectPath:  objectPath,
		Filename:    filename,
		Size:        header.Size,
		ContentType: wc.ContentType,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.cfg.ShareTTL),
	}
	dlHash := token.Hash(s.cfg.HMACPepper, dlToken)
	adminHash := token.Hash(s.cfg.HMACPepper, adminToken)
	dlExpires := clampExpires(now, time.Duration(ttlHours)*time.Hour, share.ExpiresAt)

	shareID, err := s.store.CreateShare(ctx, share, dlHash, adminHash, dlExpires, maxUses)
	if err != nil {
		_ = obj.Delete(context.Background())
		s.serverError(w, "create share", err)
		return
	}
	_ = s.store.RecordAudit(ctx, store.AuditEvent{
		ShareID: shareID, Event: "share_created", IP: clientIP(r), At: now,
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"share_id":               shareID,
		"filename":               filename,
		"size":                   header.Size,
		"expires_at":             share.ExpiresAt.Format(time.RFC3339),
		"download_token":         dlToken,
		"download_token_expires": dlExpires.Format(time.RFC3339),
		"download_token_max":     maxUses,
		"admin_token":            adminToken,
	})
}

// GET /d/{token} — render confirmation HTML without touching the token.
// Link unfurlers (Slack, email gateways, Outlook Safe Links) only ever GET,
// so the token survives until the human clicks the form button.
// HEAD is served by the same handler (Go's ServeMux maps HEAD → GET);
// we return 200 with no body and no state change.
func (s *Server) handleDownloadConfirm(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("token")
	if raw == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodHead {
		w.Header().Set("Cache-Control", "no-store, private")
		w.WriteHeader(http.StatusOK)
		return
	}

	hash := token.Hash(s.cfg.HMACPepper, raw)
	share, err := s.store.PeekDownloadToken(r.Context(), hash, time.Now().UTC())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderConfirm(w, raw, share.Filename, share.Size)
}

// POST /d/{token}/confirm — atomic burn + 303 to a 60s V4 signed URL.
// 303 See Other forces the browser to issue a GET for the redirected URL,
// which is what GCS expects for the signed download.
// All burn failures collapse to 404 to avoid leaking token existence.
func (s *Server) handleDownloadBurn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private")

	raw := r.PathValue("token")
	if raw == "" {
		http.NotFound(w, r)
		return
	}

	ip := clientIP(r)
	now := time.Now().UTC()
	hash := token.Hash(s.cfg.HMACPepper, raw)

	shareID, share, err := s.store.BurnDownloadToken(r.Context(), hash, ip, now)
	if err != nil {
		slog.Info("token_reuse_rejected",
			"event", "token_reuse_rejected",
			"ip", ip,
			"reason", classifyTokenErr(err),
		)
		http.NotFound(w, r)
		return
	}

	url, err := s.signer.SignedDownloadURL(share.ObjectPath, share.Filename, s.cfg.DownloadURLTTL)
	if err != nil {
		s.serverError(w, "sign url", err)
		return
	}

	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{
		ShareID: shareID, Event: "token_consumed", IP: ip, At: now,
	})

	http.Redirect(w, r, url, http.StatusSeeOther)
}

// GET /api/admin/{admin} — return share metadata for the admin console.
func (s *Server) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	shareID, share, ok := s.resolveAdmin(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"share_id":     shareID,
		"filename":     share.Filename,
		"size":         share.Size,
		"content_type": share.ContentType,
		"created_at":   share.CreatedAt.Format(time.RFC3339),
		"expires_at":   share.ExpiresAt.Format(time.RFC3339),
	})
}

// GET /api/admin/{admin}/tokens — list every download token bound to the share.
func (s *Server) handleAdminListTokens(w http.ResponseWriter, r *http.Request) {
	shareID, _, ok := s.resolveAdmin(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tokens, err := s.store.ListDownloadTokens(r.Context(), shareID, time.Now().UTC())
	if err != nil {
		s.serverError(w, "list tokens", err)
		return
	}
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		item := map[string]any{
			"short_id":       t.ShortID,
			"status":         t.Status,
			"created_at":     t.CreatedAt.Format(time.RFC3339),
			"expires_at":     t.ExpiresAt.Format(time.RFC3339),
			"max_uses":       t.MaxUses,
			"uses_remaining": t.UsesRemaining,
		}
		if t.FirstUsedAt != nil {
			item["first_used_at"] = t.FirstUsedAt.Format(time.RFC3339)
		}
		if t.LastUsedAt != nil {
			item["last_used_at"] = t.LastUsedAt.Format(time.RFC3339)
			item["last_used_from_ip"] = t.LastUsedFromIP
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/admin/{admin}/tokens — issue a fresh download token, ttl_hours +
// max_uses are admin-controlled and clamped to config bounds (and share TTL).
func (s *Server) handleAdminIssue(w http.ResponseWriter, r *http.Request) {
	shareID, share, ok := s.resolveAdmin(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ttlHours := clampInt(
		parseInt(r.FormValue("ttl_hours"), s.cfg.DefaultTokenTTLHours),
		s.cfg.MinTokenTTLHours, s.cfg.MaxTokenTTLHours,
	)
	maxUses := clampInt(
		parseInt(r.FormValue("max_uses"), s.cfg.DefaultMaxUses),
		1, s.cfg.MaxUsesPerToken,
	)

	dl, err := token.New()
	if err != nil {
		s.serverError(w, "token", err)
		return
	}
	now := time.Now().UTC()
	expires := clampExpires(now, time.Duration(ttlHours)*time.Hour, share.ExpiresAt)
	if err := s.store.IssueDownloadToken(r.Context(), shareID, token.Hash(s.cfg.HMACPepper, dl), now, expires, maxUses); err != nil {
		s.serverError(w, "issue token", err)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{
		ShareID: shareID, Event: "token_issued", IP: clientIP(r), At: now,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"download_token": dl,
		"expires_at":     expires.Format(time.RFC3339),
		"max_uses":       maxUses,
	})
}

// POST /api/admin/{admin}/revoke — revoke all outstanding download tokens.
func (s *Server) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	shareID, _, ok := s.resolveAdmin(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, err := s.store.RevokeOutstandingTokens(r.Context(), shareID, time.Now().UTC())
	if err != nil {
		s.serverError(w, "revoke", err)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{
		ShareID: shareID, Event: "tokens_revoked", IP: clientIP(r), At: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"revoked": n})
}

// DELETE /api/admin/{admin} — drop the share, the admin token, all download
// tokens, and the underlying GCS object.
func (s *Server) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("admin")
	adminHash := token.Hash(s.cfg.HMACPepper, raw)
	shareID, err := s.store.LookupAdmin(r.Context(), adminHash)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	share, err := s.store.GetShare(r.Context(), shareID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.gcs.Bucket(s.cfg.BucketName).Object(share.ObjectPath).Delete(r.Context()); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		s.serverError(w, "gcs delete", err)
		return
	}
	if err := s.store.DeleteShare(r.Context(), shareID, adminHash); err != nil {
		s.serverError(w, "firestore delete", err)
		return
	}
	_ = s.store.RecordAudit(r.Context(), store.AuditEvent{
		ShareID: shareID, Event: "share_deleted", IP: clientIP(r), At: time.Now().UTC(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// resolveAdmin looks up the share behind an admin path param. Returns ok=false
// if the token doesn't match anything; the caller should 404.
func (s *Server) resolveAdmin(r *http.Request) (shareID string, share *store.Share, ok bool) {
	raw := r.PathValue("admin")
	if raw == "" {
		return "", nil, false
	}
	hash := token.Hash(s.cfg.HMACPepper, raw)
	shareID, err := s.store.LookupAdmin(r.Context(), hash)
	if err != nil {
		return "", nil, false
	}
	share, err = s.store.GetShare(r.Context(), shareID)
	if err != nil {
		return "", nil, false
	}
	return shareID, share, true
}

func (s *Server) serverError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}

func safeFilename(name string) string {
	name = path.Base(name)
	if name == "." || name == "/" || name == "" {
		return ""
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return ""
	}
	if len(name) > maxFilenameLen {
		name = name[len(name)-maxFilenameLen:]
	}
	return name
}

func classifyTokenErr(err error) string {
	switch {
	case errors.Is(err, store.ErrExhausted):
		return "exhausted"
	case errors.Is(err, store.ErrRevoked):
		return "revoked"
	case errors.Is(err, store.ErrExpired):
		return "expired"
	case errors.Is(err, store.ErrNotFound):
		return "not_found"
	default:
		return "internal"
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampExpires(now time.Time, requested time.Duration, hardCap time.Time) time.Time {
	exp := now.Add(requested)
	if exp.After(hardCap) {
		return hardCap
	}
	return exp
}

// logRequests emits one structured log line per request — picked up by the ops
// module's log-based metrics.
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
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
