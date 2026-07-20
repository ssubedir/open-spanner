package fileexport

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func TestS3CompatibleIntegration(t *testing.T) {
	endpoint := os.Getenv("OPEN_SPANNER_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set OPEN_SPANNER_TEST_S3_ENDPOINT to run S3-compatible integration tests")
	}
	accessKey := envOr("OPEN_SPANNER_TEST_S3_ACCESS_KEY_ID", "minioadmin")
	secretKey := envOr("OPEN_SPANNER_TEST_S3_SECRET_ACCESS_KEY", "minioadmin")
	region := envOr("OPEN_SPANNER_TEST_S3_REGION", "us-east-1")
	bucket := "open-spanner-test-" + uuid.NewString()
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	if err != nil {
		t.Fatal(err)
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) { options.BaseEndpoint = aws.String(endpoint); options.UsePathStyle = true })
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}
	t.Cleanup(func() {
		if _, err := client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Errorf("delete test bucket: %v", err)
		}
	})
	store, err := NewS3Store(ctx, Options{S3Bucket: bucket, S3Region: region, S3Endpoint: endpoint, S3AccessKeyID: accessKey, S3SecretAccessKey: secretKey, S3Prefix: "integration", S3ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	testStoreContract(t, store)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
