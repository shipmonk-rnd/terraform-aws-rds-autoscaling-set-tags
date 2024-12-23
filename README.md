# Terraform AWS RDS autoscaled read replica set tags

This lambda functions sets tags on the autoscaled read replicas in the AWS Aurora cluster. Replica has to start with `application-autoscaling-` prefix.

## Usage

```hcl
module "rds_cluster_ro_set_tags" {
  source = "git::git@gitlab.com:shipmonk-company/platform/tf-module/terraform-aws-rds-autoscaling-set-tags.git?ref=CHECK_CHANGELOG_FOR_THE_LATEST_VERSION"

  rds_cluster_identifier = module.prod-aurora.cluster_identifier
  tags                   = var.tags
  push_tags = merge(
    local.prod_aurora_tags,
    {
      "rds_autoscaled_instance" = "True"
    }
  )
}
```

## Before you do anything in this module

Install pre-commit hooks by running following commands:

```shell script
brew install pre-commit
pre-commit install
```

<!-- BEGINNING OF PRE-COMMIT-TERRAFORM DOCS HOOK -->
## Requirements

No requirements.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_archive"></a> [archive](#provider\_archive) | n/a |
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [aws_cloudwatch_event_rule.read_replica_created](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_rule) | resource |
| [aws_cloudwatch_event_target.read_replica_target](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_event_target) | resource |
| [aws_iam_role.lambda_exec_role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.lambda_permissions](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_lambda_function.lambda](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_function) | resource |
| [aws_lambda_permission.allow_eventbridge](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_permission) | resource |
| [null_resource.lambda_builder](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [archive_file.lambda_zip](https://registry.terraform.io/providers/hashicorp/archive/latest/docs/data-sources/file) | data source |
| [aws_iam_policy_document.lambda_assume_role_policy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_iam_policy_document.lambda_permissions_policy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_do_not_creat_event_bridge"></a> [do\_not\_creat\_event\_bridge](#input\_do\_not\_creat\_event\_bridge) | If set to true, the event bridge rule will not be created | `bool` | `false` | no |
| <a name="input_push_tags"></a> [push\_tags](#input\_push\_tags) | Tags to be pushed to the new scaled read replica | `map(string)` | `{}` | no |
| <a name="input_rds_cluster_identifier"></a> [rds\_cluster\_identifier](#input\_rds\_cluster\_identifier) | The identifier of the RDS cluster, used only for setting up event bridge and tf resources naming | `any` | n/a | yes |
| <a name="input_tags"></a> [tags](#input\_tags) | A map of tags to add to all resources | `map(string)` | `{}` | no |

## Outputs

No outputs.
<!-- END OF PRE-COMMIT-TERRAFORM DOCS HOOK -->
