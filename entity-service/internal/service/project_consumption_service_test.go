// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/choreosubscription"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const testConsumptionProjectID = "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77"

// fakeProjectConsumptionRepo records what the service forwards and returns a
// scripted result, so the state-machine rules can be tested without Postgres.
type fakeProjectConsumptionRepo struct {
	state     domain.ProjectConsumption
	name      string
	key       string
	getErr    error
	upsertErr error

	gotUpsert   domain.ProjectConsumption
	upsertCalls int
}

func (f *fakeProjectConsumptionRepo) Get(_ context.Context, projectID string) (domain.ProjectConsumption, string, string, error) {
	if f.getErr != nil {
		return domain.ProjectConsumption{}, "", "", f.getErr
	}
	state := f.state
	state.ProjectID = projectID
	return state, f.name, f.key, nil
}

func (f *fakeProjectConsumptionRepo) Upsert(_ context.Context, _ string, next domain.ProjectConsumption) (domain.ProjectConsumption, error) {
	f.upsertCalls++
	f.gotUpsert = next
	if f.upsertErr != nil {
		return domain.ProjectConsumption{}, f.upsertErr
	}
	return next, nil
}

func strPtrLocal(s string) *string { return &s }

// TestGetProjectConsumption_NeverReturnsSecrets is the important one. The view
// is what a caller receives on every license download, and the three credential
// values must not be on it — see domain.ProjectConsumptionView.
func TestGetProjectConsumption_NeverReturnsSecrets(t *testing.T) {
	const (
		secret    = "consumer-secret-value"
		primary   = "primary-secret-key-value"
		secondary = "secondary-secret-key-value"
	)
	repo := &fakeProjectConsumptionRepo{
		name: "Acme Production",
		key:  "ACME-PROD",
		state: domain.ProjectConsumption{
			Status:              domain.ConsumptionStatusGeneratedSecretKeys,
			ChoreoApplicationID: strPtrLocal("app-1"),
			ConsumerKey:         strPtrLocal("consumer-key"),
			ConsumerSecret:      strPtrLocal(secret),
			PrimarySecretKey:    strPtrLocal(primary),
			SecondarySecretKey:  strPtrLocal(secondary),
		},
	}
	svc := NewProjectConsumptionService(repo, nil, true)

	view, err := svc.GetProjectConsumption(context.Background(), testConsumptionProjectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert on the serialised form, not the struct: a future field added to
	// the view would otherwise slip a secret out without failing this test.
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, leaked := range []string{secret, primary, secondary} {
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("view exposes a secret value: %s", encoded)
		}
	}

	if !view.HasConsumerSecret || !view.HasSecretKeys {
		t.Fatalf("expected presence flags to be set, got %+v", view)
	}
	if view.ConsumerKey == nil || *view.ConsumerKey != "consumer-key" {
		t.Fatalf("consumerKey should be exposed (it is not a secret), got %+v", view.ConsumerKey)
	}
}

