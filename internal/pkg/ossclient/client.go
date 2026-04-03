package ossclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	"has-smartlock-service/internal/pkg/config"
)

var ErrNotConfigured = errors.New("oss avatar upload not configured")

type PrepareUploadResult struct {
	ObjectKey string
	UploadURL string
	Bucket    string
	ExpireAt  time.Time
}

type Client struct {
	bucketName string
	baseURL    string
	prefix     string
	uploadTTL  time.Duration
	readTTL    time.Duration
	signer     *alioss.Client
}

func New(cfg config.Config) (*Client, error) {
	client := &Client{
		bucketName: cfg.OSSBucketName,
		baseURL:    strings.TrimRight(cfg.OSSPublicBaseURL, "/"),
		prefix:     strings.Trim(strings.TrimSpace(cfg.OSSAvatarPrefix), "/"),
		uploadTTL:  time.Duration(cfg.OSSUploadURLTTL) * time.Second,
		readTTL:    time.Duration(cfg.OSSSignedReadURLTTL) * time.Second,
	}
	if client.prefix == "" {
		client.prefix = "avatar"
	}
	if client.uploadTTL <= 0 {
		client.uploadTTL = 15 * time.Minute
	}
	if client.readTTL <= 0 {
		client.readTTL = 15 * time.Minute
	}

	if cfg.OSSEndpoint == "" || cfg.OSSBucketName == "" || cfg.OSSAccessKeyID == "" || cfg.OSSAccessKeySecret == "" {
		return client, nil
	}

	region, err := regionFromEndpoint(cfg.OSSEndpoint)
	if err != nil {
		return nil, err
	}

	ossCfg := alioss.LoadDefaultConfig().
		WithRegion(region).
		WithEndpoint(strings.TrimSpace(cfg.OSSEndpoint)).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret))

	client.signer = alioss.NewClient(ossCfg)
	return client, nil
}

func (c *Client) PublicURL(objectKey string) string {
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return ""
	}
	if c.baseURL == "" {
		return objectKey
	}
	return c.baseURL + "/" + objectKey
}

func (c *Client) SignedReadURL(ctx context.Context, objectKey string) (string, error) {
	if c.signer == nil || c.bucketName == "" {
		return "", ErrNotConfigured
	}

	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return "", nil
	}

	request := &alioss.GetObjectRequest{
		Bucket: alioss.Ptr(c.bucketName),
		Key:    alioss.Ptr(objectKey),
	}

	result, err := c.signer.Presign(ctx, request, alioss.PresignExpires(c.readTTL))
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func (c *Client) PrepareAvatarUpload(ctx context.Context, uid string) (*PrepareUploadResult, error) {
	if c.signer == nil || c.bucketName == "" {
		return nil, ErrNotConfigured
	}

	objectKey := c.AvatarObjectKey(uid)
	request := &alioss.PutObjectRequest{
		Bucket: alioss.Ptr(c.bucketName),
		Key:    alioss.Ptr(objectKey),
	}

	result, err := c.signer.Presign(ctx, request, alioss.PresignExpires(c.uploadTTL))
	if err != nil {
		return nil, err
	}

	return &PrepareUploadResult{
		ObjectKey: objectKey,
		UploadURL: result.URL,
		Bucket:    c.bucketName,
		ExpireAt:  result.Expiration,
	}, nil
}

func (c *Client) AvatarObjectKey(uid string) string {
	uid = strings.Trim(strings.TrimSpace(uid), "/")
	if uid == "" {
		return c.prefix
	}
	return c.prefix + "/" + uid
}

func regionFromEndpoint(endpoint string) (string, error) {
	normalized := strings.TrimSpace(endpoint)
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimSuffix(normalized, "/")
	if !strings.HasPrefix(normalized, "oss-") || !strings.HasSuffix(normalized, ".aliyuncs.com") {
		return "", fmt.Errorf("invalid oss endpoint: %s", endpoint)
	}

	region := strings.TrimPrefix(normalized, "oss-")
	region = strings.TrimSuffix(region, ".aliyuncs.com")
	if region == "" {
		return "", fmt.Errorf("invalid oss endpoint: %s", endpoint)
	}
	return region, nil
}
