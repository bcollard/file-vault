package config

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string

	// LocalMode swaps Firestore + GCS for an in-memory store and a local blob
	// directory. No GCP credentials needed. Never enable in production.
	LocalMode bool
	LocalDir  string

	ProjectID            string
	BucketName           string
	FirestoreDatabase    string
	SignerServiceAccount string
	HMACPepper           []byte

	// PublicOrigin is the scheme+host users load the UI from
	// (e.g. https://file-vault.example.com). GCS resumable sessions are bound
	// to it so the browser can PUT chunks cross-origin.
	PublicOrigin string

	DownloadURLTTL  time.Duration
	ShareTTL        time.Duration
	AuditRetention  time.Duration
	MaxUploadBytes  int64
	MaxMetaBytes    int
	TrustedProxyHop int

	// Per-token settings — the upload form may pick within these bounds.
	DefaultTokenTTLHours int
	MinTokenTTLHours     int
	MaxTokenTTLHours     int
	DefaultMaxUses       int
	MaxUsesPerToken      int

	// Per-IP rate limits (requests per minute) + burst capacity.
	UploadRPM   int
	UploadBurst int
	ReadRPM     int
	ReadBurst   int
}

func Load() (*Config, error) {
	c := &Config{
		Port:                 getenv("PORT", "8080"),
		LocalMode:            os.Getenv("LOCAL_MODE") == "1",
		LocalDir:             os.Getenv("LOCAL_DIR"),
		ProjectID:            os.Getenv("GOOGLE_CLOUD_PROJECT"),
		BucketName:           os.Getenv("BUCKET_NAME"),
		FirestoreDatabase:    getenv("FIRESTORE_DATABASE", "(default)"),
		SignerServiceAccount: os.Getenv("SIGNER_SERVICE_ACCOUNT"),
		HMACPepper:           []byte(os.Getenv("HMAC_PEPPER")),
		PublicOrigin:         strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/"),
	}

	c.DownloadURLTTL = duration("DOWNLOAD_URL_TTL_SECONDS", 60, time.Second)
	c.ShareTTL = duration("SHARE_TTL_DAYS", 7, 24*time.Hour)
	c.AuditRetention = duration("AUDIT_RETENTION_DAYS", 30, 24*time.Hour)
	c.MaxUploadBytes = int64(intEnv("MAX_UPLOAD_MB", 1024)) << 20
	c.MaxMetaBytes = intEnv("MAX_META_BYTES", 4096)
	// Google's external Application Load Balancer appends "<client-ip>, <lb-ip>"
	// to X-Forwarded-For. Anything to the left of that is client-supplied.
	c.TrustedProxyHop = intEnv("TRUSTED_PROXY_HOPS", 2)

	c.DefaultTokenTTLHours = intEnv("DEFAULT_TOKEN_TTL_HOURS", 48)
	c.MinTokenTTLHours = intEnv("MIN_TOKEN_TTL_HOURS", 1)
	c.MaxTokenTTLHours = intEnv("MAX_TOKEN_TTL_HOURS", 168)
	c.DefaultMaxUses = intEnv("DEFAULT_MAX_USES", 1)
	c.MaxUsesPerToken = intEnv("MAX_USES_PER_TOKEN", 100)

	c.UploadRPM = intEnv("RATE_LIMIT_UPLOAD_RPM", 10)
	c.UploadBurst = intEnv("RATE_LIMIT_UPLOAD_BURST", 3)
	c.ReadRPM = intEnv("RATE_LIMIT_READ_RPM", 60)
	c.ReadBurst = intEnv("RATE_LIMIT_READ_BURST", 30)

	if c.LocalMode {
		if c.PublicOrigin == "" {
			c.PublicOrigin = "http://localhost:" + c.Port
		}
		if len(c.HMACPepper) == 0 {
			c.HMACPepper = make([]byte, 32)
			_, _ = rand.Read(c.HMACPepper)
		}
		// Local requests come straight from the browser or the Vite proxy.
		if os.Getenv("TRUSTED_PROXY_HOPS") == "" {
			c.TrustedProxyHop = 0
		}
		return c, nil
	}

	if c.ProjectID == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT required")
	}
	if c.BucketName == "" {
		return nil, fmt.Errorf("BUCKET_NAME required")
	}
	if c.SignerServiceAccount == "" {
		return nil, fmt.Errorf("SIGNER_SERVICE_ACCOUNT required")
	}
	if len(c.HMACPepper) == 0 {
		return nil, fmt.Errorf("HMAC_PEPPER required")
	}
	if u, err := url.Parse(c.PublicOrigin); err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
		return nil, fmt.Errorf("PUBLIC_ORIGIN must be scheme://host, got %q", c.PublicOrigin)
	}
	return c, nil
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func duration(key string, def int, unit time.Duration) time.Duration {
	return time.Duration(intEnv(key, def)) * unit
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
