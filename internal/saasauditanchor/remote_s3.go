package saasauditanchor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultRemoteRetentionDays = 3650

var (
	remotePrefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.=-]{0,254}$`)
	artifactNamePattern = regexp.MustCompile(`^AAN-[A-F0-9]{24}\.json$`)
)

type S3RemoteArtifactConfig struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UseSSL          bool
	Prefix          string
	RetentionMode   string
	RetentionDays   int
}

type s3RemoteArtifactStore struct {
	client        *minio.Client
	bucket        string
	prefix        string
	retentionMode minio.RetentionMode
	retentionDays int
}

func NewS3RemoteArtifactStore(config S3RemoteArtifactConfig) (RemoteArtifactStore, error) {
	endpoint, secure, err := normalizeRemoteS3Endpoint(config.Endpoint, config.UseSSL)
	if err != nil {
		return nil, err
	}
	bucket := strings.TrimSpace(config.Bucket)
	accessKeyID := strings.TrimSpace(config.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.SecretAccessKey)
	if endpoint == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, Invalid("审计锚点 S3 远端证据需要同时配置 endpoint、bucket、access key 和 secret key")
	}
	prefix := strings.Trim(strings.TrimSpace(config.Prefix), "/")
	if prefix == "" {
		prefix = "mochat-go/audit-anchors"
	}
	if !remotePrefixPattern.MatchString(prefix) || strings.Contains(prefix, "//") {
		return nil, Invalid("审计锚点 S3 对象前缀无效")
	}
	for _, segment := range strings.Split(prefix, "/") {
		if segment == "." || segment == ".." {
			return nil, Invalid("审计锚点 S3 对象前缀不能包含相对路径段")
		}
	}
	mode := minio.Compliance
	switch strings.ToLower(strings.TrimSpace(config.RetentionMode)) {
	case "", "compliance":
	case "governance":
		mode = minio.Governance
	default:
		return nil, Invalid("审计锚点 S3 留存模式必须是 compliance 或 governance")
	}
	retentionDays := config.RetentionDays
	if retentionDays == 0 {
		retentionDays = defaultRemoteRetentionDays
	}
	if retentionDays < 1 || retentionDays > 36500 {
		return nil, Invalid("审计锚点 S3 留存天数必须在 1 至 36500 之间")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, strings.TrimSpace(config.SessionToken)),
		Secure: secure,
		Region: strings.TrimSpace(config.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("build audit anchor S3 client: %w", err)
	}
	return &s3RemoteArtifactStore{
		client: client, bucket: bucket, prefix: prefix, retentionMode: mode, retentionDays: retentionDays,
	}, nil
}

func normalizeRemoteS3Endpoint(raw string, defaultSecure bool) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", defaultSecure, nil
	}
	if !strings.Contains(raw, "://") {
		if strings.ContainsAny(raw, "/?#") {
			return "", false, Invalid("审计锚点 S3 endpoint 只能包含主机和端口")
		}
		return raw, defaultSecure, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false, Invalid("审计锚点 S3 endpoint URL 无效")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http":
		return parsed.Host, false, nil
	case "https":
		return parsed.Host, true, nil
	default:
		return "", false, Invalid("审计锚点 S3 endpoint 只支持 http 或 https")
	}
}

func (s *s3RemoteArtifactStore) Provider() string { return "s3-object-lock" }
func (s *s3RemoteArtifactStore) Bucket() string   { return s.bucket }
func (s *s3RemoteArtifactStore) Prefix() string   { return s.prefix }
func (s *s3RemoteArtifactStore) RetentionMode() string {
	return strings.ToLower(s.retentionMode.String())
}
func (s *s3RemoteArtifactStore) RetentionDays() int { return s.retentionDays }

func (s *s3RemoteArtifactStore) Probe(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("probe audit anchor S3 bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("audit anchor S3 bucket %q does not exist", s.bucket)
	}
	enabled, _, _, _, err := s.client.GetObjectLockConfig(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("read audit anchor S3 object lock configuration: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(enabled), "Enabled") {
		return fmt.Errorf("audit anchor S3 bucket %q must enable Object Lock", s.bucket)
	}
	return nil
}

func (s *s3RemoteArtifactStore) Ensure(
	ctx context.Context,
	artifactName string,
	content []byte,
	expectedSHA string,
	signedAt time.Time,
	existing RemoteArtifact,
) (RemoteArtifact, error) {
	if !artifactNamePattern.MatchString(strings.TrimSpace(artifactName)) {
		return RemoteArtifact{}, Invalid("审计锚点远端证据文件名无效")
	}
	if signedAt.IsZero() {
		signedAt = time.Now().UTC()
	}
	objectKey := strings.TrimSpace(existing.ObjectKey)
	if objectKey == "" {
		objectKey = path.Join(s.prefix, signedAt.UTC().Format("2006/01/02"), artifactName)
	}
	candidate := existing
	candidate.Provider = s.Provider()
	candidate.Bucket = s.bucket
	candidate.ObjectKey = objectKey
	if verified, err := s.Verify(ctx, candidate, content, expectedSHA); err == nil {
		return verified, nil
	} else if !errors.Is(err, ErrRemoteArtifactNotFound) {
		return RemoteArtifact{}, err
	}
	retainUntil := time.Now().UTC().Add(time.Duration(s.retentionDays) * 24 * time.Hour).Truncate(time.Second)
	options := minio.PutObjectOptions{
		ContentType:     "application/json",
		Mode:            s.retentionMode,
		RetainUntilDate: retainUntil,
		UserMetadata: map[string]string{
			"mochat-sha256": strings.ToLower(strings.TrimSpace(expectedSHA)),
			"mochat-schema": SchemaVersion,
		},
	}
	options.SetMatchETagExcept("*")
	upload, err := s.client.PutObject(ctx, s.bucket, objectKey, bytes.NewReader(content), int64(len(content)), options)
	if err != nil {
		if verified, verifyErr := s.Verify(ctx, candidate, content, expectedSHA); verifyErr == nil {
			return verified, nil
		}
		return RemoteArtifact{}, fmt.Errorf("upload immutable audit anchor S3 artifact: %w", err)
	}
	candidate.ETag = strings.Trim(upload.ETag, `"`)
	candidate.VersionID = upload.VersionID
	candidate.SHA256 = strings.ToLower(strings.TrimSpace(expectedSHA))
	candidate.SizeBytes = upload.Size
	candidate.RetentionMode = s.RetentionMode()
	candidate.RetainUntil = retainUntil.Format(time.RFC3339)
	candidate.ExportedAt = time.Now().UTC().Truncate(time.Second)
	return s.Verify(ctx, candidate, content, expectedSHA)
}

