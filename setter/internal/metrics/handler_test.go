package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Package metrics provides tests for RDS tag management functionality.
type mockRDS struct {
	RDSAPI
	describeDBInstancesFunc func(*rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	addTagsToResourceFunc   func(*rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error)
}

// mockRDS simulates the Planet Express RDS delivery system for testing.
func (m *mockRDS) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	if m.describeDBInstancesFunc != nil {
		return m.describeDBInstancesFunc(input)
	}

	return nil, fmt.Errorf("DescribeDBInstances not implemented")
}

// AddTagsToResource returns mock response or error based on the configured function.
func (m *mockRDS) AddTagsToResource(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
	if m.addTagsToResourceFunc != nil {
		return m.addTagsToResourceFunc(input)
	}

	return nil, fmt.Errorf("AddTagsToResource not implemented")
}

// mockSTS simulates the Space Transport Security service for testing.
type mockSTS struct {
	STSAPI
	getCallerIdentityFunc func(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

// GetCallerIdentity returns mock response or error based on the configured function.
func (m *mockSTS) GetCallerIdentity(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	if m.getCallerIdentityFunc != nil {
		return m.getCallerIdentityFunc(input)
	}

	return nil, fmt.Errorf("GetCallerIdentity not implemented")
}

// TestHandler_HandleRequest tests all paths of the HandleRequest method.
// Each test case is named after a Futurama character and simulates their unique scenarios:
//   - Nibbler: Non-autoscaling instance that should be skipped.
//   - Hypnotoad: STS errors with mind-bending messages.
//   - Zoidberg: Permission denied because nobody likes him.
//   - Fry: Happy path, because he occasionally gets things right.
//   - Leela: Invalid inputs, she's too practical for that.
//   - Amy: Missing configurations, like her missing doctorate.
//   - Hermes: Bureaucratic environment variable errors.
//   - Mom: Different cluster, because she runs a competing company.
func TestHandler_HandleRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Setup the Planet Express delivery system (RDS mock)
	defaultMockRDS := &mockRDS{
		describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{
					{
						// Bender's always part of the default setup
						DBClusterIdentifier: aws.String("planet-express"),
						DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:db:bender"),
					},
				},
			}, nil
		},
		addTagsToResourceFunc: func(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
			return &rds.AddTagsToResourceOutput{}, nil
		},
	}

	// Setup default mock STS client that returns a fixed account ID.
	defaultMockSTS := &mockSTS{
		getCallerIdentityFunc: func(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
			return &sts.GetCallerIdentityOutput{
				Account: aws.String("123456789012"),
			}, nil
		},
	}

	// Professor Farnsworth's tag specifications
	validTags := map[string]string{
		"Owner":   "professor-farnsworth",
		"Purpose": "delivery-company",
	}
	tagsJSON, err := json.Marshal(validTags)
	require.NoError(t, err)

	// Test cases represent different delivery scenarios
	tests := []struct {
		// Test case name, should describe the scenario being tested.
		name string
		// CloudWatch event input for the test case.
		event events.CloudWatchEvent
		// Environment variables required for the test.
		envVars map[string]string
		// Mock RDS client for this test case.
		rds RDSAPI
		// Mock STS client for this test case.
		sts STSAPI
		// Whether the test should result in an error.
		wantErr bool
		// Optional setup function run before the test.
		setup func()
		// Optional cleanup function run after the test.
		cleanup func()
	}{
		// Nibbler: Small but important, just skips non-autoscaling instances
		{
			name: "non-autoscaling instance",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "nibbler"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: false,
		},
		// Hypnotoad: STS errors with mind-bending messages
		{
			name: "sts get caller identity error",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-hypnotoad"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: defaultMockRDS,
			sts: &mockSTS{
				getCallerIdentityFunc: func(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
					return nil, fmt.Errorf("ALL GLORY TO THE HYPNOTOAD")
				},
			},
			wantErr: true,
		},
		// Zoidberg: Poor guy can't even get permission to add tags
		{
			name: "permission denied error",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-fry"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return &rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBClusterIdentifier: aws.String("planet-express"),
								DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:db:zoidberg"),
							},
						},
					}, nil
				},
				addTagsToResourceFunc: func(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
					return nil, fmt.Errorf("failed to add tags: permission denied")
				},
			},
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Mom: Different cluster, because she runs a competing company
		{
			name: "instance from different cluster",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-mom"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return &rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBClusterIdentifier: aws.String("momcorp"),
								DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:db:walt"),
							},
						},
					}, nil
				},
			},
			sts:     defaultMockSTS,
			wantErr: false,
		},
		// Test case: Invalid JSON in event detail should return error.
		{
			name: "invalid event detail",
			event: events.CloudWatchEvent{
				Detail: []byte(`invalid json`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: Tag addition failure should return error.
		{
			name: "add tags error",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-zoidberg"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return &rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBClusterIdentifier: aws.String("planet-express"),
								DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:db:zoidberg"),
							},
						},
					}, nil
				},
				addTagsToResourceFunc: func(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
					return nil, fmt.Errorf("failed to add tags: permission denied")
				},
			},
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: Happy path - successful tag addition to autoscaling instance.
		{
			name: "autoscaling instance with valid cluster",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-fry"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: false,
		},
		// Test case: Invalid tags JSON in environment should return error.
		{
			name: "autoscaling instance with invalid tags JSON",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-leela"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   "invalid json",
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: Empty cluster identifier should return error.
		{
			name: "missing RDS_CLUSTER_IDENTIFIER",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-amy"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "",
				"TAGS":                   string(tagsJSON),
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: Missing environment variable should return error.
		{
			name: "missing RDS_CLUSTER_IDENTIFIER environment variable",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-hermes"}`),
			},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: RDS API error should be propagated.
		{
			name: "get cluster identifier error",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-scruffy"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return nil, fmt.Errorf("failed to get cluster identifier")
				},
			},
			sts:     defaultMockSTS,
			wantErr: true,
		},
		// Test case: Instance from different cluster should be skipped without error.
		{
			name: "instance from different cluster",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-mom"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return &rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBClusterIdentifier: aws.String("momcorp"),
								DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:db:walt"),
							},
						},
					}, nil
				},
			},
			sts:     defaultMockSTS,
			wantErr: false,
		},
		{
			name: "aws api throttling error",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-bender"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				describeDBInstancesFunc: func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
					return nil, awserr.New(
						"ThrottlingException",
						"Rate exceeded",
						errors.New("request throttled"),
					)
				},
			},
			sts:     defaultMockSTS,
			wantErr: true,
		},
		{
			name: "malformed arn",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-leela"}`),
			},
			envVars: map[string]string{
				"RDS_CLUSTER_IDENTIFIER": "planet-express",
				"TAGS":                   string(tagsJSON),
			},
			rds: &mockRDS{
				addTagsToResourceFunc: func(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
					return nil, fmt.Errorf("InvalidParameterValue: Invalid resource name: invalid-arn")
				},
			},
			sts:     defaultMockSTS,
			wantErr: true,
		},
		{
			name: "missing environment variables",
			event: events.CloudWatchEvent{
				Detail: []byte(`{"SourceIdentifier": "application-autoscaling-zoidberg"}`),
			},
			envVars: map[string]string{},
			rds:     defaultMockRDS,
			sts:     defaultMockSTS,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		// Run each test case in isolation to prevent cross-test pollution.
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated logging environment for each test.
			var logBuf bytes.Buffer

			testLogger := logrus.New()
			testLogger.SetOutput(&logBuf)

			logrus.SetOutput(&logBuf)

			// Run any test-specific setup.
			if tt.setup != nil {
				tt.setup()
			}

			// Ensure global logger is restored after test.
			defer logrus.SetOutput(os.Stdout)

			// Create handler with test-specific dependencies.
			handler := NewHandler(testLogger, tt.rds, tt.sts)

			// Manage environment variables to prevent test pollution.
			originalEnv := make(map[string]string)

			for k := range tt.envVars {
				if v, ok := os.LookupEnv(k); ok {
					originalEnv[k] = v
				}
			}

			// Ensure environment is restored after test.
			t.Cleanup(func() {
				for k := range tt.envVars {
					if orig, ok := originalEnv[k]; ok {
						err := os.Setenv(k, orig)
						if err != nil {
							t.Logf("Failed to restore environment variable %s: %v", k, err)
						}
					} else {
						err := os.Unsetenv(k)
						if err != nil {
							t.Logf("Failed to unset environment variable %s: %v", k, err)
						}
					}
				}
			})

			// Apply test-specific environment variables.
			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				if err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", k, err)
				}
			}

			// Create test Lambda context with known values.
			lc := &lambdacontext.LambdaContext{
				AwsRequestID:       "test-request-id",
				InvokedFunctionArn: "test-function-arn",
			}
			ctx := lambdacontext.NewContext(context.Background(), lc)

			// Run the handler and verify results.
			err := handler.HandleRequest(ctx, tt.event)
			if tt.wantErr {
				assert.Error(t, err, "Handler should return error")
			} else {
				assert.NoError(t, err, "Handler should not return error")
			}

			// Verify specific error messages in logs.
			if tt.name == "sts get caller identity error" {
				logOutput := logBuf.String()
				assert.Contains(t, logOutput, "Error getting AWS caller identity: ALL GLORY TO THE HYPNOTOAD")
			}

			// Run any test-specific cleanup.
			if tt.cleanup != nil {
				tt.cleanup()
			}
		})
	}
}

