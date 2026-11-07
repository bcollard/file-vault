package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bcollard/file-vault/api/internal/blob"
	"github.com/bcollard/file-vault/api/internal/config"
	"github.com/bcollard/file-vault/api/internal/store"
	"github.com/bcollard/file-vault/api/internal/webui"
)

type harness struct {
	t    *testing.T
	srv  *httptest.Server
	st   *store.Memory
	logs *syncBuffer
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := &config.Config{
		LocalMode: true, HMACPepper: []byte("pepper"),
		DownloadURLTTL: time.Minute, ShareTTL: 7 * 24 * time.Hour, AuditRetention: 30 * 24 * time.Hour,
		MaxUploadBytes: 1 << 20, MaxMetaBytes: 4096,
		DefaultTokenTTLHours: 48, MinTokenTTLHours: 1, MaxTokenTTLHours: 168,
		DefaultMaxUses: 1, MaxUsesPerToken: 100,
		UploadRPM: 6000, UploadBurst: 1000, ReadRPM: 6000, ReadBurst: 1000,
	}
	st := store.NewMemory()
	lb, err := blob.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ui := webui.New(fstest.MapFS{
		"index.html":                {Data: []byte("<!doctype html><title>spa</title>")},
		"d.html":                    {Data: []byte("<!doctype html><title>spa</title>")},
		"a.html":                    {Data: []byte("<!doctype html><title>spa</title>")},
		"200.html":                  {Data: []byte("<!doctype html><title>spa-fallback</title>")},
		"_app/immutable/entry/a.js": {Data: []byte("console.log(1)")},
	})
	srv := httptest.NewServer(New(cfg, st, lb, ui, lb.Handler()).Router())
	t.Cleanup(srv.Close)
	return &harness{t: t, srv: srv, st: st, logs: logs}
}

func (h *harness) post(path string, body any) (int, map[string]any) {
	h.t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(h.srv.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// put uploads data to a session URL in chunks, like the browser does.
func (h *harness) put(uploadURL string, data []byte, chunk int) {
	h.t.Helper()
	for off := 0; off < len(data); off += chunk {
		end := min(off+chunk, len(data))
		req, _ := http.NewRequest(http.MethodPut, h.srv.URL+uploadURL, bytes.NewReader(data[off:end]))
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", off, end-1, len(data)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			h.t.Fatal(err)
		}
		resp.Body.Close()
		want := http.StatusPermanentRedirect
		if end == len(data) {
			want = http.StatusOK
		}
		if resp.StatusCode != want {
			h.t.Fatalf("PUT %d-%d: status %d, want %d", off, end-1, resp.StatusCode, want)
		}
	}
}

// share runs create → upload → finalize and returns (admin, download) tokens.
func (h *harness) share(data []byte, maxUses int) (string, string) {
	h.t.Helper()
	code, created := h.post("/api/shares", map[string]any{
		"size": len(data), "meta": "ZW5jcnlwdGVkLW1ldGE", "ttl_hours": 24, "max_uses": maxUses,
	})
	if code != http.StatusCreated {
		h.t.Fatalf("create: %d %v", code, created)
	}
	admin := created["admin_token"].(string)
	h.put(created["upload_url"].(string), data, 7)
	code, fin := h.post("/api/shares/finalize", map[string]any{"admin_token": admin})
	if code != http.StatusOK {
		h.t.Fatalf("finalize: %d %v", code, fin)
	}
	return admin, fin["download_token"].(string)
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func TestFullFlow(t *testing.T) {
	h := newHarness(t)
	data := randBytes(50)
	admin, dl := h.share(data, 2)

	code, peek := h.post("/api/download/peek", map[string]any{"token": dl})
	if code != http.StatusOK || peek["uses_remaining"].(float64) != 2 || peek["meta"] != "ZW5jcnlwdGVkLW1ldGE" {
		t.Fatalf("peek: %d %v", code, peek)
	}

	for i := range 2 {
		code, burn := h.post("/api/download/burn", map[string]any{"token": dl})
		if code != http.StatusOK {
			t.Fatalf("burn %d: %d %v", i, code, burn)
		}
		resp, err := http.Get(h.srv.URL + burn["url"].(string))
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !bytes.Equal(got, data) {
			t.Fatalf("downloaded bytes differ")
		}
		if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
			t.Fatalf("Content-Disposition = %q", cd)
		}
	}
	if code, _ := h.post("/api/download/burn", map[string]any{"token": dl}); code != http.StatusNotFound {
		t.Fatalf("third burn: %d, want 404", code)
	}

	code, info := h.post("/api/admin", map[string]any{"admin_token": admin})
	if code != http.StatusOK || info["status"] != "ready" || len(info["tokens"].([]any)) != 1 {
		t.Fatalf("admin get: %d %v", code, info)
	}

	code, issued := h.post("/api/admin/issue", map[string]any{"admin_token": admin, "ttl_hours": 9999, "max_uses": 9999})
	if code != http.StatusCreated || issued["max_uses"].(float64) != 100 {
		t.Fatalf("issue (clamped): %d %v", code, issued)
	}
	dl2 := issued["download_token"].(string)

	code, rev := h.post("/api/admin/revoke", map[string]any{"admin_token": admin})
	if code != http.StatusOK || rev["revoked"].(float64) != 1 {
		t.Fatalf("revoke: %d %v", code, rev)
	}
	if code, _ := h.post("/api/download/peek", map[string]any{"token": dl2}); code != http.StatusNotFound {
		t.Fatalf("peek revoked: %d", code)
	}

	if code, _ := h.post("/api/admin/delete", map[string]any{"admin_token": admin}); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := h.post("/api/admin", map[string]any{"admin_token": admin}); code != http.StatusNotFound {
		t.Fatalf("admin after delete: %d", code)
	}

	// Bug 1 regression: no capability token may ever reach a log line.
	logs := h.logs.String()
	for _, tok := range []string{admin, dl, dl2} {
		if strings.Contains(logs, tok) {
			t.Fatalf("token %s… found in logs", tok[:6])
		}
	}
	for _, e := range h.st.Audit() {
		if e.ExpiresAt.Sub(e.At) != 30*24*time.Hour {
			t.Fatalf("audit event %s has no retention TTL", e.Event)
		}
	}
}

func TestFinalizeRules(t *testing.T) {
	h := newHarness(t)
	_, created := h.post("/api/shares", map[string]any{"size": 40, "meta": "bWV0YQ"})
	admin := created["admin_token"].(string)

	if code, _ := h.post("/api/shares/finalize", map[string]any{"admin_token": admin}); code != http.StatusConflict {
		t.Fatalf("finalize before upload: %d, want 409", code)
	}
	if code, _ := h.post("/api/admin/issue", map[string]any{"admin_token": admin}); code != http.StatusConflict {
		t.Fatalf("issue on pending share: %d, want 409", code)
	}
	h.put(created["upload_url"].(string), randBytes(40), 40)
	if code, _ := h.post("/api/shares/finalize", map[string]any{"admin_token": admin}); code != http.StatusOK {
		t.Fatalf("finalize: %d", code)
	}
	if code, _ := h.post("/api/shares/finalize", map[string]any{"admin_token": admin}); code != http.StatusConflict {
		t.Fatalf("second finalize: %d, want 409", code)
	}
}

func TestCreateValidation(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		name string
		body any
		want int
	}{
		{"too big", map[string]any{"size": 2 << 20, "meta": "bWV0YQ"}, http.StatusRequestEntityTooLarge},
		{"too small", map[string]any{"size": 5, "meta": "bWV0YQ"}, http.StatusRequestEntityTooLarge},
		{"no meta", map[string]any{"size": 100}, http.StatusBadRequest},
		{"meta not base64url", map[string]any{"size": 100, "meta": "a b"}, http.StatusBadRequest},
		{"unknown field", map[string]any{"size": 100, "meta": "bWV0YQ", "filename": "x"}, http.StatusBadRequest},
	}
	for _, c := range cases {
		if code, _ := h.post("/api/shares", c.body); code != c.want {
			t.Errorf("%s: %d, want %d", c.name, code, c.want)
		}
	}
	if code, _ := h.post("/api/download/peek", map[string]any{"token": "short"}); code != http.StatusNotFound {
		t.Errorf("malformed token: %d, want 404", code)
	}
}

