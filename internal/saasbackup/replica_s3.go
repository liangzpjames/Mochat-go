package saasbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const s3ReplicaProvider = "s3"

var s3PrefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.=-]{0,254}$`)

type S3ReplicaConfig struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UseSSL          bool
	Prefix          string
}

type s3ReplicaStore struct {
	client *minio.Client
	bucket string
	prefix string
}

func NewS3ReplicaStore(config S3ReplicaConfig) (ReplicaStore, error) {
	endpoint, secure, err := normalizeS3Endpoint(config.Endpoint, config.UseSSL)
	if err != nil {
		return nil, err
	}
	bucket := strings.TrimSpace(config.Bucket)
	accessKeyID := strings.TrimSpace(config.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.SecretAccessKey)
	if endpoint == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, Invalid("S3 异地副本需要同时配置 endpoint、bucket、access key 和 secret key")
	}
	prefix := strings.Trim(strings.TrimSpace(config.Prefix), "/")
	if prefix == "" {
		prefix = "mochat-go/backups"
	}
	if !s3PrefixPattern.MatchString(prefix) || strings.Contains(prefix, "//") {
		return nil, Invalid("S3 异地副本对象前缀无效")
	}
	for _, segment := range strings.Split(prefix, "/") {
		if segment == "." || segment == ".." {
			return nil, Invalid("S3 异地副本对象前缀不能包含相对路径段")
		}
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, strings.TrimSpace(config.SessionToken)),
		Secure: secure,
		Region: strings.TrimSpace(config.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("build S3 replica client: %w", err)
	}
	return &s3ReplicaStore{client: client, bucket: bucket, prefix: prefix}, nil
}

func normalizeS3Endpoint(raw string, defaultSecure bool) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", defaultSecure, nil
	}
	if !strings.Contains(raw, "://") {
		if strings.ContainsAny(raw, "/?#") {
			return "", false, Invalid("S3 endpoint 只能包含主机和端口")
		}
		return raw, defaultSecure, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false, Invalid("S3 endpoint URL 无效")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http":
		return parsed.Host, false, nil
	case "https":
		return parsed.Host, true, nil
	default:
		return "", false, Invalid("S3 endpoint 只支持 http 或 https")
	}
}

func (s *s3ReplicaStore) Provider() string { return s3ReplicaProvider }
func (s *s3ReplicaStore) Bucket() string   { return s.bucket }

func (s *s3ReplicaStore) ObjectKey(artifactName string, createdAt time.Time) (string, error) {
	if _, err := safeArtifactPath(".", artifactName); err != nil {
		return "", err
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	createdAt = createdAt.UTC()
	return path.Join(s.prefix, createdAt.Format("2006/01/02"), artifactName), nil
}

func (s *s3ReplicaStore) Probe(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("probe S3 replica bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("S3 replica bucket %q does not exist", s.bucket)
	}
	return nil
}

func (s *s3ReplicaStore) Put(ctx context.Context, objectKey, localPath, expectedSHA string, expectedSize int64) (ReplicaObject, error) {
	info, err := os.Lstat(localPath)
	if err != nil {
		return ReplicaObject{}, fmt.Errorf("stat local backup before S3 upload: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != expectedSize {
		return ReplicaObject{}, fmt.Errorf("local backup changed before S3 upload")
	}
	upload, err := s.client.FPutObject(ctx, s.bucket, objectKey, localPath, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
		UserMetadata: map[string]string{
			"mochat-sha256": strings.ToLower(strings.TrimSpace(expectedSHA)),
		},
	})
	if err != nil {
		return ReplicaObject{}, fmt.Errorf("upload S3 backup replica: %w", err)
	}
	return ReplicaObject{
		Provider: s.Provider(), Bucket: s.bucket, ObjectKey: objectKey, ETag: strings.Trim(upload.ETag, `"`),
		VersionID: upload.VersionID, SHA256: strings.ToLower(expectedSHA), SizeBytes: upload.Size,
	}, nil
}

func (s *s3ReplicaStore) Verify(ctx context.Context, objectKey, versionID, expectedSHA string, expectedSize int64) (ReplicaObject, error) {
	metadata, reader, err := s.Open(ctx, objectKey, versionID)
	if err != nil {
		return ReplicaObject{}, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(hash, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return ReplicaObject{}, fmt.Errorf("read S3 backup replica: %w", copyErr)
	}
	if closeErr != nil {
		return ReplicaObject{}, fmt.Errorf("close S3 backup replica: %w", closeErr)
	}
	actualSHA := hex.EncodeToString(hash.Sum(nil))
	if size != expectedSize || metadata.SizeBytes != expectedSize || !strings.EqualFold(actualSHA, expectedSHA) {
		return ReplicaObject{}, fmt.Errorf("S3 backup replica checksum mismatch")
	}
	metadata.SHA256 = actualSHA
	return metadata, nil
}

func (s *s3ReplicaStore) Open(ctx context.Context, objectKey, versionID string) (ReplicaObject, io.ReadCloser, error) {
	statOptions := minio.StatObjectOptions{VersionID: strings.TrimSpace(versionID)}
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, statOptions)
	if err != nil {
		return ReplicaObject{}, nil, fmt.Errorf("stat S3 backup replica: %w", err)
	}
	getOptions := minio.GetObjectOptions{VersionID: strings.TrimSpace(versionID)}
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, getOptions)
	if err != nil {
		return ReplicaObject{}, nil, fmt.Errorf("open S3 backup replica: %w", err)
	}
	return ReplicaObject{
		Provider: s.Provider(), Bucket: s.bucket, ObjectKey: objectKey, ETag: strings.Trim(info.ETag, `"`),
		VersionID: info.VersionID, SizeBytes: info.Size,
	}, object, nil
}

func (s *s3ReplicaStore) Delete(ctx context.Context, objectKey, versionID string) error {
	versionID = strings.TrimSpace(versionID)
	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{VersionID: versionID})
	if err != nil && !isS3ReplicaNotFound(err) {
		return fmt.Errorf("delete S3 backup replica: %w", err)
	}
	_, err = s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{VersionID: versionID})
	if isS3ReplicaNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("verify deleted S3 backup replica: %w", err)
	}
	return fmt.Errorf("S3 backup replica still exists after delete")
}

func isS3ReplicaNotFound(err error) bool {
	if err == nil {
		return false
	}
	response := minio.ToErrorResponse(err)
	switch response.Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchVersion", "NotFound", "XMinioInvalidObjectName":
		return true
	default:
		return false
	}
}
