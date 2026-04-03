package stsclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"

	"has-smartlock-service/internal/pkg/config"
)

var (
	ErrNotConfigured = errors.New("sts token service not configured")
	sessionNameRE    = regexp.MustCompile(`[^a-zA-Z0-9@._-]`)
)

type TokenResult struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      int64
	RegionID        string
	Endpoint        string
	Bucket          string
}

type Client struct {
	appEnv          string
	accessKeyID     string
	accessKeySecret string
	roleARN         string
	sessionPrefix   string
	duration        int64
	endpoint        string
	bucketName      string
	regionID        string
	avatarPrefix    string
	stsClient       *sts.Client
}

func New(cfg config.Config) (*Client, error) {
	client := &Client{
		appEnv:          cfg.AppEnv,
		accessKeyID:     strings.TrimSpace(cfg.OSSAccessKeyID),
		accessKeySecret: strings.TrimSpace(cfg.OSSAccessKeySecret),
		roleARN:         strings.TrimSpace(cfg.OSSSTSRoleARN),
		sessionPrefix:   strings.TrimSpace(cfg.OSSSTSSessionPrefix),
		duration:        int64(cfg.OSSSTSDuration),
		endpoint:        strings.TrimSpace(cfg.OSSEndpoint),
		bucketName:      strings.TrimSpace(cfg.OSSBucketName),
		avatarPrefix:    strings.Trim(strings.TrimSpace(cfg.OSSAvatarPrefix), "/"),
	}
	if client.avatarPrefix == "" {
		client.avatarPrefix = "avatar"
	}
	if client.sessionPrefix == "" {
		client.sessionPrefix = "has-smartlock-avatar"
	}
	if client.duration <= 0 {
		client.duration = 900
	}

	if client.endpoint == "" {
		return client, nil
	}

	regionID, err := regionFromOSSEndpoint(client.endpoint)
	if err != nil {
		return nil, err
	}
	client.regionID = regionID

	if client.appEnv == "test" {
		return client, nil
	}

	if client.accessKeyID == "" || client.accessKeySecret == "" || client.roleARN == "" || client.bucketName == "" {
		return client, nil
	}

	stsConfig := &openapi.Config{
		AccessKeyId:     tea.String(client.accessKeyID),
		AccessKeySecret: tea.String(client.accessKeySecret),
		Endpoint:        tea.String("sts." + client.regionID + ".aliyuncs.com"),
	}
	stsClient, err := sts.NewClient(stsConfig)
	if err != nil {
		return nil, err
	}
	client.stsClient = stsClient

	return client, nil
}

func (c *Client) AvatarObjectKey(uid string) string {
	uid = strings.Trim(strings.TrimSpace(uid), "/")
	if uid == "" {
		return c.avatarPrefix
	}
	return c.avatarPrefix + "/" + uid
}

func (c *Client) GetAvatarToken(uid string) (*TokenResult, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, errors.New("uid required")
	}
	if c.bucketName == "" || c.endpoint == "" || c.regionID == "" {
		return nil, ErrNotConfigured
	}

	if c.appEnv == "test" {
		now := time.Now().Add(time.Duration(c.duration) * time.Second)
		return &TokenResult{
			AccessKeyID:     "test-sts-ak",
			AccessKeySecret: "test-sts-sk",
			SecurityToken:   "test-security-token",
			Expiration:      now.Unix(),
			RegionID:        c.regionID,
			Endpoint:        c.endpoint,
			Bucket:          c.bucketName,
		}, nil
	}

	if c.stsClient == nil {
		return nil, ErrNotConfigured
	}

	policy, err := c.avatarPolicy(uid)
	if err != nil {
		return nil, err
	}

	request := &sts.AssumeRoleRequest{
		RoleArn:         tea.String(c.roleARN),
		RoleSessionName: tea.String(c.sessionName(uid)),
		DurationSeconds: tea.Int64(c.duration),
		Policy:          tea.String(policy),
	}
	runtime := &util.RuntimeOptions{}
	response, err := c.stsClient.AssumeRoleWithOptions(request, runtime)
	if err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil || response.Body.Credentials == nil {
		return nil, errors.New("empty sts response")
	}

	expiration, err := parseExpiration(tea.StringValue(response.Body.Credentials.Expiration))
	if err != nil {
		return nil, err
	}

	return &TokenResult{
		AccessKeyID:     tea.StringValue(response.Body.Credentials.AccessKeyId),
		AccessKeySecret: tea.StringValue(response.Body.Credentials.AccessKeySecret),
		SecurityToken:   tea.StringValue(response.Body.Credentials.SecurityToken),
		Expiration:      expiration.Unix(),
		RegionID:        c.regionID,
		Endpoint:        c.endpoint,
		Bucket:          c.bucketName,
	}, nil
}

func (c *Client) avatarPolicy(uid string) (string, error) {
	objectKey := c.AvatarObjectKey(uid)
	statement := map[string]any{
		"Version": "1",
		"Statement": []map[string]any{
			{
				"Effect":   "Allow",
				"Action":   []string{"oss:PutObject", "oss:GetObject"},
				"Resource": []string{fmt.Sprintf("acs:oss:*:*:%s/%s", c.bucketName, objectKey)},
			},
		},
	}

	body, err := json.Marshal(statement)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) sessionName(uid string) string {
	uid = sessionNameRE.ReplaceAllString(uid, "-")
	if uid == "" {
		uid = "anonymous"
	}
	return c.sessionPrefix + "-" + uid
}

func regionFromOSSEndpoint(endpoint string) (string, error) {
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

func parseExpiration(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid sts expiration: %s", value)
}
