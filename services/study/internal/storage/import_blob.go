// Package storage buffers async import uploads (MinIO or local spool).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ImportBlobStorage stores uploaded import files for background workers.
type ImportBlobStorage interface {
	// Put streams content to storage and returns the byte size written.
	Put(ctx context.Context, key string, r io.Reader) (int64, error)
	// Open returns a read stream for an object previously stored with Put.
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the object after processing (best-effort cleanup).
	Delete(ctx context.Context, key string) error
}

// MinIOConfig configures S3-compatible import blob storage.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// NewMinIOImportStorage connects to MinIO (or any S3-compatible endpoint).
func NewMinIOImportStorage(cfg MinIOConfig) (ImportBlobStorage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("minio endpoint and bucket are required")
	}
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
		o.UsePathStyle = true
	})
	return &minioImportStorage{client: client, bucket: cfg.Bucket}, nil
}

type minioImportStorage struct {
	client *s3.Client
	bucket string
}

func (m *minioImportStorage) PutAudio(ctx context.Context, key string, data []byte, mime string) error {
	_, err := m.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(m.bucket), Key: aws.String(key), Body: bytes.NewReader(data),
		ContentType: aws.String(mime), CacheControl: aws.String("private, no-store"),
	})
	return err
}

func (m *minioImportStorage) Put(ctx context.Context, key string, r io.Reader) (int64, error) {
	_, err := m.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(m.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
	})
	if err != nil {
		return 0, fmt.Errorf("put object: %w", err)
	}
	head, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("head object: %w", err)
	}
	if head.ContentLength != nil {
		return *head.ContentLength, nil
	}
	return 0, nil
}

func (m *minioImportStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	return out.Body, nil
}

func (m *minioImportStorage) Delete(ctx context.Context, key string) error {
	_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	return err
}

// LocalSpoolStorage writes imports to a directory (dev fallback without MinIO).
type LocalSpoolStorage struct {
	root string
}

// NewLocalSpoolStorage creates a disk-backed spool under root.
func NewLocalSpoolStorage(root string) (*LocalSpoolStorage, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &LocalSpoolStorage{root: root}, nil
}

func (l *LocalSpoolStorage) path(key string) string {
	return filepath.Join(l.root, filepath.Clean(filepath.FromSlash(key)))
}

func (l *LocalSpoolStorage) Put(_ context.Context, key string, r io.Reader) (int64, error) {
	path := l.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	return n, err
}

func (l *LocalSpoolStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(l.path(key))
}

func (l *LocalSpoolStorage) Delete(_ context.Context, key string) error {
	err := os.Remove(l.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
