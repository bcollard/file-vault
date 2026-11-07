# Partial configuration: the state bucket is deployment-specific, so it isn't
# committed. Locally: cp backend.hcl.example backend.hcl (gitignored), then
#   terraform init -backend-config=backend.hcl
# CI passes -backend-config="bucket=${{ vars.TF_STATE_BUCKET }}".
terraform {
  backend "gcs" {
    prefix = "file-vault/dev"
  }
}