// TestHandler_getClusterIdentifier tests the cluster identifier retrieval functionality.
func TestHandler_getClusterIdentifier(t *testing.T) {
	mockRDS := &mockRDS{}
	mockSTS := &mockSTS{}
	handler := NewHandler(logrus.New(), mockRDS, mockSTS)

	tests := []struct {
		name          string
		instanceID    string
		mockResponse  *rds.DescribeDBInstancesOutput
		mockError     error
		wantClusterID string
		wantErr       bool
	}{
		{
			name:       "invalid instance ID",
			instanceID: "non-existent-instance",
			mockError:  fmt.Errorf("instance not found"),
			wantErr:    true,
		},
		{
			name:       "valid instance ID",
			instanceID: "test-instance",
			mockResponse: &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{
					{
						DBClusterIdentifier: aws.String("test-cluster"),
						DBInstanceArn:       aws.String("test-arn"),
					},
				},
			},
			wantClusterID: "test-cluster",
			wantErr:       false,
		},
		{
			name:       "instance not in cluster",
			instanceID: "standalone-instance",
			mockResponse: &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{
					{
						DBInstanceArn: aws.String("test-arn"),
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "empty response",
			instanceID: "empty-response",
			mockResponse: &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRDS.describeDBInstancesFunc = func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
				assert.Equal(t, tt.instanceID, aws.StringValue(input.DBInstanceIdentifier))
				return tt.mockResponse, tt.mockError
			}

			clusterID, err := handler.getClusterIdentifier(tt.instanceID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, clusterID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantClusterID, clusterID)
			}
		})
	}
}