// TestGetProjectConsumption_UnprovisionedProjectIsPending covers the "no row
// yet" case: it is step 1 of the state machine, not a missing resource.
func TestGetProjectConsumption_UnprovisionedProjectIsPending(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{
		name:  "Acme Production",
		key:   "ACME-PROD",
		state: domain.ProjectConsumption{Status: domain.ConsumptionStatusPending},
	}
	svc := NewProjectConsumptionService(repo, nil, true)

	view, err := svc.GetProjectConsumption(context.Background(), testConsumptionProjectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Status != int16(domain.ConsumptionStatusPending) {
		t.Fatalf("got status %d, want %d", view.Status, domain.ConsumptionStatusPending)
	}
	if view.HasConsumerSecret || view.HasSecretKeys {
		t.Fatalf("expected no stored credentials, got %+v", view)
	}
}

func TestGetProjectConsumption_RejectsNonUUID(t *testing.T) {
	svc := NewProjectConsumptionService(&fakeProjectConsumptionRepo{}, nil, true)

	_, err := svc.GetProjectConsumption(context.Background(), "6fa0b42d1bfa4a69a002c9d3604bcb77")
	var validationErr *apierror.ValidationError
	if !asValidationError(err, &validationErr) {
		t.Fatalf("got %v, want a ValidationError", err)
	}
}

// TestUpdateProjectConsumption_RequiresStepArtefacts asserts each step's own
// output is mandatory. Advancing the status without storing what that step
// produced is what would make a later resume re-run a side-effecting call.
func TestUpdateProjectConsumption_RequiresStepArtefacts(t *testing.T) {
	tests := map[string]struct {
		req  domain.UpdateProjectConsumptionRequest
		want string
	}{
		"created without applicationId": {
			req:  domain.UpdateProjectConsumptionRequest{Status: int16(domain.ConsumptionStatusCreated)},
			want: "applicationId",
		},
		"credentials without consumerKey": {
			req: domain.UpdateProjectConsumptionRequest{
				Status:         int16(domain.ConsumptionStatusGeneratedCredentials),
				ConsumerSecret: strPtrLocal("s"),
			},
			want: "consumerKey",
		},
		"credentials without consumerSecret": {
			req: domain.UpdateProjectConsumptionRequest{
				Status:      int16(domain.ConsumptionStatusGeneratedCredentials),
				ConsumerKey: strPtrLocal("k"),
			},
			want: "consumerSecret",
		},
		"secret keys without primary": {
			req: domain.UpdateProjectConsumptionRequest{
				Status:             int16(domain.ConsumptionStatusGeneratedSecretKeys),
				SecondarySecretKey: strPtrLocal("s"),
			},
			want: "primarySecretKey",
		},
		"secret keys without secondary": {
			req: domain.UpdateProjectConsumptionRequest{
				Status:           int16(domain.ConsumptionStatusGeneratedSecretKeys),
				PrimarySecretKey: strPtrLocal("p"),
			},
			want: "secondarySecretKey",
		},
		// An empty string is not a usable credential; it must be rejected the
		// same way an omitted field is.
		"empty string counts as missing": {
			req: domain.UpdateProjectConsumptionRequest{
				Status:        int16(domain.ConsumptionStatusCreated),
				ApplicationID: strPtrLocal(""),
			},
			want: "applicationId",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repo := &fakeProjectConsumptionRepo{}
			svc := NewProjectConsumptionService(repo, nil, true)

			_, err := svc.UpdateProjectConsumption(context.Background(), testConsumptionProjectID, tc.req)
			var validationErr *apierror.ValidationError
			if !asValidationError(err, &validationErr) {
				t.Fatalf("got %v, want a ValidationError", err)
			}
			if !strings.Contains(validationErr.Msg, tc.want) {
				t.Fatalf("error %q does not name %q", validationErr.Msg, tc.want)
			}
			if repo.upsertCalls != 0 {
				t.Fatal("repository was called despite invalid input")
			}
		})
	}
}

