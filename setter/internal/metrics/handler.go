// Package metrics provides functionality for managing RDS cluster tags.
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"counter/internal/version"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/sirupsen/logrus"
)

// RDSAPI defines the RDS operations we use for tag management.
type RDSAPI interface {
	DescribeDBInstances(*rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	AddTagsToResource(*rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error)
}

// STSAPI defines the STS operations we use for AWS identity operations.
type STSAPI interface {
	GetCallerIdentity(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

// Handler manages RDS cluster tag operations with AWS service clients and logging.
type Handler struct {
	logger logrus.FieldLogger
	rds    RDSAPI
	sts    STSAPI
}

// NewHandler creates a new Handler instance with the provided dependencies.
func NewHandler(logger logrus.FieldLogger, rdsClient RDSAPI, stsClient STSAPI) *Handler {
	return &Handler{
		logger: logger,
		rds:    rdsClient,
		sts:    stsClient,
	}
}

// EventDetail represents the CloudWatch event detail containing the RDS instance identifier.
type EventDetail struct {
	SourceIdentifier string `json:"SourceIdentifier"`
}

// loggerFromContext extracts Lambda context and returns a logger with request metadata.
func loggerFromContext(ctx context.Context) *logrus.Entry {
	lambdaCtx, ok := lambdacontext.FromContext(ctx)
	if !ok {
		return logrus.WithFields(logrus.Fields{
			"aws_request_id": "👽️",
			"version":        version.Version,
			"commit":         version.GitCommit,
			"built_at":       version.BuildTime,
		})
	}

	fields := logrus.Fields{
		"aws_request_id":   lambdaCtx.AwsRequestID,
		"function_name":    lambdacontext.FunctionName,
		"function_version": lambdacontext.FunctionVersion,
		"version":          version.Version,
		"commit":           version.GitCommit,
		"built_at":         version.BuildTime,
	}

	return logrus.WithFields(fields)
}

// getClusterIdentifier retrieves the cluster ID for a given RDS instance.
func (h *Handler) getClusterIdentifier(DBInstanceIdentifier string) (string, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(DBInstanceIdentifier),
	}

	output, err := h.rds.DescribeDBInstances(input)
	if err != nil {
		return "", fmt.Errorf("failed to describe DB instance: %w", err)
	}

	if len(output.DBInstances) == 0 {
		return "", fmt.Errorf("no DB instance found with ID: %s", DBInstanceIdentifier)
	}

	dbInstance := output.DBInstances[0]
	if dbInstance.DBClusterIdentifier == nil || dbInstance.DBInstanceArn == nil {
		return "", fmt.Errorf("instance %s is not part of a cluster or details are missing", DBInstanceIdentifier)
	}

	return aws.StringValue(dbInstance.DBClusterIdentifier), nil
}

// HandleRequest processes CloudWatch events to update RDS instance tags.
func (h *Handler) HandleRequest(ctx context.Context, event events.CloudWatchEvent) error {
	h.logger = loggerFromContext(ctx)

	// Validate required environment variables.
	expectedClusterID := os.Getenv("RDS_CLUSTER_IDENTIFIER")
	if expectedClusterID == "" {
		h.logger.Printf("RDS_CLUSTER_IDENTIFIER environment variable is not set")
		return fmt.Errorf("RDS_CLUSTER_IDENTIFIER environment variable is required")
	}

	tagsEnv := os.Getenv("TAGS")

	var tagsMap map[string]string

	if err := json.Unmarshal([]byte(tagsEnv), &tagsMap); err != nil {
		h.logger.Printf("Error parsing tags from environment: %v", err)
		return err
	}

	var detail EventDetail
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		h.logger.Printf("Error unmarshalling event detail: %v", err)
		return err
	}

	dbInstanceID := detail.SourceIdentifier
	h.logger.Printf("Received event for DB instance: %s", dbInstanceID)

	// Validate instance type and cluster membership.
	if !strings.Contains(dbInstanceID, "application-autoscaling-") {
		h.logger.Printf("DB instance %s is not an Aurora instance. Skipping.", dbInstanceID)
		return nil
	}

	clusterID, err := h.getClusterIdentifier(dbInstanceID)
	if err != nil {
		h.logger.Printf("Error getting cluster identifier for instance %s: %v", dbInstanceID, err)
		return err
	}

	if clusterID != expectedClusterID {
		h.logger.Printf("DB instance %s is not a member of cluster %s. Skipping.", dbInstanceID, expectedClusterID)
		return nil
	}

	// Get AWS account information for ARN construction.
	callerIdentityOutput, err := h.sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		h.logger.Printf("Error getting AWS caller identity: %v", err)
		return err
	}

	// Prepare tags for application.
	awsTags := make([]*rds.Tag, 0, len(tagsMap))
	for k, v := range tagsMap {
		awsTags = append(awsTags, &rds.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	// Apply tags to the RDS instance.
	arn := fmt.Sprintf("arn:aws:rds:us-east-1:%s:db:%s", *callerIdentityOutput.Account, dbInstanceID)
	_, err = h.rds.AddTagsToResource(&rds.AddTagsToResourceInput{
		ResourceName: aws.String(arn),
		Tags:         awsTags,
	})

	if err != nil {
		h.logger.Printf("Error adding tags to DB instance %s: %v", dbInstanceID, err)
		return err
	}

	return nil
}
