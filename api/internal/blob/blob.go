// Package blob stores the encrypted share objects. The browser uploads
// ciphertext directly to the backend through a resumable session URL the
// server opens, and downloads it through a short-lived signed URL. The Go
// service never proxies file bytes.
package blob

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("object not found")

// ObjectName is the fixed download filename. The real name is inside the
// encrypted metadata; the server never learns it.
const ObjectName = "file-vault.bin"

type Blobs interface {
	// StartUpload opens a resumable upload session for exactly size bytes
	// and returns the session URL the browser PUTs chunks to.
	StartUpload(ctx context.Context, object string, size int64) (string, error)
	// Size returns the stored object's length, or ErrNotFound.
	Size(ctx context.Context, object string) (int64, error)
	// DownloadURL returns a URL valid for ttl that serves the object as an
	// attachment. Content is ciphertext, so the type is always octet-stream.
	DownloadURL(ctx context.Context, object string, ttl time.Duration) (string, error)
	// Delete removes the object. Deleting a missing object is not an error.
	Delete(ctx context.Context, object string) error
	Close() error
}