func TestConcurrentBurn(t *testing.T) {
	h := newHarness(t)
	_, dl := h.share(randBytes(30), 1)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if code, _ := h.post("/api/download/burn", map[string]any{"token": dl}); code == http.StatusOK {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("%d concurrent burns succeeded, want 1", ok.Load())
	}
}

func TestStaticAndHeaders(t *testing.T) {
	h := newHarness(t)
	for p, want := range map[string]int{"/": 200, "/d": 200, "/a": 200, "/a.html": 404, "/nope": 404, "/../etc/passwd": 404} {
		resp, err := http.Get(h.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want || !strings.Contains(string(body), "spa") {
			t.Errorf("%s: %d %q, want %d", p, resp.StatusCode, body, want)
		}
		if resp.Header.Get("X-Frame-Options") != "DENY" || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Errorf("%s: missing security headers", p)
		}
	}
	resp, _ := http.Get(h.srv.URL + "/_app/immutable/entry/a.js")
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("immutable asset Cache-Control = %q", resp.Header.Get("Cache-Control"))
	}
	resp, _ = http.Get(h.srv.URL + "/api/nope")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown api route: %d", resp.StatusCode)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		xff, remote string
		hops        int
		want        string
	}{
		{"", "10.0.0.1:1234", 2, "10.0.0.1"},
		{"203.0.113.7, 34.1.2.3", "10.0.0.1:1", 2, "203.0.113.7"},
		// A forged leftmost entry must not win.
		{"1.1.1.1, 203.0.113.7, 34.1.2.3", "10.0.0.1:1", 2, "203.0.113.7"},
		{"34.1.2.3", "10.0.0.1:1", 2, "10.0.0.1"},
		{"1.1.1.1", "10.0.0.1:1", 0, "10.0.0.1"},
		{"garbage, 34.1.2.3", "10.0.0.1:1", 2, "10.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := clientIP(r, c.hops); got != c.want {
			t.Errorf("xff=%q hops=%d: got %s, want %s", c.xff, c.hops, got, c.want)
		}
	}
}
