# State bucket must exist before `terraform init`. Create it once manually:
#
#   gcloud storage buckets create gs://<your-tfstate-bucket> \
#     --location=<region> --uniform-bucket-level-access
#   gcloud storage buckets update gs://<your-tfstate-bucket> --versioning
#
# Then uncomment the block below and run `terraform init -migrate-state`.
#
# terraform {
#   backend "gcs" {
#     bucket = "REPLACE-WITH-TFSTATE-BUCKET"
#     prefix = "file-vault/dev"
#   }
# }
