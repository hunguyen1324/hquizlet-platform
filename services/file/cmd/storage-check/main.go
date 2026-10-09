// storage-check verifies configured storage credentials without modifying objects.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Storage.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Storage.AccessKeyID, cfg.Storage.SecretAccessKey, "")),
	)
	if err == nil {
		client := s3.NewFromConfig(c, func(o *s3.Options) {
			o.UsePathStyle = cfg.Storage.PathStyle
			if cfg.Storage.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.Storage.Endpoint)
			}
		})
		_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Storage.Bucket)})
	}
	if err != nil {
		// SDK errors can contain request URLs; keep credentials and endpoint out of logs.
		fmt.Fprintln(os.Stderr, "Storage check failed: verify network, bucket name and credential permissions.")
		os.Exit(1)
	}
	fmt.Printf("Storage connection OK (provider=%s). No objects modified.\n", cfg.Storage.Provider)
}
