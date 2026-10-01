package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                 string
	ProjectID            string
	BucketName           string
	FirestoreDatabase    string
	SignerServiceAccount string
	HMACPepper           []byte
	DownloadURLTTL       time.Duration
	ShareTTL             time.Duration

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
		ProjectID:            os.Getenv("GOOGLE_CLOUD_PROJECT"),
		BucketName:           os.Getenv("BUCKET_NAME"),
		FirestoreDatabase:    getenv("FIRESTORE_DATABASE", "(default)"),
		SignerServiceAccount: os.Getenv("SIGNER_SERVICE_ACCOUNT"),
		HMACPepper:           []byte(os.Getenv("HMAC_PEPPER")),
	}

	if c.ProjectID == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT not set (Cloud Run injects this automatically)")
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

	c.DownloadURLTTL = mustDuration("DOWNLOAD_URL_TTL_SECONDS", 60, time.Second)
	c.ShareTTL = mustDuration("SHARE_TTL_DAYS", 7, 24*time.Hour)

	c.DefaultTokenTTLHours = mustInt("DEFAULT_TOKEN_TTL_HOURS", 48)
	c.MinTokenTTLHours = mustInt("MIN_TOKEN_TTL_HOURS", 1)
	c.MaxTokenTTLHours = mustInt("MAX_TOKEN_TTL_HOURS", 168)
	c.DefaultMaxUses = mustInt("DEFAULT_MAX_USES", 1)
	c.MaxUsesPerToken = mustInt("MAX_USES_PER_TOKEN", 100)

	c.UploadRPM = mustInt("RATE_LIMIT_UPLOAD_RPM", 10)
	c.UploadBurst = mustInt("RATE_LIMIT_UPLOAD_BURST", 3)
	c.ReadRPM = mustInt("RATE_LIMIT_READ_RPM", 60)
	c.ReadBurst = mustInt("RATE_LIMIT_READ_BURST", 30)

	return c, nil
}

func mustInt(key string, def int) int {
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

func mustDuration(key string, def int, unit time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(def) * unit
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return time.Duration(def) * unit
	}
	return time.Duration(n) * unit
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
