// Package server is the single file-vault HTTP service: the embedded UI, the
// JSON API, and (in LOCAL_MODE) the local blob endpoints.
//
// Every capability token travels in a POST body, never in a URL path or
// query string, so no request log (app, Cloud Run, load balancer) ever
// records one. The UI reads tokens and the decryption key from the URL
// fragment, which browsers never send to the server.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/time/rate"

	"github.com/bcollard/file-vault/api/internal/blob"
	"github.com/bcollard/file-vault/api/internal/config"
	"github.com/bcollard/file-vault/api/internal/store"
	"github.com/bcollard/file-vault/api/internal/token"
)

const (
	maxJSONBody = 16 << 10
	// Smallest valid ciphertext: 4-byte header + one empty final record (16-byte GCM tag).
	minCiphertextBytes = 20
)

var (
	b64url   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	tokenFmt = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`) // 32 bytes, raw base64url
)

type Server struct {
	cfg   *config.Config
	store store.Store
	blobs blob.Blobs
	ui    http.Handler
	local http.Handler // nil unless LOCAL_MODE
	now   func() time.Time
}

// New wires the service. local is the blob.Local handler in LOCAL_MODE, nil
// otherwise.
func New(cfg *config.Config, st store.Store, bl blob.Blobs, ui, local http.Handler) *Server {
	return &Server{cfg: cfg, store: st, blobs: bl, ui: ui, local: local, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Server) Router() http.Handler {
	uploadLim := newRateLimiter(rate.Limit(float64(s.cfg.UploadRPM)/60.0), s.cfg.UploadBurst, 10*time.Minute, s.clientIP)
	readLim := newRateLimiter(rate.Limit(float64(s.cfg.ReadRPM)/60.0), s.cfg.ReadBurst, 10*time.Minute, s.clientIP)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/shares", uploadLim.wrap(s.handleCreateShare))
	mux.HandleFunc("POST /api/shares/finalize", readLim.wrap(s.handleFinalize))
	mux.HandleFunc("POST /api/download/peek", readLim.wrap(s.handlePeek))
	mux.HandleFunc("POST /api/download/burn", readLim.wrap(s.handleBurn))
	mux.HandleFunc("POST /api/admin", readLim.wrap(s.handleAdminGet))
	mux.HandleFunc("POST /api/admin/issue", readLim.wrap(s.handleAdminIssue))
	mux.HandleFunc("POST /api/admin/revoke", readLim.wrap(s.handleAdminRevoke))
	mux.HandleFunc("POST /api/admin/delete", readLim.wrap(s.handleAdminDelete))
	apiNotFound := func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound, "not found") }
	mux.HandleFunc("GET /api/", apiNotFound)
	mux.HandleFunc("POST /api/", apiNotFound)
	if s.local != nil {
		mux.Handle("PUT /_local/", s.local)
		mux.Handle("GET /_local/", s.local)
	}
	mux.Handle("GET /", s.ui)
	return logRequests(securityHeaders(!s.cfg.LocalMode, mux))
}

// --- handlers ---

func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{
		"default_ttl_hours":  s.cfg.DefaultTokenTTLHours,
		"min_ttl_hours":      s.cfg.MinTokenTTLHours,
		"max_ttl_hours":      s.cfg.MaxTokenTTLHours,
		"default_max_uses":   s.cfg.DefaultMaxUses,
		"max_uses_per_token": s.cfg.MaxUsesPerToken,
		"max_upload_bytes":   s.cfg.MaxUploadBytes,
		"share_ttl_hours":    int(s.cfg.ShareTTL / time.Hour),
	})
}

type createShareReq struct {
	Size     int64  `json:"size"`
	Meta     string `json:"meta"`
	TTLHours int    `json:"ttl_hours"`
	MaxUses  int    `json:"max_uses"`
}

// POST /api/shares — open a pending share and a resumable upload session.
// The browser then PUTs ciphertext straight to the bucket and calls finalize.
func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var req createShareReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.Size < minCiphertextBytes || req.Size > s.cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "size out of bounds")
		return
	}
	if req.Meta == "" || len(req.Meta) > s.cfg.MaxMetaBytes || !b64url.MatchString(req.Meta) {
		writeError(w, http.StatusBadRequest, "invalid meta")
		return
	}

	adminToken, err1 := token.New()
	object, err2 := token.New()
	if err := errors.Join(err1, err2); err != nil {
		s.serverError(w, "token", err)
		return
	}

	ctx := r.Context()
	uploadURL, err := s.blobs.StartUpload(ctx, object, req.Size)
	if err != nil {
		s.serverError(w, "start upload", err)
		return
	}

	now := s.now()
	share := store.Share{
		ObjectPath:      object,
		Meta:            req.Meta,
		Size:            req.Size,
		Status:          store.StatusPending,
		CreatedAt:       now,
		ExpiresAt:       now.Add(s.cfg.ShareTTL),
		InitialTTLHours: s.clampTTL(req.TTLHours),
		InitialMaxUses:  s.clampUses(req.MaxUses),
	}
	shareID, err := s.store.CreateShare(ctx, share, token.Hash(s.cfg.HMACPepper, adminToken))
	if err != nil {
		s.serverError(w, "create share", err)
		return
	}
	s.audit(ctx, shareID, "share_created", r, now)

	writeJSON(w, http.StatusCreated, map[string]any{
		"admin_token": adminToken,
		"upload_url":  uploadURL,
		"expires_at":  share.ExpiresAt.Format(time.RFC3339),
	})
}

type adminReq struct {
	AdminToken string `json:"admin_token"`
	TTLHours   int    `json:"ttl_hours"`
	MaxUses    int    `json:"max_uses"`
}

// POST /api/shares/finalize — confirm the ciphertext landed with the declared
// size, flip the share to ready, and mint the first download token.
func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	_, shareID, share, ok := s.resolveAdmin(w, r)
	if !ok {
		return
	}
	if share.Status != store.StatusPending {
		writeError(w, http.StatusConflict, "already finalized")
		return
	}
	ctx := r.Context()
	size, err := s.blobs.Size(ctx, share.ObjectPath)
	if errors.Is(err, blob.ErrNotFound) {
		writeError(w, http.StatusConflict, "upload incomplete")
		return
	}
	if err != nil {
		s.serverError(w, "stat object", err)
		return
	}
	if size != share.Size {
		_ = s.blobs.Delete(ctx, share.ObjectPath)
		writeError(w, http.StatusBadRequest, "size mismatch")
		return
	}

	dl, err := token.New()
	if err != nil {
		s.serverError(w, "token", err)
		return
	}
	now := s.now()
	expires := clampExpires(now, time.Duration(share.InitialTTLHours)*time.Hour, share.ExpiresAt)
	err = s.store.FinalizeShare(ctx, shareID, token.Hash(s.cfg.HMACPepper, dl), now, expires, share.InitialMaxUses)
	if errors.Is(err, store.ErrAlreadyFinal) {
		writeError(w, http.StatusConflict, "already finalized")
		return
	}
	if err != nil {
		s.serverError(w, "finalize", err)
		return
	}
	s.audit(ctx, shareID, "share_finalized", r, now)
	writeJSON(w, http.StatusOK, map[string]any{
		"download_token":      dl,
		"download_expires_at": expires.Format(time.RFC3339),
		"max_uses":            share.InitialMaxUses,
		"expires_at":          share.ExpiresAt.Format(time.RFC3339),
	})
}

type downloadReq struct {
	Token string `json:"token"`
}

func (s *Server) readDownloadToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store, private")
	var req downloadReq
	if !readJSON(w, r, &req) {
		return "", false
	}
	if !tokenFmt.MatchString(req.Token) {
		writeError(w, http.StatusNotFound, "not found")
		return "", false
	}
	return req.Token, true
}

// POST /api/download/peek — validate a download token without consuming it.
// The recipient page calls this to show what they're about to download.
func (s *Server) handlePeek(w http.ResponseWriter, r *http.Request) {
	raw, ok := s.readDownloadToken(w, r)
	if !ok {
		return
	}
	share, tok, err := s.store.PeekDownloadToken(r.Context(), token.Hash(s.cfg.HMACPepper, raw), s.now())
	if err != nil {
		// Unknown, expired, revoked and exhausted all look the same: no
		// enumeration signal.
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"meta":           share.Meta,
		"size":           share.Size,
		"expires_at":     tok.ExpiresAt.Format(time.RFC3339),
		"uses_remaining": tok.UsesRemaining,
		"max_uses":       tok.MaxUses,
	})
}

// POST /api/download/burn — atomic decrement, then a short-lived URL for the
// ciphertext. Only an explicit click on the recipient page calls this, so
// link unfurlers can't consume a use.
func (s *Server) handleBurn(w http.ResponseWriter, r *http.Request) {
	raw, ok := s.readDownloadToken(w, r)
	if !ok {
		return
	}
	now := s.now()
	shareID, share, err := s.store.BurnDownloadToken(r.Context(), token.Hash(s.cfg.HMACPepper, raw), s.clientIP(r), now)
	if err != nil {
		slog.Info("token_reuse_rejected",
			"event", "token_reuse_rejected",
			"ip", s.clientIP(r),
			"reason", classifyTokenErr(err),
		)
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	url, err := s.blobs.DownloadURL(r.Context(), share.ObjectPath, s.cfg.DownloadURLTTL)
	if err != nil {
		s.serverError(w, "sign url", err)
		return
	}
	s.audit(r.Context(), shareID, "token_consumed", r, now)
	writeJSON(w, http.StatusOK, map[string]any{"url": url, "size": share.Size})
}

// POST /api/admin — share metadata plus every download token.
func (s *Server) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	_, shareID, share, ok := s.resolveAdmin(w, r)
	if !ok {
		return
	}
	tokens, err := s.store.ListDownloadTokens(r.Context(), shareID, s.now())
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
	writeJSON(w, http.StatusOK, map[string]any{
		"share_id":   shareID,
		"meta":       share.Meta,
		"size":       share.Size,
		"status":     share.Status,
		"created_at": share.CreatedAt.Format(time.RFC3339),
		"expires_at": share.ExpiresAt.Format(time.RFC3339),
		"tokens":     out,
	})
}

// POST /api/admin/issue — mint another download token, clamped to config
// bounds and to the share's own expiry.
func (s *Server) handleAdminIssue(w http.ResponseWriter, r *http.Request) {
	req, shareID, share, ok := s.resolveAdmin(w, r)
	if !ok {
		return
	}
	if share.Status != store.StatusReady {
		writeError(w, http.StatusConflict, "upload not finalized")
		return
	}
	dl, err := token.New()
	if err != nil {
		s.serverError(w, "token", err)
		return
	}
	now := s.now()
	maxUses := s.clampUses(req.MaxUses)
	expires := clampExpires(now, time.Duration(s.clampTTL(req.TTLHours))*time.Hour, share.ExpiresAt)
	if err := s.store.IssueDownloadToken(r.Context(), shareID, token.Hash(s.cfg.HMACPepper, dl), now, expires, maxUses); err != nil {
		s.serverError(w, "issue token", err)
		return
	}
	s.audit(r.Context(), shareID, "token_issued", r, now)
	writeJSON(w, http.StatusCreated, map[string]any{
		"download_token": dl,
		"expires_at":     expires.Format(time.RFC3339),
		"max_uses":       maxUses,
	})
}

// POST /api/admin/revoke — revoke every outstanding download token.
func (s *Server) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	_, shareID, _, ok := s.resolveAdmin(w, r)
	if !ok {
		return
	}
	now := s.now()
	n, err := s.store.RevokeOutstandingTokens(r.Context(), shareID, now)
	if err != nil {
		s.serverError(w, "revoke", err)
		return
	}
	s.audit(r.Context(), shareID, "tokens_revoked", r, now)
	writeJSON(w, http.StatusOK, map[string]any{"revoked": n})
}

// POST /api/admin/delete — drop the object, the share and every token.
func (s *Server) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	req, shareID, share, ok := s.resolveAdmin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := s.blobs.Delete(ctx, share.ObjectPath); err != nil {
		s.serverError(w, "blob delete", err)
		return
	}
	if err := s.store.DeleteShare(ctx, shareID, token.Hash(s.cfg.HMACPepper, req.AdminToken)); err != nil {
		s.serverError(w, "store delete", err)
		return
	}
	s.audit(ctx, shareID, "share_deleted", r, s.now())
	w.WriteHeader(http.StatusNoContent)
}

// resolveAdmin decodes an adminReq and looks up the share. On failure it has
// already written the response (404 for anything token-related).
func (s *Server) resolveAdmin(w http.ResponseWriter, r *http.Request) (adminReq, string, *store.Share, bool) {
	w.Header().Set("Cache-Control", "no-store, private")
	var req adminReq
	if !readJSON(w, r, &req) {
		return req, "", nil, false
	}
	if !tokenFmt.MatchString(req.AdminToken) {
		writeError(w, http.StatusNotFound, "not found")
		return req, "", nil, false
	}
	ctx := r.Context()
	shareID, err := s.store.LookupAdmin(ctx, token.Hash(s.cfg.HMACPepper, req.AdminToken))
	if err != nil {
		s.notFoundOrError(w, err)
		return req, "", nil, false
	}
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		s.notFoundOrError(w, err)
		return req, "", nil, false
	}
	return req, shareID, share, true
}

func (s *Server) notFoundOrError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrExpired) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	s.serverError(w, "lookup", err)
}

func (s *Server) audit(ctx context.Context, shareID, event string, r *http.Request, now time.Time) {
	err := s.store.RecordAudit(ctx, store.AuditEvent{
		ShareID:   shareID,
		Event:     event,
		IP:        s.clientIP(r),
		At:        now,
		ExpiresAt: now.Add(s.cfg.AuditRetention),
	})
	if err != nil {
		slog.Warn("audit write failed", "event", event, "err", err)
	}
}

func (s *Server) serverError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func (s *Server) clientIP(r *http.Request) string {
	return clientIP(r, s.cfg.TrustedProxyHop)
}

func (s *Server) clampTTL(h int) int {
	if h == 0 {
		h = s.cfg.DefaultTokenTTLHours
	}
	return clampInt(h, s.cfg.MinTokenTTLHours, s.cfg.MaxTokenTTLHours)
}

func (s *Server) clampUses(n int) int {
	if n == 0 {
		n = s.cfg.DefaultMaxUses
	}
	return clampInt(n, 1, s.cfg.MaxUsesPerToken)
}

// --- helpers ---

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
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
	case errors.Is(err, store.ErrNotReady):
		return "not_ready"
	default:
		return "internal"
	}
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func clampExpires(now time.Time, requested time.Duration, hardCap time.Time) time.Time {
	exp := now.Add(requested)
	if exp.After(hardCap) {
		return hardCap
	}
	return exp
}
