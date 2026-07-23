package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type saasReleaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn saasReleaseRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newTestSaaSReleaseArtifactVerifier(body string, status int, contentLength int64) *httpSaaSReleaseArtifactVerifier {
	return &httpSaaSReleaseArtifactVerifier{
		client: &http.Client{Transport: saasReleaseRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}, "ETag": []string{"test-etag"}},
				Body:       io.NopCloser(strings.NewReader(body)), ContentLength: contentLength, Request: request,
			}, nil
		})},
		timeout: 2 * time.Second, maxBytes: 1024,
		status: SaaSReleaseEvidenceVerifierStatus{Configured: true, VerifyOnPass: true, VerifyOnCandidateGate: true},
	}
}

func TestSaaSReleaseArtifactVerifierStreamsAndMatchesArtifact(t *testing.T) {
	body := "verified production evidence"
	digest := sha256.Sum256([]byte(body))
	verifier := newTestSaaSReleaseArtifactVerifier(body, http.StatusOK, int64(len(body)))
	result, err := verifier.Verify(context.Background(), "https://evidence.company.cn/release/artifact", hex.EncodeToString(digest[:]), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Verified || result.ActualSHA256 != hex.EncodeToString(digest[:]) || result.ActualSizeBytes != int64(len(body)) || result.HTTPStatus != http.StatusOK {
		t.Fatalf("verification=%+v", result)
	}
}

func TestSaaSReleaseArtifactVerifierRejectsIntegrityAndResponseFailures(t *testing.T) {
	body := "verified production evidence"
	digest := sha256.Sum256([]byte(body))
	tests := []struct {
		name          string
		verifier      *httpSaaSReleaseArtifactVerifier
		expectedSHA   string
		expectedSize  int64
		errorContains string
	}{
		{name: "sha mismatch", verifier: newTestSaaSReleaseArtifactVerifier(body, http.StatusOK, int64(len(body))), expectedSHA: strings.Repeat("a", 64), expectedSize: int64(len(body)), errorContains: "SHA-256"},
		{name: "size mismatch", verifier: newTestSaaSReleaseArtifactVerifier(body, http.StatusOK, int64(len(body))), expectedSHA: hex.EncodeToString(digest[:]), expectedSize: int64(len(body) + 1), errorContains: "Content-Length"},
		{name: "http status", verifier: newTestSaaSReleaseArtifactVerifier(body, http.StatusNotFound, int64(len(body))), expectedSHA: hex.EncodeToString(digest[:]), expectedSize: int64(len(body)), errorContains: "HTTP 404"},
		{name: "max bytes", verifier: newTestSaaSReleaseArtifactVerifier(body, http.StatusOK, int64(len(body))), expectedSHA: hex.EncodeToString(digest[:]), expectedSize: 1025, errorContains: "校验上限"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.verifier.Verify(context.Background(), "https://evidence.company.cn/release/artifact", test.expectedSHA, test.expectedSize)
			if err == nil || result.Verified || !strings.Contains(result.Error, test.errorContains) {
				t.Fatalf("verification=%+v err=%v", result, err)
			}
		})
	}
}

func TestSaaSReleaseArtifactVerifierPreservesTransportFailure(t *testing.T) {
	verifier := &httpSaaSReleaseArtifactVerifier{
		client: &http.Client{Transport: saasReleaseRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		})},
		timeout: time.Second, maxBytes: 1024,
	}
	result, err := verifier.Verify(context.Background(), "https://evidence.company.cn/release/artifact", strings.Repeat("a", 64), 10)
	if err == nil || result.Verified || !strings.Contains(result.Error, "network unavailable") {
		t.Fatalf("verification=%+v err=%v", result, err)
	}
}
