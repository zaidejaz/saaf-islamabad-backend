package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Config holds Cloudflare R2 credentials and bucket settings.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicBaseURL   string
}

// R2Storage uploads objects to a Cloudflare R2 bucket (S3-compatible API).
type R2Storage struct {
	client       *s3.Client
	bucket       string
	publicBaseURL string
}

func NewR2(cfg R2Config) (*R2Storage, error) {
	accountID := strings.TrimSpace(cfg.AccountID)
	accessKey := strings.TrimSpace(cfg.AccessKeyID)
	secretKey := strings.TrimSpace(cfg.SecretAccessKey)
	bucket := strings.TrimSpace(cfg.Bucket)
	publicBase := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")

	if accountID == "" || accessKey == "" || secretKey == "" || bucket == "" || publicBase == "" {
		return nil, fmt.Errorf("R2 storage requires R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET, and R2_PUBLIC_BASE_URL")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		UsePathStyle: true,
	})

	return &R2Storage{
		client:        client,
		bucket:        bucket,
		publicBaseURL: publicBase,
	}, nil
}

func (s *R2Storage) Driver() string { return DriverR2 }

func (s *R2Storage) Save(key string, body io.Reader, contentType string, size int64) (string, error) {
	key = strings.TrimPrefix(strings.ReplaceAll(key, "\\", "/"), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid storage key")
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	if _, err := s.client.PutObject(context.Background(), input); err != nil {
		return "", fmt.Errorf("upload to R2: %w", err)
	}

	return s.publicBaseURL + "/" + key, nil
}
