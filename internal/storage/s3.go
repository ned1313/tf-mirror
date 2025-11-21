package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage implements Storage interface for AWS S3 or S3-compatible storage
type S3Storage struct {
	client         *s3.Client
	bucket         string
	presignClient  *s3.PresignClient
	region         string
	forcePathStyle bool
}

// S3Config contains configuration for S3 storage
type S3Config struct {
	Region         string
	Bucket         string
	Endpoint       string // Optional: for MinIO or custom S3 endpoints
	AccessKey      string // Optional: leave empty to use IAM role
	SecretKey      string // Optional: leave empty to use IAM role
	ForcePathStyle bool   // Required for MinIO compatibility
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	var awsCfg aws.Config
	var err error

	// Load AWS config based on authentication method
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		// Use access key/secret key authentication
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			)),
		)
	} else {
		// Use IAM role authentication (default)
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with optional custom endpoint
	var s3Client *s3.Client
	if cfg.Endpoint != "" {
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	} else {
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}

	presignClient := s3.NewPresignClient(s3Client)

	return &S3Storage{
		client:         s3Client,
		bucket:         cfg.Bucket,
		presignClient:  presignClient,
		region:         cfg.Region,
		forcePathStyle: cfg.ForcePathStyle,
	}, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	// Add metadata if provided
	if len(metadata) > 0 {
		input.Metadata = metadata
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload object %s: %w", key, err)
	}

	return nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download object %s: %w", key, err)
	}

	return result.Body, nil
}

// Delete removes a file from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", key, err)
	}

	return nil
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("key cannot be empty")
	}

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if the error is a "not found" error
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence %s: %w", key, err)
	}

	return true, nil
}

// GetPresignedURL generates a presigned URL for downloading
func (s *S3Storage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key cannot be empty")
	}

	request, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL for %s: %w", key, err)
	}

	return request.URL, nil
}

// GetMetadata retrieves metadata for an object
func (s *S3Storage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for %s: %w", key, err)
	}

	return result.Metadata, nil
}

// ListObjects lists objects with a given prefix
func (s *S3Storage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects with prefix %s: %w", prefix, err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

// GetObjectSize returns the size of an object in bytes
func (s *S3Storage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return 0, fmt.Errorf("key cannot be empty")
	}

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return 0, fmt.Errorf("failed to get object size for %s: %w", key, err)
	}

	if result.ContentLength == nil {
		return 0, nil
	}

	return *result.ContentLength, nil
}

// Close closes any open connections (S3 client doesn't require explicit closing)
func (s *S3Storage) Close() error {
	// AWS SDK v2 clients don't require explicit closing
	return nil
}
