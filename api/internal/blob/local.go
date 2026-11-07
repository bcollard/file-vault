package blob

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Local stores objects in a directory and serves the same upload protocol the
// browser uses against GCS: PUT chunks with Content-Range, 308 until the last
// byte, then 200. LOCAL_MODE and tests only.
type Local struct {
	dir    string
	secret []byte
	clock  func() time.Time

	mu      sync.Mutex
	uploads map[string]*localUpload
}

type localUpload struct {
	object  string
	size    int64
	written int64
}

var _ Blobs = (*Local)(nil)

// Object names are random base64url tokens; reject anything else so a path
// can't escape the directory.
var objectRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return &Local{dir: dir, secret: secret, clock: time.Now, uploads: map[string]*localUpload{}}, nil
}

func (l *Local) Close() error { return nil }

func (l *Local) path(object string) (string, error) {
	if !objectRe.MatchString(object) {
		return "", fmt.Errorf("invalid object name")
	}
	return filepath.Join(l.dir, object), nil
}

func (l *Local) StartUpload(_ context.Context, object string, size int64) (string, error) {
	p, err := l.path(object)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(p+".part", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	_ = f.Close()
	idb := make([]byte, 16)
	_, _ = rand.Read(idb)
	id := hex.EncodeToString(idb)
	l.mu.Lock()
	l.uploads[id] = &localUpload{object: object, size: size}
	l.mu.Unlock()
	return "/_local/upload/" + id, nil
}

func (l *Local) Size(_ context.Context, object string) (int64, error) {
	p, err := l.path(object)
	if err != nil {
		return 0, ErrNotFound
	}
	st, err := os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func (l *Local) sign(object string, exp int64) string {
	h := hmac.New(sha256.New, l.secret)
	fmt.Fprintf(h, "%s|%d", object, exp)
	return hex.EncodeToString(h.Sum(nil))
}

func (l *Local) DownloadURL(_ context.Context, object string, ttl time.Duration) (string, error) {
	exp := l.clock().Add(ttl).Unix()
	return fmt.Sprintf("/_local/blob/%s?exp=%d&sig=%s", object, exp, l.sign(object, exp)), nil
}

func (l *Local) Delete(_ context.Context, object string) error {
	p, err := l.path(object)
	if err != nil {
		return nil
	}
	for _, f := range []string{p, p + ".part"} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Handler serves /_local/upload/{id} and /_local/blob/{object}.
func (l *Local) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /_local/upload/{id}", l.handlePut)
	mux.HandleFunc("GET /_local/blob/{object}", l.handleGet)
	return mux
}

var rangeRe = regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+)$`)

func (l *Local) handlePut(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	up, ok := l.uploads[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	m := rangeRe.FindStringSubmatch(r.Header.Get("Content-Range"))
	if m == nil {
		http.Error(w, "bad Content-Range", http.StatusBadRequest)
		return
	}
	start, _ := strconv.ParseInt(m[1], 10, 64)
	end, _ := strconv.ParseInt(m[2], 10, 64)
	total, _ := strconv.ParseInt(m[3], 10, 64)
	if total != up.size || end >= total || start > end || start > up.written {
		http.Error(w, "range out of bounds", http.StatusBadRequest)
		return
	}
	p, _ := l.path(up.object)
	f, err := os.OpenFile(p+".part", os.O_WRONLY, 0o600)
	if err != nil {
		http.Error(w, "upload gone", http.StatusNotFound)
		return
	}
	defer f.Close()
	n, err := io.Copy(io.NewOffsetWriter(f, start), io.LimitReader(r.Body, end-start+2))
	if err != nil || n != end-start+1 {
		http.Error(w, "short body", http.StatusBadRequest)
		return
	}
	if end+1 > up.written {
		up.written = end + 1
	}
	if up.written < up.size {
		w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", up.written-1))
		w.WriteHeader(http.StatusPermanentRedirect) // 308, as GCS does
		return
	}
	_ = f.Close()
	if err := os.Rename(p+".part", p); err != nil {
		http.Error(w, "finalize", http.StatusInternalServerError)
		return
	}
	delete(l.uploads, r.PathValue("id"))
	w.WriteHeader(http.StatusOK)
}

func (l *Local) handleGet(w http.ResponseWriter, r *http.Request) {
	object := r.PathValue("object")
	exp, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || l.clock().Unix() > exp ||
		!hmac.Equal([]byte(l.sign(object, exp)), []byte(r.URL.Query().Get("sig"))) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	p, err := l.path(object)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+ObjectName+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, p)
}
