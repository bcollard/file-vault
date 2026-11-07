package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The suite runs against Memory always, and against Firestore when
// FIRESTORE_EMULATOR_HOST is set (`gcloud emulators firestore start`).
func backends(t *testing.T) map[string]Store {
	out := map[string]Store{"memory": NewMemory()}
	if os.Getenv("FIRESTORE_EMULATOR_HOST") != "" {
		fs, err := NewFirestore(context.Background(), "file-vault-test", "(default)")
		if err != nil {
			t.Fatalf("firestore emulator: %v", err)
		}
		t.Cleanup(func() { _ = fs.Close() })
		out["firestore"] = fs
	}
	return out
}

func randHash() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// readyShare creates a finalized share with one download token.
func readyShare(t *testing.T, s Store, now time.Time, maxUses int) (shareID, adminHash, dlHash string) {
	t.Helper()
	ctx := context.Background()
	adminHash, dlHash = randHash(), randHash()
	shareID, err := s.CreateShare(ctx, Share{
		ObjectPath: "obj", Meta: "bWV0YQ", Size: 100, Status: StatusPending,
		CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
	}, adminHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PeekDownloadToken(ctx, dlHash, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token before finalize: want ErrNotFound, got %v", err)
	}
	if err := s.FinalizeShare(ctx, shareID, dlHash, now, now.Add(time.Hour), maxUses); err != nil {
		t.Fatal(err)
	}
	return shareID, adminHash, dlHash
}

func TestStore(t *testing.T) {
	for name, s := range backends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Millisecond)

			t.Run("finalize twice", func(t *testing.T) {
				shareID, _, _ := readyShare(t, s, now, 1)
				err := s.FinalizeShare(ctx, shareID, randHash(), now, now.Add(time.Hour), 1)
				if !errors.Is(err, ErrAlreadyFinal) {
					t.Fatalf("want ErrAlreadyFinal, got %v", err)
				}
			})

			t.Run("burn until exhausted", func(t *testing.T) {
				_, _, dl := readyShare(t, s, now, 2)
				for i := range 2 {
					if _, _, err := s.BurnDownloadToken(ctx, dl, "192.0.2.1", now); err != nil {
						t.Fatalf("burn %d: %v", i, err)
					}
				}
				if _, _, err := s.BurnDownloadToken(ctx, dl, "192.0.2.1", now); !errors.Is(err, ErrExhausted) {
					t.Fatalf("third burn: want ErrExhausted, got %v", err)
				}
				if _, _, err := s.PeekDownloadToken(ctx, dl, now); !errors.Is(err, ErrExhausted) {
					t.Fatalf("peek: want ErrExhausted, got %v", err)
				}
			})

			t.Run("peek does not consume", func(t *testing.T) {
				_, _, dl := readyShare(t, s, now, 1)
				for range 3 {
					_, tok, err := s.PeekDownloadToken(ctx, dl, now)
					if err != nil || tok.UsesRemaining != 1 {
						t.Fatalf("peek: %v, remaining=%v", err, tok)
					}
				}
			})

			t.Run("expired", func(t *testing.T) {
				_, _, dl := readyShare(t, s, now, 1)
				later := now.Add(2 * time.Hour)
				if _, _, err := s.BurnDownloadToken(ctx, dl, "", later); !errors.Is(err, ErrExpired) {
					t.Fatalf("want ErrExpired, got %v", err)
				}
			})

			t.Run("revoke", func(t *testing.T) {
				shareID, _, dl := readyShare(t, s, now, 3)
				extra := randHash()
				if err := s.IssueDownloadToken(ctx, shareID, extra, now, now.Add(time.Hour), 1); err != nil {
					t.Fatal(err)
				}
				n, err := s.RevokeOutstandingTokens(ctx, shareID, now)
				if err != nil || n != 2 {
					t.Fatalf("revoke: n=%d err=%v", n, err)
				}
				if _, _, err := s.BurnDownloadToken(ctx, dl, "", now); !errors.Is(err, ErrRevoked) {
					t.Fatalf("want ErrRevoked, got %v", err)
				}
				list, err := s.ListDownloadTokens(ctx, shareID, now)
				if err != nil || len(list) != 2 {
					t.Fatalf("list: %v %v", list, err)
				}
				for _, ts := range list {
					if ts.Status != "revoked" {
						t.Errorf("status = %s, want revoked", ts.Status)
					}
				}
			})

			t.Run("concurrent burns of a one-time token", func(t *testing.T) {
				_, _, dl := readyShare(t, s, now, 1)
				var ok atomic.Int32
				var wg sync.WaitGroup
				for range 20 {
					wg.Go(func() {
						if _, _, err := s.BurnDownloadToken(ctx, dl, "", now); err == nil {
							ok.Add(1)
						}
					})
				}
				wg.Wait()
				if ok.Load() != 1 {
					t.Fatalf("%d burns succeeded, want exactly 1", ok.Load())
				}
			})

			t.Run("delete", func(t *testing.T) {
				shareID, admin, dl := readyShare(t, s, now, 1)
				if err := s.DeleteShare(ctx, shareID, admin); err != nil {
					t.Fatal(err)
				}
				if _, err := s.LookupAdmin(ctx, admin); !errors.Is(err, ErrNotFound) {
					t.Fatalf("admin after delete: %v", err)
				}
				if _, _, err := s.PeekDownloadToken(ctx, dl, now); !errors.Is(err, ErrNotFound) {
					t.Fatalf("token after delete: %v", err)
				}
			})
		})
	}
}
