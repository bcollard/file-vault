// Package store persists shares and their capability tokens. Two backends
// implement Store: Firestore (production) and an in-memory map (LOCAL_MODE and
// tests). Both must give the same answers for the same calls; the shared test
// suite in store_test.go runs against each.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrExhausted    = errors.New("uses exhausted")
	ErrRevoked      = errors.New("revoked")
	ErrExpired      = errors.New("expired")
	ErrNotReady     = errors.New("share upload not finalized")
	ErrAlreadyFinal = errors.New("share already finalized")
)

const (
	StatusPending = "pending"
	StatusReady   = "ready"
)

const (
	CollShares         = "shares"
	CollDownloadTokens = "download_tokens"
	CollAdminTokens    = "admin_tokens"
	CollAuditEvents    = "audit_events"
)

// Share is one uploaded object. The server never sees plaintext: Meta is the
// browser-encrypted {name, type, size} blob and Size is the ciphertext length.
type Share struct {
	ObjectPath      string    `firestore:"object_path"`
	Meta            string    `firestore:"meta"`
	Size            int64     `firestore:"size"`
	Status          string    `firestore:"status"`
	CreatedAt       time.Time `firestore:"created_at"`
	ExpiresAt       time.Time `firestore:"expires_at"`
	InitialTTLHours int       `firestore:"initial_ttl_hours"`
	InitialMaxUses  int       `firestore:"initial_max_uses"`
}

// DownloadToken models max_uses + uses_remaining. Burn = atomic decrement.
type DownloadToken struct {
	ShareID        string     `firestore:"share_id"`
	CreatedAt      time.Time  `firestore:"created_at"`
	ExpiresAt      time.Time  `firestore:"expires_at"`
	MaxUses        int        `firestore:"max_uses"`
	UsesRemaining  int        `firestore:"uses_remaining"`
	FirstUsedAt    *time.Time `firestore:"first_used_at,omitempty"`
	LastUsedAt     *time.Time `firestore:"last_used_at,omitempty"`
	LastUsedFromIP string     `firestore:"last_used_from_ip,omitempty"`
	Revoked        bool       `firestore:"revoked,omitempty"`
}

type AdminToken struct {
	ShareID   string    `firestore:"share_id"`
	CreatedAt time.Time `firestore:"created_at"`
	ExpiresAt time.Time `firestore:"expires_at"`
}

// AuditEvent carries its own ExpiresAt so Firestore TTL drops it (and the IP
// it holds) after the retention window.
type AuditEvent struct {
	ShareID   string    `firestore:"share_id"`
	Event     string    `firestore:"event"`
	IP        string    `firestore:"ip,omitempty"`
	At        time.Time `firestore:"at"`
	ExpiresAt time.Time `firestore:"expires_at"`
}

// TokenStatus is the admin-facing view of a download token. ShortID is the
// first 8 chars of the HMAC hash — stable enough to identify a row in the UI,
// useless to an attacker (the raw token isn't recoverable).
type TokenStatus struct {
	ShortID        string
	Status         string // "active" | "exhausted" | "revoked" | "expired"
	CreatedAt      time.Time
	ExpiresAt      time.Time
	MaxUses        int
	UsesRemaining  int
	FirstUsedAt    *time.Time
	LastUsedAt     *time.Time
	LastUsedFromIP string
}

type Store interface {
	// CreateShare writes a pending share and its admin token. The first
	// download token is only minted by FinalizeShare, once the ciphertext is
	// confirmed in the bucket.
	CreateShare(ctx context.Context, share Share, adminHash string) (shareID string, err error)
	// FinalizeShare flips pending → ready and creates the first download
	// token in one transaction. Returns ErrAlreadyFinal on a second call.
	FinalizeShare(ctx context.Context, shareID, downloadHash string, now, downloadExpires time.Time, maxUses int) error
	// PeekDownloadToken validates a token without consuming it.
	PeekDownloadToken(ctx context.Context, hash string, now time.Time) (*Share, *DownloadToken, error)
	// BurnDownloadToken atomically decrements uses_remaining. Every failure
	// mode means "do not hand out a URL".
	BurnDownloadToken(ctx context.Context, hash, ip string, now time.Time) (shareID string, share *Share, err error)
	LookupAdmin(ctx context.Context, hash string) (shareID string, err error)
	GetShare(ctx context.Context, shareID string) (*Share, error)
	ListDownloadTokens(ctx context.Context, shareID string, now time.Time) ([]TokenStatus, error)
	IssueDownloadToken(ctx context.Context, shareID, hash string, now, expires time.Time, maxUses int) error
	RevokeOutstandingTokens(ctx context.Context, shareID string, now time.Time) (int, error)
	// DeleteShare removes share + admin token + all download tokens. The
	// caller deletes the blob first.
	DeleteShare(ctx context.Context, shareID, adminHash string) error
	RecordAudit(ctx context.Context, e AuditEvent) error
	Close() error
}

func classifyToken(t DownloadToken, now time.Time) string {
	switch {
	case t.Revoked:
		return "revoked"
	case now.After(t.ExpiresAt):
		return "expired"
	case t.UsesRemaining <= 0:
		return "exhausted"
	default:
		return "active"
	}
}

// checkUsable returns the error matching classifyToken, or nil if active.
func checkUsable(t DownloadToken, now time.Time) error {
	switch classifyToken(t, now) {
	case "revoked":
		return ErrRevoked
	case "expired":
		return ErrExpired
	case "exhausted":
		return ErrExhausted
	}
	return nil
}

func shortID(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func toStatus(hash string, t DownloadToken, now time.Time) TokenStatus {
	return TokenStatus{
		ShortID:        shortID(hash),
		Status:         classifyToken(t, now),
		CreatedAt:      t.CreatedAt,
		ExpiresAt:      t.ExpiresAt,
		MaxUses:        t.MaxUses,
		UsesRemaining:  t.UsesRemaining,
		FirstUsedAt:    t.FirstUsedAt,
		LastUsedAt:     t.LastUsedAt,
		LastUsedFromIP: t.LastUsedFromIP,
	}
}
