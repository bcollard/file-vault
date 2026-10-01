package store

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrExhausted = errors.New("uses exhausted")
	ErrRevoked   = errors.New("revoked")
	ErrExpired   = errors.New("expired")
)

const (
	CollShares         = "shares"
	CollDownloadTokens = "download_tokens"
	CollAdminTokens    = "admin_tokens"
	CollAuditEvents    = "audit_events"
)

type Share struct {
	ObjectPath  string    `firestore:"object_path"`
	Filename    string    `firestore:"filename"`
	Size        int64     `firestore:"size"`
	ContentType string    `firestore:"content_type"`
	CreatedAt   time.Time `firestore:"created_at"`
	ExpiresAt   time.Time `firestore:"expires_at"`
}

// DownloadToken now models max_uses + uses_remaining instead of a single
// consumed_at boolean. Burn = atomic decrement.
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

type AuditEvent struct {
	ShareID string    `firestore:"share_id"`
	Event   string    `firestore:"event"`
	IP      string    `firestore:"ip,omitempty"`
	At      time.Time `firestore:"at"`
}

type Store struct {
	c *firestore.Client
}

func New(ctx context.Context, projectID, database string) (*Store, error) {
	var (
		c   *firestore.Client
		err error
	)
	if database == "" || database == "(default)" {
		c, err = firestore.NewClient(ctx, projectID)
	} else {
		c, err = firestore.NewClientWithDatabase(ctx, projectID, database)
	}
	if err != nil {
		return nil, err
	}
	return &Store{c: c}, nil
}

func (s *Store) Close() error { return s.c.Close() }

// CreateShare writes the share + first download token + admin token in one
// transaction. Returns the auto-generated share_id.
func (s *Store) CreateShare(ctx context.Context, share Share, downloadHash, adminHash string, downloadExpires time.Time, maxUses int) (string, error) {
	shareRef := s.c.Collection(CollShares).NewDoc()
	shareID := shareRef.ID

	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Create(shareRef, share); err != nil {
			return err
		}
		if err := tx.Create(s.c.Collection(CollDownloadTokens).Doc(downloadHash), DownloadToken{
			ShareID:       shareID,
			CreatedAt:     share.CreatedAt,
			ExpiresAt:     downloadExpires,
			MaxUses:       maxUses,
			UsesRemaining: maxUses,
		}); err != nil {
			return err
		}
		return tx.Create(s.c.Collection(CollAdminTokens).Doc(adminHash), AdminToken{
			ShareID:   shareID,
			CreatedAt: share.CreatedAt,
			ExpiresAt: share.ExpiresAt,
		})
	})
	if err != nil {
		return "", err
	}
	return shareID, nil
}

// BurnDownloadToken atomically decrements uses_remaining and returns the bound
// share. Failure modes (ErrNotFound, ErrRevoked, ErrExhausted, ErrExpired) all
// mean "do not redirect."
func (s *Store) BurnDownloadToken(ctx context.Context, hash, ip string, now time.Time) (shareID string, share *Share, err error) {
	tokenRef := s.c.Collection(CollDownloadTokens).Doc(hash)

	err = s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(tokenRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrNotFound
			}
			return err
		}
		var tok DownloadToken
		if err := snap.DataTo(&tok); err != nil {
			return err
		}
		if tok.Revoked {
			return ErrRevoked
		}
		if now.After(tok.ExpiresAt) {
			return ErrExpired
		}
		if tok.UsesRemaining <= 0 {
			return ErrExhausted
		}

		updates := []firestore.Update{
			{Path: "uses_remaining", Value: tok.UsesRemaining - 1},
			{Path: "last_used_at", Value: now},
			{Path: "last_used_from_ip", Value: ip},
		}
		if tok.FirstUsedAt == nil {
			updates = append(updates, firestore.Update{Path: "first_used_at", Value: now})
		}
		if err := tx.Update(tokenRef, updates); err != nil {
			return err
		}

		shareSnap, err := tx.Get(s.c.Collection(CollShares).Doc(tok.ShareID))
		if err != nil {
			return err
		}
		var sh Share
		if err := shareSnap.DataTo(&sh); err != nil {
			return err
		}
		shareID = tok.ShareID
		share = &sh
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return shareID, share, nil
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

