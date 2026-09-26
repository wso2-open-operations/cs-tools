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

// Package choreosubscription provides an HTTP client for the Choreo
// choreo-subscription-on-project-create operation.
package choreosubscription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// jsonInt is an int that also accepts a JSON number written with a decimal
// point — 5.0 as well as 5.
type jsonInt int

func (n *jsonInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return fmt.Errorf("choreosubscription: %v is not a whole number", f)
	}
	*n = jsonInt(f)
	return nil
}

// ConsumptionStatusRequest is the input for POST /projects/{projectId}/consumption/status.
type ConsumptionStatusRequest struct {
	Email        string `json:"email"`
	DeploymentID string `json:"deploymentId"`
}

// ConsumptionData carries the current provisioning state for a project.
type ConsumptionData struct {
	Status        jsonInt `json:"status"`
	ApplicationID *string `json:"applicationId,omitempty"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
}

// ConsumptionResult wraps ConsumptionData.
type ConsumptionResult struct {
	Message       *string         `json:"message,omitempty"`
	ApplicationID *string         `json:"applicationId,omitempty"`
	Result        ConsumptionData `json:"result"`
}

// ApplicationCreateRequest is the input for POST /applications.
type ApplicationCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ApplicationCreateResponse is the response for POST /applications.
type ApplicationCreateResponse struct {
	Name          string `json:"name"`
	ApplicationID string `json:"applicationId"`
}

// UpdateProjectStatusRequest is the input for PATCH /projects/{projectId}.
type UpdateProjectStatusRequest struct {
	Status             int     `json:"status"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	ConsumerKey        *string `json:"consumerKey,omitempty"`
	ConsumerSecret     *string `json:"consumerSecret,omitempty"`
	PrimarySecretKey   *string `json:"primarySecretKey,omitempty"`
	SecondarySecretKey *string `json:"secondarySecretKey,omitempty"`
}

// ApplicationSubscriptionResponse is the response for
// POST /applications/{applicationId}/subscribe.
type ApplicationSubscriptionResponse struct {
	ApplicationID  string `json:"applicationId"`
	SubscriptionID string `json:"subscriptionId"`
	APIID          string `json:"apiId"`
}

// ApplicationKeyGenerationResponse is the response for
// POST /applications/{applicationId}/generate-credentials.
type ApplicationKeyGenerationResponse struct {
	ConsumerKey    string `json:"consumerKey"`
	ConsumerSecret string `json:"consumerSecret"`
}

// SecretKeysResponse is the response for POST /generate-secret-keys.
type SecretKeysResponse struct {
	PrimarySecretKey   string `json:"primarySecretKey"`
	SecondarySecretKey string `json:"secondarySecretKey"`
}

type licenseResult struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	License domain.License `json:"license"`
}

type licenseResponse struct {
	Result licenseResult `json:"result"`
}

// Client abstracts operations against choreo-subscription-on-project-create.
type Client interface {
	GetConsumptionStatus(ctx context.Context, projectID string, req ConsumptionStatusRequest) (ConsumptionResult, error)
	CreateApplication(ctx context.Context, req ApplicationCreateRequest) (ApplicationCreateResponse, error)
	SubscribeApplication(ctx context.Context, applicationID string) (ApplicationSubscriptionResponse, error)
	GenerateCredentials(ctx context.Context, applicationID string) (ApplicationKeyGenerationResponse, error)
	GenerateSecretKeys(ctx context.Context) (SecretKeysResponse, error)
	UpdateProjectStatus(ctx context.Context, projectID string, req UpdateProjectStatusRequest) (ConsumptionResult, error)
	GetDeploymentLicense(ctx context.Context, projectID, deploymentID string, req domain.DeploymentLicenseRequest) (domain.License, error)
}

// provisioningTimeout bounds one upstream provisioning call. The flow makes
// up to five of them in sequence, each reaching ServiceNow or the Choreo
// Developer Portal.
const provisioningTimeout = 60 * time.Second

// noRedirect keeps an authenticated request from being replayed against
// whatever host a redirect names — the bearer token would travel with it.
func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// ClientCredentialsConfig holds the OAuth2 client credentials used to obtain a
// bearer token for the operation, which sits behind the Choreo gateway like
// every other upstream this service calls.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is a space-separated list of OAuth2 scopes.
	Scopes string
}

