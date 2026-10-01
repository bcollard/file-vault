package signer

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iamcredentials/v1"
)

// Signer mints V4 signed URLs scoped to a single bucket, using IAM SignBlob so
// the runtime service account never needs a JSON key on disk.
type Signer struct {
	bucket  string
	saEmail string
	creds   *iamcredentials.Service
}

func New(ctx context.Context, bucket, signerSA string) (*Signer, error) {
	creds, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iamcredentials.NewService: %w", err)
	}
	return &Signer{bucket: bucket, saEmail: signerSA, creds: creds}, nil
}

// SignedDownloadURL returns a V4 GET URL good for ttl. The filename is forced
// into a Content-Disposition: attachment response header so browsers download
// rather than render the file — critical because uploads carry the user's
// Content-Type to GCS, so an uploaded HTML page would otherwise execute as a
// page on storage.googleapis.com.
func (s *Signer) SignedDownloadURL(object, filename string, ttl time.Duration) (string, error) {
	return storage.SignedURL(s.bucket, object, &storage.SignedURLOptions{
		GoogleAccessID: s.saEmail,
		Method:         "GET",
		Expires:        time.Now().Add(ttl),
		Scheme:         storage.SigningSchemeV4,
		QueryParameters: url.Values{
			"response-content-disposition": []string{contentDisposition(filename)},
		},
		SignBytes: s.signBytes,
	})
}

// contentDisposition builds an attachment header with both a US-ASCII filename
// (for legacy clients) and an RFC 5987 filename* (for Unicode).
func contentDisposition(name string) string {
	ascii := asciiFilename(name)
	encoded := url.PathEscape(name)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, encoded)
}

func asciiFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('_')
		case r < 0x20 || r > 0x7e:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Signer) signBytes(b []byte) ([]byte, error) {
	resp, err := s.creds.Projects.ServiceAccounts.SignBlob(
		"projects/-/serviceAccounts/"+s.saEmail,
		&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(b)},
	).Do()
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.SignedBlob)
}
