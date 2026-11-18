resource "sigma_storage_integration" "exports" {
  provider_type        = "s3"
  name                 = "exports"
  bucket_name          = "my-sigma-exports"
  path_prefix          = "sigma/"
  storage_aws_role_arn = "arn:aws:iam::123456789012:role/sigma-exports"
}