// TestUpdateProjectConsumption_SubscribedNeedsNoArtefacts — step 3 produces
// nothing to store, so it must not be forced to carry a field.
func TestUpdateProjectConsumption_SubscribedNeedsNoArtefacts(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{
		name:  "Acme",
		key:   "ACME",
		state: domain.ProjectConsumption{Status: domain.ConsumptionStatusSubscribed},
	}
	svc := NewProjectConsumptionService(repo, nil, true)

	_, err := svc.UpdateProjectConsumption(context.Background(), testConsumptionProjectID,
		domain.UpdateProjectConsumptionRequest{Status: int16(domain.ConsumptionStatusSubscribed)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("got %d upsert calls, want 1", repo.upsertCalls)
	}
}

func TestUpdateProjectConsumption_RejectsOutOfRangeStatus(t *testing.T) {
	for _, status := range []int16{0, 6, -1, 100} {
		repo := &fakeProjectConsumptionRepo{}
		svc := NewProjectConsumptionService(repo, nil, true)

		_, err := svc.UpdateProjectConsumption(context.Background(), testConsumptionProjectID,
			domain.UpdateProjectConsumptionRequest{Status: status})
		var validationErr *apierror.ValidationError
		if !asValidationError(err, &validationErr) {
			t.Fatalf("status %d: got %v, want a ValidationError", status, err)
		}
		if repo.upsertCalls != 0 {
			t.Fatalf("status %d: repository was called", status)
		}
	}
}

// TestUpdateProjectConsumption_StaleStatusIsNoOp covers two callers racing for
// the same project. The loser must converge on the stored state rather than
// fail — a failure here would make the portal retry a step that already ran.
func TestUpdateProjectConsumption_StaleStatusIsNoOp(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{
		name:      "Acme",
		key:       "ACME",
		upsertErr: repository.ErrConsumptionStatusStale,
		state: domain.ProjectConsumption{
			Status:              domain.ConsumptionStatusGeneratedCredentials,
			ChoreoApplicationID: strPtrLocal("app-1"),
			ConsumerKey:         strPtrLocal("consumer-key"),
			ConsumerSecret:      strPtrLocal("secret"),
		},
	}
	svc := NewProjectConsumptionService(repo, nil, true)

	resp, err := svc.UpdateProjectConsumption(context.Background(), testConsumptionProjectID,
		domain.UpdateProjectConsumptionRequest{
			Status:        int16(domain.ConsumptionStatusCreated),
			ApplicationID: strPtrLocal("app-2"),
		})
	if err != nil {
		t.Fatalf("a stale status must not be an error, got %v", err)
	}
	if resp.Result.Status != int16(domain.ConsumptionStatusGeneratedCredentials) {
		t.Fatalf("got status %d, want the stored %d", resp.Result.Status, domain.ConsumptionStatusGeneratedCredentials)
	}
	if resp.Result.ApplicationID == nil || *resp.Result.ApplicationID != "app-1" {
		t.Fatalf("stale write must not overwrite the stored application id, got %+v", resp.Result.ApplicationID)
	}
}

// TestUpdateProjectConsumption_ForwardsOnlySuppliedFields guards the partial
// write: recording step 4 must not clear step 2's application id.
func TestUpdateProjectConsumption_ForwardsOnlySuppliedFields(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{name: "Acme", key: "ACME"}
	svc := NewProjectConsumptionService(repo, nil, true)

	_, err := svc.UpdateProjectConsumption(context.Background(), testConsumptionProjectID,
		domain.UpdateProjectConsumptionRequest{
			Status:         int16(domain.ConsumptionStatusGeneratedCredentials),
			ConsumerKey:    strPtrLocal("k"),
			ConsumerSecret: strPtrLocal("s"),
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotUpsert.ChoreoApplicationID != nil {
		t.Fatalf("an unsupplied field must reach the repository as nil, got %v", *repo.gotUpsert.ChoreoApplicationID)
	}
	if repo.gotUpsert.PrimarySecretKey != nil || repo.gotUpsert.SecondarySecretKey != nil {
		t.Fatal("unsupplied secret keys must reach the repository as nil")
	}
	if repo.gotUpsert.ConsumerKey == nil || *repo.gotUpsert.ConsumerKey != "k" {
		t.Fatalf("consumerKey was not forwarded: %+v", repo.gotUpsert.ConsumerKey)
	}
}

type fakeChoreoSubscriptionClient struct {
	statusRes         choreosubscription.ConsumptionResult
	statusErr         error
	createAppRes      choreosubscription.ApplicationCreateResponse
	createAppErr      error
	subscribeRes      choreosubscription.ApplicationSubscriptionResponse
	subscribeErr      error
	generateCredsRes  choreosubscription.ApplicationKeyGenerationResponse
	generateCredsErr  error
	generateSecretRes choreosubscription.SecretKeysResponse
	generateSecretErr error
	updateStatusRes   choreosubscription.ConsumptionResult
	updateStatusErr   error
	licenseRes        domain.License
	licenseErr        error

	statusCalls         int
	createAppCalls      int
	subscribeCalls      int
	generateCredsCalls  int
	generateSecretCalls int
	updateStatusCalls   []choreosubscription.UpdateProjectStatusRequest
	licenseCalls        int
}

func (f *fakeChoreoSubscriptionClient) GetConsumptionStatus(_ context.Context, _ string, _ choreosubscription.ConsumptionStatusRequest) (choreosubscription.ConsumptionResult, error) {
	f.statusCalls++
	return f.statusRes, f.statusErr
}

func (f *fakeChoreoSubscriptionClient) CreateApplication(_ context.Context, _ choreosubscription.ApplicationCreateRequest) (choreosubscription.ApplicationCreateResponse, error) {
	f.createAppCalls++
	return f.createAppRes, f.createAppErr
}

func (f *fakeChoreoSubscriptionClient) SubscribeApplication(_ context.Context, _ string) (choreosubscription.ApplicationSubscriptionResponse, error) {
	f.subscribeCalls++
	return f.subscribeRes, f.subscribeErr
}

func (f *fakeChoreoSubscriptionClient) GenerateCredentials(_ context.Context, _ string) (choreosubscription.ApplicationKeyGenerationResponse, error) {
	f.generateCredsCalls++
	return f.generateCredsRes, f.generateCredsErr
}

func (f *fakeChoreoSubscriptionClient) GenerateSecretKeys(_ context.Context) (choreosubscription.SecretKeysResponse, error) {
	f.generateSecretCalls++
	return f.generateSecretRes, f.generateSecretErr
}

func (f *fakeChoreoSubscriptionClient) UpdateProjectStatus(_ context.Context, _ string, req choreosubscription.UpdateProjectStatusRequest) (choreosubscription.ConsumptionResult, error) {
	f.updateStatusCalls = append(f.updateStatusCalls, req)
	return f.updateStatusRes, f.updateStatusErr
}

func (f *fakeChoreoSubscriptionClient) GetDeploymentLicense(_ context.Context, _, _ string, _ domain.DeploymentLicenseRequest) (domain.License, error) {
	f.licenseCalls++
	return f.licenseRes, f.licenseErr
}

func TestProcessLicenseDownload_FullFlow(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{
		name:  "Acme",
		key:   "ACME",
		state: domain.ProjectConsumption{Status: domain.ConsumptionStatusPending},
	}
	appID := "app-xyz"
	appName := "ACME-APP"
	appDesc := "Description for ACME"
	choreo := &fakeChoreoSubscriptionClient{
		statusRes: choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:      1,
				Name:        &appName,
				Description: &appDesc,
			},
		},
		createAppRes: choreosubscription.ApplicationCreateResponse{
			ApplicationID: appID,
		},
		generateCredsRes: choreosubscription.ApplicationKeyGenerationResponse{
			ConsumerKey:    "ck-123",
			ConsumerSecret: "cs-123",
		},
		generateSecretRes: choreosubscription.SecretKeysResponse{
			PrimarySecretKey:   "pk-123",
			SecondarySecretKey: "sk-123",
		},
		licenseRes: domain.License{
			Signature:        "test-sig",
			SubscriptionData: json.RawMessage(`{"deploymentId":"11111111-1111-1111-1111-111111111111"}`),
		},
	}
	svc := NewProjectConsumptionService(repo, choreo, true)

	lic, err := svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lic.Signature != "test-sig" {
		t.Fatalf("got signature %q, want test-sig", lic.Signature)
	}
	if choreo.createAppCalls != 1 {
		t.Errorf("got %d createApp calls, want 1", choreo.createAppCalls)
	}
	if choreo.subscribeCalls != 1 {
		t.Errorf("got %d subscribe calls, want 1", choreo.subscribeCalls)
	}
	if choreo.generateCredsCalls != 1 {
		t.Errorf("got %d generateCreds calls, want 1", choreo.generateCredsCalls)
	}
	if choreo.generateSecretCalls != 1 {
		t.Errorf("got %d generateSecret calls, want 1", choreo.generateSecretCalls)
	}
	if len(choreo.updateStatusCalls) != 4 {
		t.Fatalf("got %d updateStatus calls, want 4", len(choreo.updateStatusCalls))
	}
	if repo.upsertCalls != 4 {
		t.Fatalf("got %d repo upsert calls, want 4", repo.upsertCalls)
	}
}

func TestProcessLicenseDownload_ResumeFromSubscribed(t *testing.T) {
	repo := &fakeProjectConsumptionRepo{
		name:  "Acme",
		key:   "ACME",
		state: domain.ProjectConsumption{Status: domain.ConsumptionStatusSubscribed},
	}
	appID := "app-xyz"
	choreo := &fakeChoreoSubscriptionClient{
		statusRes: choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:        3,
				ApplicationID: &appID,
			},
		},
		generateCredsRes: choreosubscription.ApplicationKeyGenerationResponse{
			ConsumerKey:    "ck-123",
			ConsumerSecret: "cs-123",
		},
		generateSecretRes: choreosubscription.SecretKeysResponse{
			PrimarySecretKey:   "pk-123",
			SecondarySecretKey: "sk-123",
		},
		licenseRes: domain.License{Signature: "sig-resumed"},
	}
	svc := NewProjectConsumptionService(repo, choreo, true)

	lic, err := svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lic.Signature != "sig-resumed" {
		t.Fatalf("got signature %q, want sig-resumed", lic.Signature)
	}
	if choreo.createAppCalls != 0 || choreo.subscribeCalls != 0 {
		t.Errorf("createApp (%d) or subscribe (%d) called on resume from step 3", choreo.createAppCalls, choreo.subscribeCalls)
	}
	if choreo.generateCredsCalls != 1 || choreo.generateSecretCalls != 1 {
		t.Errorf("creds (%d) or secret (%d) not called as expected", choreo.generateCredsCalls, choreo.generateSecretCalls)
	}
	if len(choreo.updateStatusCalls) != 2 {
		t.Errorf("got %d updateStatus calls, want 2", len(choreo.updateStatusCalls))
	}
}