// tokenResponse is the subset of the OAuth2 token endpoint response we use.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Config configures the client. Credentials are mandatory: the operation
// creates Choreo applications and issues signed licences for real customers,
// and the gateway rejects an unauthenticated caller.
type Config struct {
	BaseURL string
	Creds   ClientCredentialsConfig
	// HTTPClient replaces the default transport. Tests inject httptest's
	// client here; leave it nil everywhere else.
	HTTPClient *http.Client
}

type client struct {
	baseURL    string
	creds      ClientCredentialsConfig
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// requireSecureOrLoopback rejects a plaintext HTTP URL unless it points at the
// local machine.
func requireSecureOrLoopback(label, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("choreosubscription: %s is not a valid URL: %w", label, err)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" || net.ParseIP(host).IsLoopback() {
			return nil
		}
		return fmt.Errorf("choreosubscription: %s must use https (got http://%s)", label, host)
	default:
		return fmt.Errorf("choreosubscription: %s must use https, got scheme %q", label, u.Scheme)
	}
}

// NewClient constructs a Client that authenticates against the Choreo
// subscription operation with the OAuth2 client-credentials grant.
//
// It returns an error rather than a half-configured client when the base URL
// or any credential is missing: a client that silently sends unauthenticated
// requests fails only at provisioning time, against production.
func NewClient(cfg Config) (Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("choreosubscription: base URL is required")
	}
	switch {
	case cfg.Creds.TokenURL == "":
		return nil, errors.New("choreosubscription: token URL is required")
	case cfg.Creds.ClientID == "":
		return nil, errors.New("choreosubscription: client ID is required")
	case cfg.Creds.ClientSecret == "":
		return nil, errors.New("choreosubscription: client secret is required")
	}
	// The client secret travels to the token URL and the bearer token to the
	// base URL, so neither may be plaintext HTTP off-host. Loopback is allowed
	// so tests and a locally-run operation still work — traffic that never
	// leaves the machine has no network to be intercepted on.
	if err := requireSecureOrLoopback("base URL", cfg.BaseURL); err != nil {
		return nil, err
	}
	if err := requireSecureOrLoopback("token URL", cfg.Creds.TokenURL); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: provisioningTimeout, CheckRedirect: noRedirect}
	}
	return &client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		creds:      cfg.Creds,
		httpClient: httpClient,
	}, nil
}

