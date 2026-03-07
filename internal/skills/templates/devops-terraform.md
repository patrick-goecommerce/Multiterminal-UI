You are working on a project that uses Terraform. Follow these principles:

- Organize code into reusable modules. Each module should manage a single logical resource group (e.g., VPC, database, application).
- Use remote state backends (S3 + DynamoDB, GCS, Terraform Cloud) with state locking. Never commit `.tfstate` files to version control.
- Always run `terraform plan` before `terraform apply`. Review the plan output carefully, especially for destroy/recreate operations.
- Use `terraform fmt` and `terraform validate` in CI. Enforce consistent formatting across the team.
- Define all configurable values as `variable` blocks with descriptions, types, and validation rules. Use `locals` for computed values.
- Use `tfvars` files per environment (e.g., `dev.tfvars`, `prod.tfvars`). Never hardcode environment-specific values in modules.
- Tag all resources with at minimum: `Environment`, `Project`, `ManagedBy=terraform`, and `Owner`. Use a shared `default_tags` block where supported.
- Use `lifecycle` blocks deliberately: `prevent_destroy` for critical resources, `create_before_destroy` for zero-downtime replacements.
- Pin provider and module versions with exact constraints (e.g., `~> 5.0`). Update deliberately and test before upgrading.
- Use `data` sources to reference existing resources. Never import or hardcode IDs of resources managed outside the current state.
- Use workspaces or directory-based separation for environment isolation. Directory-based is clearer for significant environment differences.
- Implement `moved` blocks when refactoring resource addresses to avoid destroy/recreate cycles.
- Use `sensitive = true` for variables containing secrets. Integrate with a secrets manager (Vault, AWS Secrets Manager) for runtime secrets.
- Write `output` blocks for values other modules or stacks need. Document outputs clearly.
- Use `checkov`, `tfsec`, or `trivy` for static security analysis of Terraform code. Run these in CI alongside `plan`.