func TestProcessLicenseDownload_CompletedDirectLicense(t *testing.T) {
	appID := "app-xyz"
	choreo := &fakeChoreoSubscriptionClient{
		statusRes: choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:        5,
				ApplicationID: &appID,
			},
		},
		licenseRes: domain.License{Signature: "sig-direct"},
	}
	repo := &fakeProjectConsumptionRepo{}
	svc := NewProjectConsumptionService(repo, choreo, true)

	lic, err := svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lic.Signature != "sig-direct" {
		t.Fatalf("got signature %q, want sig-direct", lic.Signature)
	}
	if choreo.licenseCalls != 1 || len(choreo.updateStatusCalls) != 0 || repo.upsertCalls != 0 {
		t.Fatalf("completed status mutated state: updateCalls=%d upsertCalls=%d", len(choreo.updateStatusCalls), repo.upsertCalls)
	}
}

func TestProcessLicenseDownload_PostgresWriteErrorIsNonFatal(t *testing.T) {
	appID := "app-xyz"
	appName := "ACME-APP"
	appDesc := "Description for ACME"
	choreo := &fakeChoreoSubscriptionClient{
		statusRes: choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:      1,
				Name:        &appName,
				Description: &appDesc,
			},
		},
		createAppRes: choreosubscription.ApplicationCreateResponse{
			ApplicationID: appID,
		},
		generateCredsRes: choreosubscription.ApplicationKeyGenerationResponse{
			ConsumerKey:    "ck-123",
			ConsumerSecret: "cs-123",
		},
		generateSecretRes: choreosubscription.SecretKeysResponse{
			PrimarySecretKey:   "pk-123",
			SecondarySecretKey: "sk-123",
		},
		licenseRes: domain.License{Signature: "sig-db-err"},
	}
	repo := &fakeProjectConsumptionRepo{upsertErr: errors.New("db write failed")}
	svc := NewProjectConsumptionService(repo, choreo, true)

	lic, err := svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err != nil {
		t.Fatalf("postgres upsert error must be non-fatal, got %v", err)
	}
	if lic.Signature != "sig-db-err" {
		t.Fatalf("got signature %q, want sig-db-err", lic.Signature)
	}
}