// accessToken returns a valid bearer token, fetching a new one if the cached
// token is absent or within 30 seconds of expiry — the same caching the
// ServiceNow integration client uses.
func (c *client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Add(30*time.Second).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.creds.ClientID)
	data.Set("client_secret", c.creds.ClientSecret)
	if c.creds.Scopes != "" {
		data.Set("scope", c.creds.Scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.creds.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("choreosubscription: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("choreosubscription: token endpoint unavailable: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("choreosubscription: read token response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", &apierror.UnauthorizedError{Msg: fmt.Sprintf("choreosubscription: token endpoint rejected credentials (status %d) — check client ID and secret", resp.StatusCode)}
	default:
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("choreosubscription: token endpoint returned %d", resp.StatusCode)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("choreosubscription: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", errors.New("choreosubscription: token endpoint returned empty access_token")
	}

	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(3600 * time.Second)
	}
	return c.cachedToken, nil
}

func (c *client) GetConsumptionStatus(ctx context.Context, projectID string, req ConsumptionStatusRequest) (ConsumptionResult, error) {
	req.DeploymentID = uuidToSysID(req.DeploymentID)
	var out ConsumptionResult
	path := fmt.Sprintf("/projects/%s/consumption/status", url.PathEscape(uuidToSysID(projectID)))
	err := c.postJSON(ctx, path, req, &out)
	return out, err
}

func (c *client) CreateApplication(ctx context.Context, req ApplicationCreateRequest) (ApplicationCreateResponse, error) {
	var out ApplicationCreateResponse
	err := c.postJSON(ctx, "/applications", req, &out)
	return out, err
}

func (c *client) SubscribeApplication(ctx context.Context, applicationID string) (ApplicationSubscriptionResponse, error) {
	var out ApplicationSubscriptionResponse
	path := fmt.Sprintf("/applications/%s/subscribe", url.PathEscape(applicationID))
	err := c.postText(ctx, path, applicationID, &out)
	return out, err
}

func (c *client) GenerateCredentials(ctx context.Context, applicationID string) (ApplicationKeyGenerationResponse, error) {
	var out ApplicationKeyGenerationResponse
	path := fmt.Sprintf("/applications/%s/generate-credentials", url.PathEscape(applicationID))
	err := c.postJSON(ctx, path, struct{}{}, &out)
	return out, err
}

func (c *client) GenerateSecretKeys(ctx context.Context) (SecretKeysResponse, error) {
	var out SecretKeysResponse
	err := c.postJSON(ctx, "/generate-secret-keys", struct{}{}, &out)
	return out, err
}

func (c *client) UpdateProjectStatus(ctx context.Context, projectID string, req UpdateProjectStatusRequest) (ConsumptionResult, error) {
	var out ConsumptionResult
	path := fmt.Sprintf("/projects/%s", url.PathEscape(uuidToSysID(projectID)))
	err := c.patchJSON(ctx, path, req, &out)
	return out, err
}

func (c *client) GetDeploymentLicense(ctx context.Context, projectID, deploymentID string, req domain.DeploymentLicenseRequest) (domain.License, error) {
	var out licenseResponse
	path := fmt.Sprintf("/projects/%s/deployments/%s/license",
		url.PathEscape(uuidToSysID(projectID)),
		url.PathEscape(uuidToSysID(deploymentID)),
	)
	err := c.postJSON(ctx, path, req, &out)
	if err != nil {
		return domain.License{}, err
	}
	// A refusal arrives as a 200 carrying success:false — "Deployment not
	// found" is the one seen in practice. Without this check the caller gets an
	// empty licence and no error, and hands the customer a file with no
	// subscription data and no signature.
	if !out.Result.Success {
		msg := out.Result.Message
		if msg == "" {
			msg = "no reason given"
		}
		// A missing deployment is the caller's mistake, not this service's, and
		// openapi.yaml documents a 404 for it. Every other refusal stays an
		// ordinary error and surfaces as a 500, which is what it is: the
		// licensing service declined for a reason we do not model.
		//
		// Matching on the upstream message is the only signal available — the
		// refusal carries no code, and the transport status is 200 either way.
		// If that wording ever changes the case degrades to a 500 rather than
		// misreporting something else as not-found, so this fails safe.
		if strings.Contains(strings.ToLower(msg), "not found") {
			slog.WarnContext(ctx, "the licensing service reported the deployment as not found",
				"projectId", projectID, "deploymentId", deploymentID, "upstreamMessage", msg)
			return domain.License{}, &apierror.NotFoundError{Msg: "deployment not found"}
		}
		return domain.License{}, fmt.Errorf("choreosubscription: the licensing service did not issue a licence: %s", msg)
	}
	if !isJSONObject(out.Result.License.SubscriptionData) {
		return domain.License{}, errors.New("choreosubscription: the licensing service reported success but returned no subscription data")
	}
	return out.Result.License, nil
}

// isJSONObject reports whether raw holds a JSON object.
//
// A length check alone is not enough: json.RawMessage stores JSON null as the
// four bytes "null", which is non-empty but carries nothing to sign or verify,
// and both licence schemas declare subscriptionData an object. Anything that is
// not an object would reach the customer as a licence their product cannot
// parse.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func (c *client) postJSON(ctx context.Context, path string, body any, out any) error {
	return c.doRequest(ctx, http.MethodPost, path, "application/json", body, out)
}

func (c *client) patchJSON(ctx context.Context, path string, body any, out any) error {
	return c.doRequest(ctx, http.MethodPatch, path, "application/json", body, out)
}

func (c *client) postText(ctx context.Context, path string, text string, out any) error {
	return c.doRequest(ctx, http.MethodPost, path, "text/plain", strings.NewReader(text), out)
}

func (c *client) doRequest(ctx context.Context, method, path, contentType string, body any, out any) error {
	var bodyReader io.Reader
	if reader, ok := body.(io.Reader); ok {
		bodyReader = reader
	} else if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("choreosubscription: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("choreosubscription: create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("choreosubscription: upstream call failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("choreosubscription: upstream returned %d: %s", res.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return fmt.Errorf("choreosubscription: decode response body: %w", err)
		}
	}
	return nil
}

// uuidToSysID strips hyphens from a canonical UUID string to match ServiceNow's
// 32-hex character sysid format.
func uuidToSysID(id string) string {
	if len(id) == 36 && id[8] == '-' && id[13] == '-' && id[18] == '-' && id[23] == '-' {
		return strings.ReplaceAll(id, "-", "")
	}
	return id
}
