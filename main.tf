resource "aws_iam_role" "lambda_exec_role" {
  name               = "ro_set_tags_${var.rds_cluster_identifier}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role_policy.json
  tags               = var.tags
}

data "aws_iam_policy_document" "lambda_assume_role_policy" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "lambda_permissions" {
  role   = aws_iam_role.lambda_exec_role.name
  policy = data.aws_iam_policy_document.lambda_permissions_policy.json
}

data "aws_iam_policy_document" "lambda_permissions_policy" {
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "rds-data:*",
      "rds:*",
    ]
    resources = ["*"]
  }
}

# Build the Go binary and create zip file
resource "null_resource" "lambda_builder" {
  # Trigger rebuild on code changes
  triggers = {
    code_hash = sha256(join("", [
      for f in fileset("${path.module}/setter", "**/*") : filesha256("${path.module}/setter/${f}") if !contains([
        "main.zip",
        "bootstrap",
        ".gitignore",
      ], f)
    ]))
  }

  provisioner "local-exec" {
    working_dir = "${path.module}/setter"
    command     = <<EOT
      # Check if go and make are installed
      if ! command -v go &> /dev/null; then
        echo "go command not found, please install Go"
        exit 1
      fi

      if ! command -v make &> /dev/null; then
        echo "make command not found, please install Make"
        exit 1
      fi

      # If both commands are found, run the build and package steps
      make clean && make build && make package
    EOT
  }
}

# Create zip file for Lambda
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/setter/bootstrap"
  output_path = "${path.module}/setter/main.zip"

  depends_on = [null_resource.lambda_builder]
}

resource "aws_lambda_function" "lambda" {
  filename         = data.archive_file.lambda_zip.output_path
  function_name    = "ro_set_tags_${var.rds_cluster_identifier}"
  role             = aws_iam_role.lambda_exec_role.arn
  handler          = "HandleRequest"
  memory_size      = 128
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256

  runtime = "provided.al2"

  tags = merge(var.tags, {
    Name = "ro_set_tags_${var.rds_cluster_identifier}"
  })

  environment {
    variables = {
      TAGS                   = jsonencode(var.push_tags),
      RDS_CLUSTER_IDENTIFIER = var.rds_cluster_identifier,
    }
  }
  lifecycle {
    ignore_changes = [
      last_modified,
    ]
  }
}

resource "aws_cloudwatch_event_rule" "read_replica_created" {
  name        = "ro_set_tags_${var.rds_cluster_identifier}"
  description = "Trigger Lambda when instance is created in ${var.rds_cluster_identifier}"
  event_pattern = jsonencode({
    "source" : ["aws.rds"],
    "detail-type" : ["RDS DB Instance Event"],
    "detail" : {
      "EventID" : ["RDS-EVENT-0005"] # Event: "DB instance created"
    }
  })
}

resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "ro_set_tags_${var.rds_cluster_identifier}"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.lambda.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.read_replica_created.arn
}

resource "aws_cloudwatch_event_target" "read_replica_target" {
  rule      = aws_cloudwatch_event_rule.read_replica_created.name
  target_id = "ro_set_tags_${var.rds_cluster_identifier}"
  arn       = aws_lambda_function.lambda.arn
}
