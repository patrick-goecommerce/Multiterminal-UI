You are working on a project that uses AWS services. Follow these principles:

- Follow the principle of least privilege for all IAM policies. Use specific resource ARNs and actions, never `*/*` in production.
- Use IAM roles (not access keys) for service-to-service authentication. Use instance profiles for EC2, execution roles for Lambda.
- For Lambda functions, keep cold start times low: minimize package size, use provisioned concurrency for latency-sensitive paths, prefer ARM64 (Graviton) for cost savings.
- Set Lambda memory based on profiling, not guessing. CPU scales proportionally with memory. Use AWS Lambda Power Tuning.
- Use S3 lifecycle policies to transition infrequently accessed objects to cheaper storage classes. Enable versioning for critical buckets.
- Enable S3 server-side encryption (SSE-S3 or SSE-KMS) by default. Block public access at the account level unless explicitly required.
- Design DynamoDB tables around access patterns. Use single-table design for related entities. Choose partition keys that distribute traffic evenly.
- Set DynamoDB capacity mode based on workload: on-demand for unpredictable traffic, provisioned with auto-scaling for steady workloads.
- Use SQS for async decoupling between services. Set visibility timeouts to at least 6x the consumer processing time. Use DLQs for failed messages.
- Use CloudWatch alarms for operational metrics. Set alarms on error rates and latency, not just resource utilization.
- Use AWS CDK or Terraform for infrastructure as code. Avoid ClickOps for anything that needs to be reproducible.
- Enable CloudTrail and VPC Flow Logs for audit and security forensics. Centralize logs in a dedicated logging account.
- Use Parameter Store or Secrets Manager for configuration and secrets, not environment variables with hardcoded values.
- Use VPC endpoints (PrivateLink) for services like S3, DynamoDB, and SQS to keep traffic off the public internet.
- Implement multi-AZ deployments for production workloads. Use us-east-1 only if you have a reason; choose a region close to your users.
