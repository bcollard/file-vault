# Wiring file-vault into your Cloud Load Balancer

Terraform creates one serverless NEG for the file-vault service but never
touches your load balancer. Attach it as a backend on the external HTTPS
Application Load Balancer you already manage.

```
$ terraform -chdir=terraform/envs/dev output neg_id
"projects/<project>/regions/europe-west9/networkEndpointGroups/file-vault-dev"
```

## Layout

```
  file-vault.example.com
            |
   [ external HTTPS ALB ]  (+ Cloud Armor)
            |
   host rule → file-vault backend service → serverless NEG → Cloud Run
```

Every path goes to the same backend: the service serves the UI, the API and
nothing else. There are no path rules to maintain.

## Create the backend service

```bash
PROJECT=your-gcp-project
REGION=europe-west9
ENV=dev

gcloud compute backend-services create file-vault-$ENV \
  --project=$PROJECT --global \
  --load-balancing-scheme=EXTERNAL_MANAGED \
  --protocol=HTTPS

gcloud compute backend-services add-backend file-vault-$ENV \
  --project=$PROJECT --global \
  --network-endpoint-group=file-vault-$ENV \
  --network-endpoint-group-region=$REGION
```

Logging on the backend service is safe to enable: tokens and keys are in the
URL fragment or POST bodies, never in request URLs.

## CDN

Optional. If you enable Cloud CDN, use `--cache-mode=USE_ORIGIN_HEADERS`. The
service marks hashed assets `immutable`, pages `no-cache`, and API responses
`no-store`.

## Host rule

```bash
gcloud compute url-maps add-path-matcher YOUR_EXISTING_URL_MAP \
  --project=$PROJECT \
  --path-matcher-name=file-vault-$ENV \
  --default-service=file-vault-$ENV

gcloud compute url-maps add-host-rule YOUR_EXISTING_URL_MAP \
  --project=$PROJECT \
  --hosts=file-vault.example.com \
  --path-matcher-name=file-vault-$ENV
```

## Cloud Armor (recommended)

Per-IP throttling at the edge, in front of the per-instance Go limiter:

```bash
gcloud compute security-policies create file-vault-$ENV \
  --project=$PROJECT --type=CLOUD_ARMOR

gcloud compute security-policies rules create 1000 \
  --project=$PROJECT \
  --security-policy=file-vault-$ENV \
  --src-ip-ranges='*' \
  --action=throttle \
  --rate-limit-threshold-count=120 \
  --rate-limit-threshold-interval-sec=60 \
  --conform-action=allow \
  --exceed-action=deny-429 \
  --enforce-on-key=IP

gcloud compute backend-services update file-vault-$ENV \
  --project=$PROJECT --global \
  --security-policy=file-vault-$ENV
```

## Client IP

The service takes the client address from the second-from-right
`X-Forwarded-For` entry (`TRUSTED_PROXY_HOPS=2`), which is what the external
ALB appends. If you put another proxy in front of the LB, raise the hop count
in `terraform/modules/app/variables.tf`.

## Verify

```bash
curl -sI https://file-vault.example.com/            # 200, the upload page
curl -s  https://file-vault.example.com/api/config   # JSON limits
curl -s  -X POST https://file-vault.example.com/api/download/peek \
  -H 'content-type: application/json' -d '{"token":"x"}'   # 404 — deliberate
```
