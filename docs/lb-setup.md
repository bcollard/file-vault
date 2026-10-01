# Wiring file-vault into your existing Cloud Load Balancer

Terraform produces two serverless NEGs but never touches your LB. You connect them as backends on the LB you already manage.

After `terraform apply`, the relevant outputs are:

```
$ terraform -chdir=terraform/envs/dev output
api_neg_id       = "projects/<project>/regions/europe-west9/networkEndpointGroups/file-vault-api-dev"
frontend_neg_id  = "projects/<project>/regions/europe-west9/networkEndpointGroups/file-vault-frontend-dev"
```

## Layout you're aiming for

```
  file-vault.example.com
            |
       [ your LB ]
            |
   URL map / path matcher
       |              |
   /d/*           everything else
       |              |
   API NEG       Frontend NEG
```

## Step 1 — Create the two backend services

One backend service per NEG. Both are unauthenticated at the LB layer (capability tokens are the auth model).

```bash
PROJECT=your-gcp-project
REGION=europe-west9
ENV=dev

# API backend
gcloud compute backend-services create file-vault-api-$ENV \
  --project=$PROJECT \
  --global \
  --load-balancing-scheme=EXTERNAL_MANAGED \
  --protocol=HTTPS

gcloud compute backend-services add-backend file-vault-api-$ENV \
  --project=$PROJECT \
  --global \
  --network-endpoint-group=file-vault-api-$ENV \
  --network-endpoint-group-region=$REGION

# Frontend backend
gcloud compute backend-services create file-vault-frontend-$ENV \
  --project=$PROJECT \
  --global \
  --load-balancing-scheme=EXTERNAL_MANAGED \
  --protocol=HTTPS

gcloud compute backend-services add-backend file-vault-frontend-$ENV \
  --project=$PROJECT \
  --global \
  --network-endpoint-group=file-vault-frontend-$ENV \
  --network-endpoint-group-region=$REGION
```

## Step 2 — Disable CDN caching on dynamic paths

CDN must not cache `/d/*` (one-shot redirect) or `/a/*` (admin actions). Two options:

**A — Disable CDN on the API backend service entirely** (simplest, since the API only serves dynamic paths):

```bash
gcloud compute backend-services update file-vault-api-$ENV \
  --project=$PROJECT --global --no-enable-cdn
```

**B — Keep CDN on but rely on `Cache-Control` headers from the app.** The Go API sets `Cache-Control: no-store, private` on `/d/*` responses. Make sure your CDN policy uses `USE_ORIGIN_HEADERS` rather than forcing a TTL.

On the frontend backend service:

```bash
gcloud compute backend-services update file-vault-frontend-$ENV \
  --project=$PROJECT --global \
  --enable-cdn \
  --cache-mode=USE_ORIGIN_HEADERS
```

The frontend serves `/a/*` admin pages with `Cache-Control: no-store, private` too.

## Step 3 — Path matcher in your URL map

Add a host rule for `file-vault.example.com` pointing at a new path matcher. Default service = frontend; `/d/*` overrides to API.

```bash
gcloud compute url-maps add-path-matcher YOUR_EXISTING_URL_MAP \
  --project=$PROJECT \
  --path-matcher-name=file-vault \
  --default-service=file-vault-frontend-$ENV \
  --backend-service-path-rules='/d/*=file-vault-api-'$ENV

gcloud compute url-maps add-host-rule YOUR_EXISTING_URL_MAP \
  --project=$PROJECT \
  --hosts=file-vault.example.com \
  --path-matcher-name=file-vault
```

The `/api/*` paths on the API service are reached only by the frontend over the VPC mesh — they do **not** need a public path rule.

## Step 4 — Cloud Armor (optional, but recommended)

A minimal policy on the API backend to throttle per-IP guessing against `/d/*`:

```bash
gcloud compute security-policies create file-vault-api-$ENV \
  --project=$PROJECT \
  --type=CLOUD_ARMOR

gcloud compute security-policies rules create 1000 \
  --project=$PROJECT \
  --security-policy=file-vault-api-$ENV \
  --src-ip-ranges='*' \
  --action=throttle \
  --rate-limit-threshold-count=60 \
  --rate-limit-threshold-interval-sec=60 \
  --conform-action=allow \
  --exceed-action=deny-429 \
  --enforce-on-key=IP

gcloud compute backend-services update file-vault-api-$ENV \
  --project=$PROJECT --global \
  --security-policy=file-vault-api-$ENV
```

This is ~$8/mo flat (1 policy + 1 rule) plus per-request fees — see the cost note in the architecture conversation.

## Verifying it works

```bash
# Should return the frontend upload page.
curl -sI https://file-vault.example.com/

# Unknown token — API returns 404 (deliberate; not 410, to avoid enumeration signal).
curl -sI https://file-vault.example.com/d/not-a-real-token
```