// ListDownloadTokens returns every download token bound to the share, newest first.
func (s *Store) ListDownloadTokens(ctx context.Context, shareID string, now time.Time) ([]TokenStatus, error) {
	iter := s.c.Collection(CollDownloadTokens).
		Where("share_id", "==", shareID).
		OrderBy("created_at", firestore.Desc).
		Documents(ctx)
	defer iter.Stop()

	var out []TokenStatus
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var tok DownloadToken
		if err := snap.DataTo(&tok); err != nil {
			return nil, err
		}
		short := snap.Ref.ID
		if len(short) > 8 {
			short = short[:8]
		}
		out = append(out, TokenStatus{
			ShortID:        short,
			Status:         classifyToken(tok, now),
			CreatedAt:      tok.CreatedAt,
			ExpiresAt:      tok.ExpiresAt,
			MaxUses:        tok.MaxUses,
			UsesRemaining:  tok.UsesRemaining,
			FirstUsedAt:    tok.FirstUsedAt,
			LastUsedAt:     tok.LastUsedAt,
			LastUsedFromIP: tok.LastUsedFromIP,
		})
	}
	return out, nil
}

// PeekDownloadToken reads token + share without mutating state. Used by the
// download confirmation page so link unfurlers (Slack, email gateways, etc.)
// don't burn a use before the human clicks.
func (s *Store) PeekDownloadToken(ctx context.Context, hash string, now time.Time) (*Share, error) {
	snap, err := s.c.Collection(CollDownloadTokens).Doc(hash).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var tok DownloadToken
	if err := snap.DataTo(&tok); err != nil {
		return nil, err
	}
	if tok.Revoked {
		return nil, ErrRevoked
	}
	if now.After(tok.ExpiresAt) {
		return nil, ErrExpired
	}
	if tok.UsesRemaining <= 0 {
		return nil, ErrExhausted
	}
	return s.GetShare(ctx, tok.ShareID)
}

// LookupAdmin returns the share_id behind an admin token hash.
func (s *Store) LookupAdmin(ctx context.Context, hash string) (string, error) {
	snap, err := s.c.Collection(CollAdminTokens).Doc(hash).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", ErrNotFound
		}
		return "", err
	}
	var at AdminToken
	if err := snap.DataTo(&at); err != nil {
		return "", err
	}
	return at.ShareID, nil
}

func (s *Store) GetShare(ctx context.Context, shareID string) (*Share, error) {
	snap, err := s.c.Collection(CollShares).Doc(shareID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var sh Share
	if err := snap.DataTo(&sh); err != nil {
		return nil, err
	}
	return &sh, nil
}

// IssueDownloadToken adds a new download token to an existing share.
func (s *Store) IssueDownloadToken(ctx context.Context, shareID, hash string, now, expires time.Time, maxUses int) error {
	_, err := s.c.Collection(CollDownloadTokens).Doc(hash).Create(ctx, DownloadToken{
		ShareID:       shareID,
		CreatedAt:     now,
		ExpiresAt:     expires,
		MaxUses:       maxUses,
		UsesRemaining: maxUses,
	})
	return err
}

// RevokeOutstandingTokens flips revoked=true on every token still usable
// (active or partially-consumed but not yet exhausted/expired/revoked).
// Filter happens client-side so we don't need an inequality composite index.
func (s *Store) RevokeOutstandingTokens(ctx context.Context, shareID string, now time.Time) (int, error) {
	iter := s.c.Collection(CollDownloadTokens).
		Where("share_id", "==", shareID).
		Documents(ctx)
	defer iter.Stop()

	bw := s.c.BulkWriter(ctx)
	count := 0
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			bw.End()
			return count, err
		}
		var tok DownloadToken
		if err := snap.DataTo(&tok); err != nil {
			bw.End()
			return count, err
		}
		if classifyToken(tok, now) != "active" {
			continue
		}
		if _, err := bw.Update(snap.Ref, []firestore.Update{{Path: "revoked", Value: true}}); err != nil {
			bw.End()
			return count, err
		}
		count++
	}
	bw.End()
	return count, nil
}

// DeleteShare removes the share + admin token + all download tokens. The caller
// is responsible for deleting the GCS object first.
func (s *Store) DeleteShare(ctx context.Context, shareID, adminHash string) error {
	bw := s.c.BulkWriter(ctx)
	bw.Delete(s.c.Collection(CollShares).Doc(shareID))
	bw.Delete(s.c.Collection(CollAdminTokens).Doc(adminHash))

	iter := s.c.Collection(CollDownloadTokens).Where("share_id", "==", shareID).Documents(ctx)
	defer iter.Stop()
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			bw.End()
			return err
		}
		bw.Delete(snap.Ref)
	}
	bw.End()
	return nil
}

func (s *Store) RecordAudit(ctx context.Context, e AuditEvent) error {
	_, _, err := s.c.Collection(CollAuditEvents).Add(ctx, e)
	return err
}