// TestLoggerFromContext verifies proper logger initialization from Lambda context.
func TestLoggerFromContext(t *testing.T) {
	tests := []struct {
		// Test case name describing the scenario.
		name string
		// Function to create context with or without Lambda metadata.
		setupContext func() context.Context
		// Expected AWS request ID in logger fields.
		expectedField string
	}{
		// Test case: Lambda context available with request ID.
		{
			name: "with lambda context",
			setupContext: func() context.Context {
				// Create Lambda context with known values.
				lc := &lambdacontext.LambdaContext{
					AwsRequestID:       "test-request-id",
					InvokedFunctionArn: "test-function-arn",
				}
				return lambdacontext.NewContext(context.Background(), lc)
			},
			expectedField: "test-request-id",
		},
		// Test case: No Lambda context, should use fallback emoji.
		{
			name: "without lambda context",
			setupContext: func() context.Context {
				// Return empty context without Lambda metadata.
				return context.Background()
			},
			expectedField: "👽️",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context based on test case setup.
			ctx := tt.setupContext()
			// Get logger from context and verify fields.
			logger := loggerFromContext(ctx)

			assert.NotNil(t, logger)
			// Verify AWS request ID is set correctly in logger fields.
			fields := logger.Data
			assert.Contains(t, fields, "aws_request_id")
			assert.Equal(t, tt.expectedField, fields["aws_request_id"])
		})
	}
}

// TestNewHandler verifies proper handler initialization with dependencies.
func TestNewHandler(t *testing.T) {
	logger := logrus.New()
	mockRDS := &mockRDS{}
	mockSTS := &mockSTS{}
	handler := NewHandler(logger, mockRDS, mockSTS)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
	assert.Equal(t, mockRDS, handler.rds)
	assert.Equal(t, mockSTS, handler.sts)
}
