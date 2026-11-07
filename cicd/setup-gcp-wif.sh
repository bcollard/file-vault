#!/usr/bin/env bash
# One-time setup for GitHub Actions → GCP via Workload Identity Federation.
#
# Creates two service accounts (build + terraform), an Artifact Registry repo,
# a versioned Terraform state bucket, a Workload Identity pool + OIDC provider
# scoped to one repo, and the IAM bindings to tie them together.
#
# Configure by copying setup.env.example to setup.env and filling in values,
# or by exporting the variables before running.
#
# Run on a workstation with gcloud authenticated as a project owner. Re-running
# is safe — each resource check skips creation if it already exists.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/setup.env" ]]; then
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/setup.env"
fi

: "${PROJECT_ID:?Set PROJECT_ID in cicd/setup.env or environment}"
: "${GH_REPO_OWNER:?Set GH_REPO_OWNER}"
: "${GH_REPO_NAME:?Set GH_REPO_NAME}"

REGION="${REGION:-europe-west9}"
AR_REPO="${AR_REPO:-file-vault}"
TFSTATE_BUCKET="${TFSTATE_BUCKET:-${PROJECT_ID}-tfstate-file-vault}"
WI_POOL="${WI_POOL:-gitops-pool}"
WI_PROVIDER="${WI_PROVIDER:-gh-provider}"

SA_BUILD="gha-build-file-vault"
SA_TF="gha-tf-file-vault"
SA_BUILD_EMAIL="${SA_BUILD}@${PROJECT_ID}.iam.gserviceaccount.com"
SA_TF_EMAIL="${SA_TF}@${PROJECT_ID}.iam.gserviceaccount.com"

echo "→ Project:   ${PROJECT_ID}"
echo "→ Region:    ${REGION}"
echo "→ AR repo:   ${AR_REPO}"
echo "→ tfstate:   gs://${TFSTATE_BUCKET}"
echo "→ Build SA:  ${SA_BUILD_EMAIL}"
echo "→ TF SA:     ${SA_TF_EMAIL}"
echo "→ Repo:      github.com/${GH_REPO_OWNER}/${GH_REPO_NAME}"
echo

# ── 0. Enable required APIs (idempotent) ──────────────────────────────
echo "→ Enabling required APIs"
gcloud services enable \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  artifactregistry.googleapis.com \
  storage.googleapis.com \
  cloudresourcemanager.googleapis.com \
  --project="${PROJECT_ID}" --quiet >/dev/null

# ── 1. Service accounts (idempotent) ──────────────────────────────────
for sa in "${SA_BUILD}|GHA builder · file-vault" \
          "${SA_TF}|GHA terraform · file-vault"; do
  name="${sa%%|*}"
  display="${sa#*|}"
  email="${name}@${PROJECT_ID}.iam.gserviceaccount.com"
  if ! gcloud iam service-accounts describe "${email}" --project="${PROJECT_ID}" >/dev/null 2>&1; then
    echo "→ Creating service account ${name}"
    gcloud iam service-accounts create "${name}" \
      --project="${PROJECT_ID}" \
      --display-name="${display}"
  else
    echo "✓ Service account ${name} already exists"
  fi
done

# ── 2. Artifact Registry repo (idempotent) ────────────────────────────
if ! gcloud artifacts repositories describe "${AR_REPO}" \
       --project="${PROJECT_ID}" --location="${REGION}" >/dev/null 2>&1; then
  echo "→ Creating Artifact Registry repo ${AR_REPO}"
  gcloud artifacts repositories create "${AR_REPO}" \
    --project="${PROJECT_ID}" \
    --location="${REGION}" \
    --repository-format=docker \
    --description="file-vault container images"
else
  echo "✓ AR repo ${AR_REPO} already exists"
fi

# ── 3. tfstate bucket (idempotent, versioned) ─────────────────────────
if ! gcloud storage buckets describe "gs://${TFSTATE_BUCKET}" >/dev/null 2>&1; then
  echo "→ Creating tfstate bucket gs://${TFSTATE_BUCKET}"
  gcloud storage buckets create "gs://${TFSTATE_BUCKET}" \
    --project="${PROJECT_ID}" \
    --location="${REGION}" \
    --uniform-bucket-level-access
  gcloud storage buckets update "gs://${TFSTATE_BUCKET}" --versioning
else
  echo "✓ tfstate bucket gs://${TFSTATE_BUCKET} already exists"
fi

# ── 4. IAM bindings for the build SA ──────────────────────────────────
echo "→ Granting AR writer to ${SA_BUILD}"
gcloud artifacts repositories add-iam-policy-binding "${AR_REPO}" \
  --project="${PROJECT_ID}" --location="${REGION}" \
  --member="serviceAccount:${SA_BUILD_EMAIL}" \
  --role="roles/artifactregistry.writer" \
  --condition=None --quiet >/dev/null

