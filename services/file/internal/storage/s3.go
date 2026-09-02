package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/config"
)

type s3Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
	endpoint      string
}

func newS3(cfg config.StorageConfig) (Storage, error) {
	// For S3, endpoint can be empty (uses default AWS endpoint).
	// For R2, endpoint is https://<account_id>.r2.cloudflarestorage.com
	var opts []func(*awsconfig.LoadOptions) error
	opts = append(opts, awsconfig.WithRegion(cfg.Region))
	opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID, cfg.SecretAccessKey, "",
	)))

	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
			},
		)
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle
	})

	return &s3Storage{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.Bucket,
		publicBaseURL: cfg.PublicBaseURL,
		endpoint:      cfg.Endpoint,
	}, nil
}

func (s *s3Storage) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, time.Time, error) {
	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign put: %w", err)
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (s *s3Storage) HeadObject(ctx context.Context, key string) (int64, string, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, "", fmt.Errorf("head object: %w", err)
	}
	ct := ""
	if resp.ContentType != nil {
		ct = *resp.ContentType
	}
	size := int64(0)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	return size, ct, nil
}

func (s *s3Storage) PublicURL(_ context.Context, key string, _ time.Duration) (string, error) {
	if s.publicBaseURL != "" {
		return fmt.Sprintf("%s/%s", s.publicBaseURL, key), nil
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, "us-east-1", key), nil
}

func (s *s3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
