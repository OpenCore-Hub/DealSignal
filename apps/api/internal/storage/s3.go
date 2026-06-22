package storage

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps an S3-compatible client (MinIO / AWS S3).
type Client struct {
	bucket          string
	client          *s3.Client
	presigner       *s3.PresignClient
	publicPresigner *s3.PresignClient
	endpoint        string
	pathStyle       bool
}

// NewS3Client creates a storage client from config.
func NewS3Client(cfg *config.Config) (*Client, error) {
	region := cfg.S3Region
	if region == "" {
		region = "us-east-1"
	}
	pathStyle := strings.ToLower(cfg.S3UsePathStyle) == "true" || cfg.S3UsePathStyle == "1"

	awsCfg, err := loadAWSConfig(region, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Endpoint)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := newS3Client(awsCfg, pathStyle)
	c := &Client{
		bucket:    cfg.S3Bucket,
		client:    client,
		presigner: s3.NewPresignClient(client),
		endpoint:  cfg.S3Endpoint,
		pathStyle: pathStyle,
	}

	if cfg.S3PublicEndpoint != "" {
		publicCfg, err := loadAWSConfig(region, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3PublicEndpoint)
		if err != nil {
			return nil, fmt.Errorf("load public aws config: %w", err)
		}
		publicClient := newS3Client(publicCfg, pathStyle)
		c.publicPresigner = s3.NewPresignClient(publicClient)
	}

	return c, nil
}

func loadAWSConfig(region, accessKey, secretKey, endpoint string) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	}
	if endpoint != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(endpoint))
	}
	return awsconfig.LoadDefaultConfig(context.Background(), opts...)
}

func newS3Client(awsCfg aws.Config, pathStyle bool) *s3.Client {
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = pathStyle
	})
}

// PutObject uploads an object to the configured bucket.
func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        c.bucketPtr(),
		Key:           c.keyPtr(key),
		Body:          body,
		ContentLength: &size,
		ContentType:   &contentType,
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// GetObject returns a reader for an object.
func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: c.bucketPtr(),
		Key:    c.keyPtr(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return out.Body, nil
}

// PresignedGetURL returns a temporary URL for reading an object.
func (c *Client) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presigner := c.presigner
	if c.publicPresigner != nil {
		presigner = c.publicPresigner
	}
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: c.bucketPtr(),
		Key:    c.keyPtr(key),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return req.URL, nil
}

// PresignedPutURL returns a temporary URL for uploading an object.
func (c *Client) PresignedPutURL(ctx context.Context, key string, expiry time.Duration, contentType string) (string, error) {
	presigner := c.presigner
	if c.publicPresigner != nil {
		presigner = c.publicPresigner
	}
	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      c.bucketPtr(),
		Key:         c.keyPtr(key),
		ContentType: &contentType,
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign put %s: %w", key, err)
	}
	return req.URL, nil
}

func (c *Client) bucketPtr() *string { return &c.bucket }
func (c *Client) keyPtr(key string) *string { return &key }

// ObjectKey builds a tenant/workspace-scoped object key.
func ObjectKey(tenantID, workspaceID, documentID, fileName string) string {
	return fmt.Sprintf("tenants/%s/workspaces/%s/documents/%s/%s", tenantID, workspaceID, documentID, fileName)
}

// ParseBool is a small helper for environment booleans.
func ParseBool(s string) bool {
	b, _ := strconv.ParseBool(strings.ToLower(s))
	return b
}
