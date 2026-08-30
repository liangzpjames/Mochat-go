package main

import (
	"context"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

type companyProfileWeComVerifier struct {
	client *dashboard.RoomWelcomeWeComClient
}

type weWorkCallbackRedisCapabilities interface {
	Ping(context.Context) error
	dashboard.ContactWelcomeEnqueuer
	dashboard.ContactWelcomeStatusCache
	dashboard.AutoTagMarkTagsQueue
}

type weWorkCallbackRedisCapabilityResolver struct {
	candidate weWorkCallbackRedisCapabilities
}

func (r weWorkCallbackRedisCapabilityResolver) ResolveWeWorkCallbackCapabilities(ctx context.Context) (dashboard.WeWorkCallbackWorkerCapabilities, error) {
	if r.candidate == nil || r.candidate.Ping(ctx) != nil {
		return dashboard.WeWorkCallbackWorkerCapabilities{}, fmt.Errorf("%w: dependency=redis capability=callback_downstream_queue", dashboard.ErrWeWorkCallbackDependencyUnavailable)
	}
	return dashboard.WeWorkCallbackWorkerCapabilities{ContactWelcomeQueue: r.candidate, ContactWelcomeCache: r.candidate, MarkTagsQueue: r.candidate}, nil
}

func optionalWeWorkCallbackCapabilities(ctx context.Context, candidate weWorkCallbackRedisCapabilities) (dashboard.WeWorkCallbackWorkerCapabilities, bool) {
	if candidate == nil || candidate.Ping(ctx) != nil {
		return dashboard.WeWorkCallbackWorkerCapabilities{}, false
	}
	return dashboard.WeWorkCallbackWorkerCapabilities{ContactWelcomeQueue: candidate, ContactWelcomeCache: candidate, MarkTagsQueue: candidate}, true
}

type dashboardArchiveComponentBridge struct {
	client *archiveprovider.BridgeArchiveClient
}

func (bridge dashboardArchiveComponentBridge) FetchArchiveComponent(ctx context.Context, object dashboard.ArchiveComponentObject) (dashboard.ArchiveComponentContent, error) {
	if bridge.client == nil {
		return dashboard.ArchiveComponentContent{}, fmt.Errorf("archive component bridge is unavailable")
	}
	content, err := bridge.client.FetchComponent(ctx, archiveprovider.ComponentRequest{Scope: archiveprovider.Scope{TenantID: int64(object.TenantID), CorpID: int64(object.CorpID)}, WXCorpID: object.WXCorpID, MessageID: object.MessageID, PublicKeyVersion: object.PublicKeyVersion, EncryptedSecretKey: object.EncryptedSecretKey})
	if err != nil {
		return dashboard.ArchiveComponentContent{}, err
	}
	return dashboard.ArchiveComponentContent{Type: content.Type, MIMEType: content.MIMEType, FileName: content.FileName, Body: content.Data}, nil
}

func (v companyProfileWeComVerifier) Verify(ctx context.Context, request companyprofile.VerificationRequest) (companyprofile.VerificationResult, error) {
	if v.client == nil || request.TenantID <= 0 || request.CorpID <= 0 || strings.TrimSpace(request.WXCorpID) == "" {
		return companyprofile.VerificationResult{}, fmt.Errorf("company verification provider is not configured")
	}
	result, err := v.client.VerifyCompany(ctx, request.WXCorpID, request.Credentials.EmployeeSecret, request.Credentials.ContactSecret)
	if err != nil {
		return companyprofile.VerificationResult{}, err
	}
	return companyprofile.VerificationResult{WXCorpID: result.WXCorpID, CorpName: result.CorpName}, nil
}
