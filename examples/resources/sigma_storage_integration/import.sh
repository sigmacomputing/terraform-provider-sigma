# bucket_name and path_prefix are not returned by Sigma: set them in config and the next apply reconciles.
terraform import sigma_storage_integration.example '<storage_integration_id>'
