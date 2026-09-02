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

type minioStorage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
}

func newMinIO(cfg config.StorageConfig) (Storage, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("minio config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // required for MinIO
	})

	return &minioStorage{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.Bucket,
		publicBaseURL: cfg.PublicBaseURL,
	}, nil
}

func (m *minioStorage) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, time.Time, error) {
	req, err := m.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(m.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("presign put: %w", err)
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (m *minioStorage) HeadObject(ctx context.Context, key string) (int64, string, error) {
	resp, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
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

func (m *minioStorage) PublicURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s", m.publicBaseURL, key), nil
}

func (m *minioStorage) DeleteObject(ctx context.Context, key string) error {
	_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	return err
}
