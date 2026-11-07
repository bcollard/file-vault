package blob

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iamcredentials/v1"
)

type GCS struct {
	bucket  string
	origin  string
	saEmail string
	client  *storage.Client
	authed  *http.Client
	iam     *iamcredentials.Service
}

var _ Blobs = (*GCS)(nil)

// NewGCS builds the GCS backend. signerSA signs V4 URLs through IAM SignBlob,
// so the runtime never needs a JSON key. origin is bound to each resumable
// session so GCS answers the browser's cross-origin chunk PUTs.
func NewGCS(ctx context.Context, bucket, signerSA, origin string) (*GCS, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	authed, err := google.DefaultClient(ctx, storage.ScopeReadWrite)
	if err != nil {
		return nil, fmt.Errorf("authed http client: %w", err)
	}
	iam, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iamcredentials: %w", err)
	}
	return &GCS{bucket: bucket, origin: origin, saEmail: signerSA, client: client, authed: authed, iam: iam}, nil
}

func (g *GCS) Close() error { return g.client.Close() }

// StartUpload initiates a JSON-API resumable upload. X-Upload-Content-Length
// pins the object size, ifGenerationMatch=0 refuses to overwrite, and the
// Origin header makes GCS return CORS headers on the session URL.
func (g *GCS) StartUpload(ctx context.Context, object string, size int64) (string, error) {
	q := url.Values{
		"uploadType":        {"resumable"},
		"name":              {object},
		"ifGenerationMatch": {"0"},
	}
	endpoint := "https://storage.googleapis.com/upload/storage/v1/b/" + url.PathEscape(g.bucket) + "/o?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(`{"contentType":"application/octet-stream"}`))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Type", "application/octet-stream")
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	req.Header.Set("Origin", g.origin)

	resp, err := g.authed.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("resumable init: %s: %s", resp.Status, b)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", errors.New("resumable init: no Location header")
	}
	return loc, nil
}

func (g *GCS) Size(ctx context.Context, object string) (int64, error) {
	attrs, err := g.client.Bucket(g.bucket).Object(object).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return attrs.Size, nil
}

// DownloadURL forces Content-Disposition: attachment and an octet-stream
// type, so the bucket never serves anything a browser would render.
func (g *GCS) DownloadURL(_ context.Context, object string, ttl time.Duration) (string, error) {
	return storage.SignedURL(g.bucket, object, &storage.SignedURLOptions{
		GoogleAccessID: g.saEmail,
		Method:         http.MethodGet,
		Expires:        time.Now().Add(ttl),
		Scheme:         storage.SigningSchemeV4,
		QueryParameters: url.Values{
			"response-content-disposition": {`attachment; filename="` + ObjectName + `"`},
			"response-content-type":        {"application/octet-stream"},
		},
		SignBytes: g.signBytes,
	})
}

func (g *GCS) Delete(ctx context.Context, object string) error {
	err := g.client.Bucket(g.bucket).Object(object).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

func (g *GCS) signBytes(b []byte) ([]byte, error) {
	resp, err := g.iam.Projects.ServiceAccounts.SignBlob(
		"projects/-/serviceAccounts/"+g.saEmail,
		&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(b)},
	).Do()
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.SignedBlob)
}
