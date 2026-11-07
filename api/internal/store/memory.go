package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Memory is an in-process Store for LOCAL_MODE and tests. A single mutex
// gives it the same atomicity the Firestore transactions provide.
type Memory struct {
	mu     sync.Mutex
	shares map[string]Share
	tokens map[string]DownloadToken
	admins map[string]AdminToken
	audit  []AuditEvent
	clock  func() time.Time
}

var _ Store = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		shares: map[string]Share{},
		tokens: map[string]DownloadToken{},
		admins: map[string]AdminToken{},
		clock:  time.Now,
	}
}

func (m *Memory) Close() error { return nil }

// Audit returns a copy of the recorded audit events (tests only).
func (m *Memory) Audit() []AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]AuditEvent(nil), m.audit...)
}

func newID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Memory) CreateShare(_ context.Context, share Share, adminHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.admins[adminHash]; ok {
		return "", ErrAlreadyFinal
	}
	id := newID()
	m.shares[id] = share
	m.admins[adminHash] = AdminToken{ShareID: id, CreatedAt: share.CreatedAt, ExpiresAt: share.ExpiresAt}
	return id, nil
}

func (m *Memory) FinalizeShare(_ context.Context, shareID, downloadHash string, now, downloadExpires time.Time, maxUses int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sh, ok := m.shares[shareID]
	if !ok {
		return ErrNotFound
	}
	if sh.Status != StatusPending {
		return ErrAlreadyFinal
	}
	sh.Status = StatusReady
	m.shares[shareID] = sh
	m.tokens[downloadHash] = DownloadToken{
		ShareID: shareID, CreatedAt: now, ExpiresAt: downloadExpires,
		MaxUses: maxUses, UsesRemaining: maxUses,
	}
	return nil
}

func (m *Memory) peekLocked(hash string, now time.Time) (*Share, DownloadToken, error) {
	tok, ok := m.tokens[hash]
	if !ok {
		return nil, tok, ErrNotFound
	}
	if err := checkUsable(tok, now); err != nil {
		return nil, tok, err
	}
	sh, ok := m.shares[tok.ShareID]
	if !ok || m.clock().After(sh.ExpiresAt) {
		return nil, tok, ErrNotFound
	}
	if sh.Status != StatusReady {
		return nil, tok, ErrNotReady
	}
	return &sh, tok, nil
}

func (m *Memory) PeekDownloadToken(_ context.Context, hash string, now time.Time) (*Share, *DownloadToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sh, tok, err := m.peekLocked(hash, now)
	if err != nil {
		return nil, nil, err
	}
	return sh, &tok, nil
}

func (m *Memory) BurnDownloadToken(_ context.Context, hash, ip string, now time.Time) (string, *Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sh, tok, err := m.peekLocked(hash, now)
	if err != nil {
		return "", nil, err
	}
	tok.UsesRemaining--
	t := now
	tok.LastUsedAt = &t
	tok.LastUsedFromIP = ip
	if tok.FirstUsedAt == nil {
		tok.FirstUsedAt = &t
	}
	m.tokens[hash] = tok
	return tok.ShareID, sh, nil
}

func (m *Memory) LookupAdmin(_ context.Context, hash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at, ok := m.admins[hash]
	if !ok {
		return "", ErrNotFound
	}
	if m.clock().After(at.ExpiresAt) {
		return "", ErrExpired
	}
	return at.ShareID, nil
}

func (m *Memory) GetShare(_ context.Context, shareID string) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sh, ok := m.shares[shareID]
	if !ok {
		return nil, ErrNotFound
	}
	if m.clock().After(sh.ExpiresAt) {
		return nil, ErrExpired
	}
	return &sh, nil
}

func (m *Memory) ListDownloadTokens(_ context.Context, shareID string, now time.Time) ([]TokenStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []TokenStatus{}
	for h, t := range m.tokens {
		if t.ShareID == shareID {
			out = append(out, toStatus(h, t, now))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ShortID > out[j].ShortID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *Memory) IssueDownloadToken(_ context.Context, shareID, hash string, now, expires time.Time, maxUses int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tokens[hash]; ok {
		return ErrAlreadyFinal
	}
	m.tokens[hash] = DownloadToken{
		ShareID: shareID, CreatedAt: now, ExpiresAt: expires,
		MaxUses: maxUses, UsesRemaining: maxUses,
	}
	return nil
}

func (m *Memory) RevokeOutstandingTokens(_ context.Context, shareID string, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for h, t := range m.tokens {
		if t.ShareID == shareID && classifyToken(t, now) == "active" {
			t.Revoked = true
			m.tokens[h] = t
			n++
		}
	}
	return n, nil
}

func (m *Memory) DeleteShare(_ context.Context, shareID, adminHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.shares, shareID)
	delete(m.admins, adminHash)
	for h, t := range m.tokens {
		if t.ShareID == shareID {
			delete(m.tokens, h)
		}
	}
	return nil
}

func (m *Memory) RecordAudit(_ context.Context, e AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = append(m.audit, e)
	return nil
}