# ── 5. IAM bindings for the terraform SA ──────────────────────────────
# Editor covers Cloud Run, KMS, Firestore, GCS, Compute, Secret Manager,
# Scheduler, Monitoring, Logging. IAM admin + SA admin cover IAM bindings
# (Editor can't manage IAM). Service usage admin enables APIs.
for role in roles/editor \
            roles/iam.securityAdmin \
            roles/iam.serviceAccountAdmin \
            roles/serviceusage.serviceUsageAdmin; do
  echo "→ Granting ${role} to ${SA_TF}"
  gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
    --member="serviceAccount:${SA_TF_EMAIL}" \
    --role="${role}" \
    --condition=None --quiet >/dev/null
done

echo "→ Granting tfstate bucket admin to ${SA_TF}"
gcloud storage buckets add-iam-policy-binding "gs://${TFSTATE_BUCKET}" \
  --member="serviceAccount:${SA_TF_EMAIL}" \
  --role="roles/storage.objectAdmin" --quiet >/dev/null

echo "→ Granting AR reader to ${SA_TF}"
gcloud artifacts repositories add-iam-policy-binding "${AR_REPO}" \
  --project="${PROJECT_ID}" --location="${REGION}" \
  --member="serviceAccount:${SA_TF_EMAIL}" \
  --role="roles/artifactregistry.reader" \
  --condition=None --quiet >/dev/null

# ── 6. Workload Identity Pool (idempotent) ────────────────────────────
if ! gcloud iam workload-identity-pools describe "${WI_POOL}" \
       --project="${PROJECT_ID}" --location="global" >/dev/null 2>&1; then
  echo "→ Creating Workload Identity pool ${WI_POOL}"
  gcloud iam workload-identity-pools create "${WI_POOL}" \
    --project="${PROJECT_ID}" --location="global" \
    --display-name="GitOps pool"
else
  echo "✓ WI pool ${WI_POOL} already exists (reusing)"
fi

POOL_ID=$(gcloud iam workload-identity-pools describe "${WI_POOL}" \
  --project="${PROJECT_ID}" --location="global" --format="value(name)")

# ── 7. OIDC provider for GitHub (idempotent) ──────────────────────────
if ! gcloud iam workload-identity-pools providers describe "${WI_PROVIDER}" \
       --project="${PROJECT_ID}" --location="global" \
       --workload-identity-pool="${WI_POOL}" >/dev/null 2>&1; then
  echo "→ Creating OIDC provider ${WI_PROVIDER}"
  gcloud iam workload-identity-pools providers create-oidc "${WI_PROVIDER}" \
    --project="${PROJECT_ID}" --location="global" \
    --workload-identity-pool="${WI_POOL}" \
    --display-name="GitHub provider" \
    --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner" \
    --issuer-uri="https://token.actions.githubusercontent.com"
else
  echo "✓ OIDC provider ${WI_PROVIDER} already exists (reusing)"
fi

# ── 8. Allow this repo to impersonate both SAs ────────────────────────
for sa_email in "${SA_BUILD_EMAIL}" "${SA_TF_EMAIL}"; do
  echo "→ Allowing repo ${GH_REPO_OWNER}/${GH_REPO_NAME} to impersonate ${sa_email}"
  gcloud iam service-accounts add-iam-policy-binding "${sa_email}" \
    --project="${PROJECT_ID}" \
    --role="roles/iam.workloadIdentityUser" \
    --member="principalSet://iam.googleapis.com/${POOL_ID}/attribute.repository/${GH_REPO_OWNER}/${GH_REPO_NAME}" \
    --condition=None --quiet >/dev/null
done

PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')

cat <<EOF

✓ Setup complete.

── Set these GitHub Actions repository variables ───────────────────
   (Settings → Secrets and variables → Actions → Variables)

   GCP_PROJECT_ID        ${PROJECT_ID}
   GCP_PROJECT_NUMBER    ${PROJECT_NUMBER}
   GCP_REGION            ${REGION}        (optional, defaults to europe-west9)
   AR_REPO               ${AR_REPO}       (optional, defaults to file-vault)
   WIF_POOL              ${WI_POOL}       (optional, defaults to gitops-pool)
   WIF_PROVIDER          ${WI_PROVIDER}   (optional, defaults to gh-provider)
   TF_STATE_BUCKET       ${TFSTATE_BUCKET}

   gh variable set GCP_PROJECT_ID     --body ${PROJECT_ID}
   gh variable set GCP_PROJECT_NUMBER --body ${PROJECT_NUMBER}
   gh variable set TF_STATE_BUCKET    --body ${TFSTATE_BUCKET}

── For reference ──────────────────────────────────────────────────
   workload_identity_provider:  projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${WI_POOL}/providers/${WI_PROVIDER}
   build service_account:       ${SA_BUILD_EMAIL}
   terraform service_account:   ${SA_TF_EMAIL}
   Artifact Registry path:      ${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}
   tfstate bucket:              gs://${TFSTATE_BUCKET}

── Next ──────────────────────────────────────────────────────────
1. cp terraform/envs/prod/terraform.tfvars.example terraform/envs/prod/terraform.tfvars
   and set project_id, bucket_name, domain, alert_email (gitignored).
2. gh secret set TFVARS --env prod < terraform/envs/prod/terraform.tfvars
3. For local runs: echo 'bucket = "${TFSTATE_BUCKET}"' > terraform/envs/prod/backend.hcl
4. Push to main: the Deploy workflow builds the image and applies Terraform.
EOF