func TestProcessLicenseDownload_ServiceNowWriteErrorIsFatal(t *testing.T) {
	appID := "app-xyz"
	appName := "ACME-APP"
	appDesc := "Description for ACME"
	choreo := &fakeChoreoSubscriptionClient{
		statusRes: choreosubscription.ConsumptionResult{
			Result: choreosubscription.ConsumptionData{
				Status:      1,
				Name:        &appName,
				Description: &appDesc,
			},
		},
		createAppRes: choreosubscription.ApplicationCreateResponse{
			ApplicationID: appID,
		},
		updateStatusErr: errors.New("servicenow update failed"),
	}
	repo := &fakeProjectConsumptionRepo{}
	svc := NewProjectConsumptionService(repo, choreo, true)

	_, err := svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err == nil {
		t.Fatal("expected error when servicenow update fails, got nil")
	}
	if choreo.subscribeCalls != 0 {
		t.Errorf("subscribe called despite previous step failing")
	}
}

func TestProcessLicenseDownload_ValidationErrors(t *testing.T) {
	svc := NewProjectConsumptionService(&fakeProjectConsumptionRepo{}, &fakeChoreoSubscriptionClient{}, true)

	// Bad project ID
	_, err := svc.ProcessLicenseDownload(context.Background(), "invalid-uuid", "11111111-1111-1111-1111-111111111111", "test@wso2.com")
	if err == nil {
		t.Fatal("expected validation error for invalid project ID")
	}

	// Bad deployment ID
	_, err = svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "invalid-uuid", "test@wso2.com")
	if err == nil {
		t.Fatal("expected validation error for invalid deployment ID")
	}

	// Empty email
	_, err = svc.ProcessLicenseDownload(context.Background(), testConsumptionProjectID, "11111111-1111-1111-1111-111111111111", "")
	if err == nil {
		t.Fatal("expected validation error for empty email")
	}
}