func (s *s3RemoteArtifactStore) Verify(
	ctx context.Context,
	artifact RemoteArtifact,
	expectedContent []byte,
	expectedSHA string,
) (RemoteArtifact, error) {
	objectKey := strings.TrimSpace(artifact.ObjectKey)
	if objectKey == "" {
		return RemoteArtifact{}, ErrRemoteArtifactNotFound
	}
	statOptions := minio.StatObjectOptions{VersionID: strings.TrimSpace(artifact.VersionID)}
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, statOptions)
	if err != nil {
		if isRemoteObjectNotFound(err) {
			return RemoteArtifact{}, ErrRemoteArtifactNotFound
		}
		return RemoteArtifact{}, fmt.Errorf("stat immutable audit anchor S3 artifact: %w", err)
	}
	getOptions := minio.GetObjectOptions{VersionID: strings.TrimSpace(artifact.VersionID)}
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, getOptions)
	if err != nil {
		if isRemoteObjectNotFound(err) {
			return RemoteArtifact{}, ErrRemoteArtifactNotFound
		}
		return RemoteArtifact{}, fmt.Errorf("open immutable audit anchor S3 artifact: %w", err)
	}
	actual, readErr := io.ReadAll(object)
	closeErr := object.Close()
	if readErr != nil {
		return RemoteArtifact{}, fmt.Errorf("read immutable audit anchor S3 artifact: %w", readErr)
	}
	if closeErr != nil {
		return RemoteArtifact{}, fmt.Errorf("close immutable audit anchor S3 artifact: %w", closeErr)
	}
	hash := sha256.Sum256(actual)
	actualSHA := hex.EncodeToString(hash[:])
	if info.Size != int64(len(expectedContent)) || !strings.EqualFold(actualSHA, expectedSHA) || !bytes.Equal(actual, expectedContent) {
		return RemoteArtifact{}, errors.New("审计锚点 S3 远端证据内容或摘要不匹配")
	}
	versionID := strings.TrimSpace(artifact.VersionID)
	if versionID == "" {
		versionID = info.VersionID
	}
	mode, retainUntil, err := s.client.GetObjectRetention(ctx, s.bucket, objectKey, versionID)
	if err != nil {
		return RemoteArtifact{}, fmt.Errorf("read immutable audit anchor S3 retention: %w", err)
	}
	if mode == nil || retainUntil == nil || !strings.EqualFold(mode.String(), s.retentionMode.String()) {
		return RemoteArtifact{}, errors.New("审计锚点 S3 远端证据未按配置启用 Object Lock 留存")
	}
	if !retainUntil.After(time.Now().UTC()) {
		return RemoteArtifact{}, errors.New("审计锚点 S3 远端证据 Object Lock 留存已过期")
	}
	verified := RemoteArtifact{
		Provider: s.Provider(), Bucket: s.bucket, ObjectKey: objectKey,
		ETag: strings.Trim(info.ETag, `"`), VersionID: versionID,
		SHA256: actualSHA, SizeBytes: info.Size,
		RetentionMode: strings.ToLower(mode.String()), RetainUntil: retainUntil.UTC().Truncate(time.Second).Format(time.RFC3339),
		ExportedAt: artifact.ExportedAt, VerifiedAt: time.Now().UTC().Truncate(time.Second),
	}
	if verified.ExportedAt.IsZero() {
		verified.ExportedAt = verified.VerifiedAt
	}
	return verified, nil
}

func (s *s3RemoteArtifactStore) ListObjectKeys(ctx context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	options := minio.ListObjectsOptions{Prefix: strings.TrimSuffix(s.prefix, "/") + "/", Recursive: true, WithVersions: true}
	for item := range s.client.ListObjects(ctx, s.bucket, options) {
		if item.Err != nil {
			return nil, fmt.Errorf("list immutable audit anchor S3 artifacts: %w", item.Err)
		}
		name := path.Base(item.Key)
		if artifactNamePattern.MatchString(name) {
			seen[item.Key] = struct{}{}
		}
	}
	items := make([]string, 0, len(seen))
	for item := range seen {
		items = append(items, item)
	}
	sort.Strings(items)
	return items, nil
}

func isRemoteObjectNotFound(err error) bool {
	response := minio.ToErrorResponse(err)
	switch response.Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchVersion", "NotFound", "XMinioInvalidObjectName":
		return true
	default:
		return false
	}
}
