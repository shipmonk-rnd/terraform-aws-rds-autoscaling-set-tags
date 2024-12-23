package main

import (
	"flag"
	"fmt"
	"os"

	"counter/internal/metrics"
	"counter/internal/version"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/sirupsen/logrus"
)

// RDS Tag Setter Lambda applies tags to Aurora read replicas created by application autoscaling.

// printVersion outputs version information when running with --version flag.
func printVersion() {
	fmt.Printf("RDS Tag Setter %s (%s) built at %s\n",
		version.Version,
		version.GitCommit,
		version.BuildTime,
	)
}

func main() {
	// Handle version flag for local version checking without Lambda invocation.
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		printVersion()
		os.Exit(0)
	}

	// Log version information to CloudWatch for deployment tracking.
	logrus.Infof("Starting RDS Tag Setter version=%s commit=%s built=%s",
		version.Version, version.GitCommit, version.BuildTime)

	// Setup JSON structured logging to stdout.
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Create AWS session using environment variables and IAM roles.
	sess := session.Must(session.NewSession())

	// Initialize handler with AWS clients and logger for Lambda business logic.
	handler := metrics.NewHandler(
		logger,
		rds.New(sess),
		sts.New(sess),
	)

	// Start Lambda handler - blocks until Lambda environment stops the process.
	lambda.Start(handler.HandleRequest)
}
