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

type Firestore struct {
	c *firestore.Client
}

var _ Store = (*Firestore)(nil)

func NewFirestore(ctx context.Context, projectID, database string) (*Firestore, error) {
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
	return &Firestore{c: c}, nil
}

func (s *Firestore) Close() error { return s.c.Close() }

func notFound(err error) bool { return status.Code(err) == codes.NotFound }

func (s *Firestore) CreateShare(ctx context.Context, share Share, adminHash string) (string, error) {
	shareRef := s.c.Collection(CollShares).NewDoc()
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Create(shareRef, share); err != nil {
			return err
		}
		return tx.Create(s.c.Collection(CollAdminTokens).Doc(adminHash), AdminToken{
			ShareID:   shareRef.ID,
			CreatedAt: share.CreatedAt,
			ExpiresAt: share.ExpiresAt,
		})
	})
	if err != nil {
		return "", err
	}
	return shareRef.ID, nil
}

func (s *Firestore) FinalizeShare(ctx context.Context, shareID, downloadHash string, now, downloadExpires time.Time, maxUses int) error {
	shareRef := s.c.Collection(CollShares).Doc(shareID)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(shareRef)
		if err != nil {
			if notFound(err) {
				return ErrNotFound
			}
			return err
		}
		var sh Share
		if err := snap.DataTo(&sh); err != nil {
			return err
		}
		if sh.Status != StatusPending {
			return ErrAlreadyFinal
		}
		if err := tx.Update(shareRef, []firestore.Update{{Path: "status", Value: StatusReady}}); err != nil {
			return err
		}
		return tx.Create(s.c.Collection(CollDownloadTokens).Doc(downloadHash), DownloadToken{
			ShareID:       shareID,
			CreatedAt:     now,
			ExpiresAt:     downloadExpires,
			MaxUses:       maxUses,
			UsesRemaining: maxUses,
		})
	})
}

func (s *Firestore) getToken(ctx context.Context, get func(*firestore.DocumentRef) (*firestore.DocumentSnapshot, error), hash string) (DownloadToken, error) {
	snap, err := get(s.c.Collection(CollDownloadTokens).Doc(hash))
	if err != nil {
		if notFound(err) {
			return DownloadToken{}, ErrNotFound
		}
		return DownloadToken{}, err
	}
	var tok DownloadToken
	err = snap.DataTo(&tok)
	return tok, err
}

func (s *Firestore) PeekDownloadToken(ctx context.Context, hash string, now time.Time) (*Share, *DownloadToken, error) {
	tok, err := s.getToken(ctx, func(r *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) { return r.Get(ctx) }, hash)
	if err != nil {
		return nil, nil, err
	}
	if err := checkUsable(tok, now); err != nil {
		return nil, nil, err
	}
	sh, err := s.GetShare(ctx, tok.ShareID)
	if err != nil {
		return nil, nil, err
	}
	if sh.Status != StatusReady {
		return nil, nil, ErrNotReady
	}
	return sh, &tok, nil
}

func (s *Firestore) BurnDownloadToken(ctx context.Context, hash, ip string, now time.Time) (shareID string, share *Share, err error) {
	tokenRef := s.c.Collection(CollDownloadTokens).Doc(hash)
	err = s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		tok, err := s.getToken(ctx, tx.Get, hash)
		if err != nil {
			return err
		}
		if err := checkUsable(tok, now); err != nil {
			return err
		}
		// Firestore transactions require all reads before writes.
		shareSnap, err := tx.Get(s.c.Collection(CollShares).Doc(tok.ShareID))
		if err != nil {
			if notFound(err) {
				return ErrNotFound
			}
			return err
		}
		var sh Share
		if err := shareSnap.DataTo(&sh); err != nil {
			return err
		}
		if sh.Status != StatusReady {
			return ErrNotReady
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
		shareID = tok.ShareID
		share = &sh
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return shareID, share, nil
}

func (s *Firestore) LookupAdmin(ctx context.Context, hash string) (string, error) {
	snap, err := s.c.Collection(CollAdminTokens).Doc(hash).Get(ctx)
	if err != nil {
		if notFound(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	var at AdminToken
	if err := snap.DataTo(&at); err != nil {
		return "", err
	}
	// Firestore TTL deletion lags by up to a day; enforce expiry here too.
	if time.Now().After(at.ExpiresAt) {
		return "", ErrExpired
	}
	return at.ShareID, nil
}

func (s *Firestore) GetShare(ctx context.Context, shareID string) (*Share, error) {
	snap, err := s.c.Collection(CollShares).Doc(shareID).Get(ctx)
	if err != nil {
		if notFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var sh Share
	if err := snap.DataTo(&sh); err != nil {
		return nil, err
	}
	if time.Now().After(sh.ExpiresAt) {
		return nil, ErrExpired
	}
	return &sh, nil
}

func (s *Firestore) ListDownloadTokens(ctx context.Context, shareID string, now time.Time) ([]TokenStatus, error) {
	iter := s.c.Collection(CollDownloadTokens).
		Where("share_id", "==", shareID).
		OrderBy("created_at", firestore.Desc).
		Documents(ctx)
	defer iter.Stop()

	out := []TokenStatus{}
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
		out = append(out, toStatus(snap.Ref.ID, tok, now))
	}
	return out, nil
}

func (s *Firestore) IssueDownloadToken(ctx context.Context, shareID, hash string, now, expires time.Time, maxUses int) error {
	_, err := s.c.Collection(CollDownloadTokens).Doc(hash).Create(ctx, DownloadToken{
		ShareID:       shareID,
		CreatedAt:     now,
		ExpiresAt:     expires,
		MaxUses:       maxUses,
		UsesRemaining: maxUses,
	})
	return err
}

// RevokeOutstandingTokens flips revoked=true on every token still usable.
// Filter happens client-side so we don't need an inequality composite index.
func (s *Firestore) RevokeOutstandingTokens(ctx context.Context, shareID string, now time.Time) (int, error) {
	iter := s.c.Collection(CollDownloadTokens).
		Where("share_id", "==", shareID).
		Documents(ctx)
	defer iter.Stop()

	bw := s.c.BulkWriter(ctx)
	defer bw.End()
	count := 0
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return count, err
		}
		var tok DownloadToken
		if err := snap.DataTo(&tok); err != nil {
			return count, err
		}
		if classifyToken(tok, now) != "active" {
			continue
		}
		if _, err := bw.Update(snap.Ref, []firestore.Update{{Path: "revoked", Value: true}}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Firestore) DeleteShare(ctx context.Context, shareID, adminHash string) error {
	bw := s.c.BulkWriter(ctx)
	defer bw.End()
	if _, err := bw.Delete(s.c.Collection(CollShares).Doc(shareID)); err != nil {
		return err
	}
	if _, err := bw.Delete(s.c.Collection(CollAdminTokens).Doc(adminHash)); err != nil {
		return err
	}
	iter := s.c.Collection(CollDownloadTokens).Where("share_id", "==", shareID).Documents(ctx)
	defer iter.Stop()
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := bw.Delete(snap.Ref); err != nil {
			return err
		}
	}
}

func (s *Firestore) RecordAudit(ctx context.Context, e AuditEvent) error {
	_, _, err := s.c.Collection(CollAuditEvents).Add(ctx, e)
	return err
}
