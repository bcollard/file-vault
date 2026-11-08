---
title: Configuration
description: Environment variables of the service and the Terraform inputs that set them.
---

Terraform sets the service's environment. Change behaviour through `terraform.tfvars` (or the `TFVARS` secret in CI) where a Terraform input exists.

## Terraform inputs

Set in `terraform/envs/<env>/terraform.tfvars`:

| Input | Default | Notes |
|---|---|---|
| `project_id` | required | GCP project |
| `bucket_name` | required | Globally unique bucket name |
| `domain` | required | Hostname users load the UI from |
| `alert_email` | required | Monitoring notifications. Sensitive |
| `region` | `europe-west9` | Cloud Run, bucket, KMS and Firestore |
| `share_ttl_days` | `7` | Share lifetime; also the bucket lifecycle age |
| `max_upload_mb` | `1024` | Largest ciphertext accepted |
| `min_instances` / `max_instances` | `0` / `10` | Cloud Run scaling |
| `force_destroy_bucket` | `false` (prod) | Allow `terraform destroy` on a non-empty bucket |

`image` is passed by the deploy workflow on every run.

## Service environment variables

| Variable | Default | Notes |
|---|---|---|
| `GOOGLE_CLOUD_PROJECT` | — | Set by Terraform. Cloud Run doesn't provide it |
| `PUBLIC_ORIGIN` | — | `https://<domain>`. Upload sessions are bound to it |
| `BUCKET_NAME` | — | Set by Terraform |
| `FIRESTORE_DATABASE` | `(default)` | Terraform sets `file-vault-<env>` |
| `SIGNER_SERVICE_ACCOUNT` | — | Signs download URLs through IAM SignBlob |
| `HMAC_PEPPER` | — | From Secret Manager |
| `MAX_UPLOAD_MB` | `1024` | Downloads decrypt in browser memory; keep it moderate |
| `SHARE_TTL_DAYS` | `7` | |
| `DEFAULT_TOKEN_TTL_HOURS` | `48` | |
| `MIN_TOKEN_TTL_HOURS` / `MAX_TOKEN_TTL_HOURS` | `1` / `168` | |
| `DEFAULT_MAX_USES` / `MAX_USES_PER_TOKEN` | `1` / `100` | The UI slider stops at 20 |
| `DOWNLOAD_URL_TTL_SECONDS` | `60` | Signed URL lifetime |
| `AUDIT_RETENTION_DAYS` | `30` | TTL on audit events |
| `TRUSTED_PROXY_HOPS` | `2` | `X-Forwarded-For` entries added by trusted proxies |
| `RATE_LIMIT_UPLOAD_RPM` / `RATE_LIMIT_UPLOAD_BURST` | `10` / `3` | Per IP, per instance |
| `RATE_LIMIT_READ_RPM` / `RATE_LIMIT_READ_BURST` | `60` / `30` | Per IP, per instance |
| `LOCAL_MODE` | unset | `1` = in-memory store and local disk. Never in production |
