package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/outboundhttp"
)

const (
	defaultSaaSReleaseEvidenceVerifyTimeout = 30 * time.Second
	defaultSaaSReleaseEvidenceMaxBytes      = int64(64 << 20)
	maxSaaSReleaseVerificationTextLength    = 500
)

type SaaSReleaseArtifactVerification struct {
	EvidenceKey       string `json:"evidenceKey"`
	EvidenceVersion   int    `json:"evidenceVersion"`
	EvidenceURL       string `json:"evidenceUrl"`
	ExpectedSHA256    string `json:"expectedSha256"`
	ExpectedSizeBytes int64  `json:"expectedSizeBytes"`
	Verified          bool   `json:"verified"`
	AttemptedAt       string `json:"attemptedAt"`
	ActualSHA256      string `json:"actualSha256"`
	ActualSizeBytes   int64  `json:"actualSizeBytes"`
	HTTPStatus        int    `json:"httpStatus"`
	ContentType       string `json:"contentType"`
	ETag              string `json:"etag"`
	LastModified      string `json:"lastModified"`
	Error             string `json:"error"`
}

type SaaSReleaseEvidenceVerifierStatus struct {
	Configured               bool  `json:"configured"`
	VerifyOnPass             bool  `json:"verifyOnPass"`
	VerifyOnCandidateGate    bool  `json:"verifyOnCandidateGate"`
	TimeoutSeconds           int64 `json:"timeoutSeconds"`
	MaxBytes                 int64 `json:"maxBytes"`
	RequireHTTPS             bool  `json:"requireHttps"`
	PrivateNetworksBlocked   bool  `json:"privateNetworksBlocked"`
	MetadataAddressesBlocked bool  `json:"metadataAddressesBlocked"`
	DNSPinningEnabled        bool  `json:"dnsPinningEnabled"`
	SameOriginRedirectsOnly  bool  `json:"sameOriginRedirectsOnly"`
	EnvironmentProxyDisabled bool  `json:"environmentProxyDisabled"`
	CustomRootCAsConfigured  bool  `json:"customRootCasConfigured"`
	ExplicitAllowedCIDRCount int   `json:"explicitAllowedCidrCount"`
}

type SaaSReleaseArtifactVerifier interface {
	Verify(context.Context, string, string, int64) (SaaSReleaseArtifactVerification, error)
	Status() SaaSReleaseEvidenceVerifierStatus
}

type httpSaaSReleaseArtifactVerifier struct {
	client   *http.Client
	timeout  time.Duration
	maxBytes int64
	status   SaaSReleaseEvidenceVerifierStatus
}

func NewSaaSReleaseArtifactVerifier(guard *outboundhttp.Guard, timeout time.Duration, maxBytes int64) (SaaSReleaseArtifactVerifier, error) {
	if guard == nil {
		return nil, errors.New("release evidence outbound guard is required")
	}
	if timeout <= 0 {
		timeout = defaultSaaSReleaseEvidenceVerifyTimeout
	}
	if maxBytes <= 0 {
		maxBytes = defaultSaaSReleaseEvidenceMaxBytes
	}
	security := guard.Status()
	return &httpSaaSReleaseArtifactVerifier{
		client:   guard.NewClient(),
		timeout:  timeout,
		maxBytes: maxBytes,
		status: SaaSReleaseEvidenceVerifierStatus{
			Configured: true, VerifyOnPass: true, VerifyOnCandidateGate: true,
			TimeoutSeconds: int64(timeout / time.Second), MaxBytes: maxBytes,
			RequireHTTPS: security.RequireHTTPS, PrivateNetworksBlocked: security.PrivateNetworksBlocked,
			MetadataAddressesBlocked: security.MetadataAddressesBlocked, DNSPinningEnabled: security.DNSPinningEnabled,
			SameOriginRedirectsOnly: security.SameOriginRedirectsOnly, EnvironmentProxyDisabled: security.EnvironmentProxyDisabled,
			CustomRootCAsConfigured:  security.CustomRootCAsConfigured,
			ExplicitAllowedCIDRCount: security.AllowedCIDRCount,
		},
	}, nil
}

func (v *httpSaaSReleaseArtifactVerifier) Status() SaaSReleaseEvidenceVerifierStatus {
	if v == nil {
		return SaaSReleaseEvidenceVerifierStatus{}
	}
	return v.status
}

func (v *httpSaaSReleaseArtifactVerifier) Verify(ctx context.Context, rawURL string, expectedSHA256 string, expectedSizeBytes int64) (SaaSReleaseArtifactVerification, error) {
	result := SaaSReleaseArtifactVerification{
		EvidenceURL: rawURL, ExpectedSHA256: strings.ToLower(strings.TrimSpace(expectedSHA256)),
		ExpectedSizeBytes: expectedSizeBytes, AttemptedAt: time.Now().UTC().Format(time.RFC3339),
	}
	fail := func(err error) (SaaSReleaseArtifactVerification, error) {
		result.Error = truncateSaaSReleaseVerificationText(err.Error())
		return result, errors.New(result.Error)
	}
	if v == nil || v.client == nil {
		return fail(errors.New("远端工件校验器未配置"))
	}
	if err := validateSaaSReleaseEvidenceURL(rawURL); err != nil {
		return fail(err)
	}
	if !saasReleaseFingerprintPattern.MatchString(result.ExpectedSHA256) || expectedSizeBytes <= 0 {
		return fail(errors.New("工件预期 SHA-256 或字节大小无效"))
	}
	if expectedSizeBytes > v.maxBytes {
		return fail(fmt.Errorf("工件预期大小 %d 超过校验上限 %d", expectedSizeBytes, v.maxBytes))
	}

	requestContext, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, rawURL, nil)
	if err != nil {
		return fail(fmt.Errorf("创建远端工件请求失败: %w", err))
	}
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "mochat-go-release-evidence-verifier/1.0")
	response, err := v.client.Do(request)
	if err != nil {
		return fail(fmt.Errorf("下载远端工件失败: %w", err))
	}
	defer response.Body.Close()
	result.HTTPStatus = response.StatusCode
	result.ContentType = truncateSaaSReleaseVerificationText(response.Header.Get("Content-Type"))
	result.ETag = truncateSaaSReleaseVerificationText(response.Header.Get("ETag"))
	result.LastModified = truncateSaaSReleaseVerificationText(response.Header.Get("Last-Modified"))
	if response.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("远端工件返回 HTTP %d", response.StatusCode))
	}
	if response.ContentLength >= 0 && response.ContentLength != expectedSizeBytes {
		result.ActualSizeBytes = response.ContentLength
		return fail(fmt.Errorf("远端工件 Content-Length 为 %d，预期 %d", response.ContentLength, expectedSizeBytes))
	}

	hash := sha256.New()
	actualSize, err := io.Copy(hash, io.LimitReader(response.Body, expectedSizeBytes+1))
	result.ActualSizeBytes = actualSize
	result.ActualSHA256 = hex.EncodeToString(hash.Sum(nil))
	if err != nil {
		return fail(fmt.Errorf("读取远端工件失败: %w", err))
	}
	if actualSize != expectedSizeBytes {
		return fail(fmt.Errorf("远端工件实际大小为 %d，预期 %d", actualSize, expectedSizeBytes))
	}
	if result.ActualSHA256 != result.ExpectedSHA256 {
		return fail(errors.New("远端工件 SHA-256 与预期不一致"))
	}
	result.Verified = true
	return result, nil
}

func truncateSaaSReleaseVerificationText(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxSaaSReleaseVerificationTextLength {
		return value
	}
	return string(runes[:maxSaaSReleaseVerificationTextLength])
}
