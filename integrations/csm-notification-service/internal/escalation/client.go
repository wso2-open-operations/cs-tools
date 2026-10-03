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

package escalation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// EntityConfig holds the configuration for the entity-service client below.
// Same shape and same credential story as slaengine.EntityConfig: BaseURL and
// Scopes are this client's own env vars, while TokenURL/ClientID/ClientSecret
// are filled by cmd/server/main.go from the shared OAUTH2_* app.
type EntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EntityClient is a narrow entity-service client with exactly one job:
// appending the execution summary to an incident as a work note (section
// 11.0). A deliberate second client rather than a method on
// slaengine.EntityClient — that one exists for the sla_clocks endpoints, which
// have nothing to do with incidents.
type EntityClient struct {
	http    *http.Client
	baseURL string
	// tokens is the same client-credentials source the HTTP client uses. It is
	// held separately so do() can put the access token in x-jwt-assertion as
	// well as in Authorization -- see do() for why that header is the one that
	// decides whether this caller counts as internal.
	tokens oauth2.TokenSource
}

// NewEntityClient constructs an EntityClient authenticated via the OAuth2
// client credentials grant. Never fails and never contacts the token endpoint
// — a missing or invalid configuration only surfaces as an error the first
// time AppendWorkNote is called.
func NewEntityClient(cfg EntityConfig) *EntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &EntityClient{
		tokens:  cc.TokenSource(tokenCtx),
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// updateIncidentRequest is the subset of entity-service's
// PATCH /incidents/{id} body this client sends. Work notes only, and that
// matters twice over for loop safety:
//
//   - A PATCH touching neither state nor priority publishes no escalation
//     signal (see entity-service's publishEscalationSignals), so writing a
//     summary cannot come back as a new trigger or an acknowledgement.
//   - incident.comment_added is published by CreateComment, a different
//     endpoint this client never calls — and even if it were reached, a work
//     note carries IsPublic false, which the engine ignores.
//
// So the engine writing its own execution summary can never cancel a ladder,
// including the one it is writing the summary for.
type updateIncidentRequest struct {
	WorkNotes string `json:"workNotes"`
}

// AppendWorkNote writes note onto the incident as a work note. ServiceNow
// work-note fields are journals — a write appends an entry rather than
// replacing the field — so this is additive despite being a PATCH.
func (c *EntityClient) AppendWorkNote(ctx context.Context, incidentID, note string) error {
	if strings.TrimSpace(incidentID) == "" {
		return fmt.Errorf("escalation: incidentId is required")
	}
	body, err := json.Marshal(updateIncidentRequest{WorkNotes: note})
	if err != nil {
		return fmt.Errorf("escalation: encode work note: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(incidentID), body)
	return err
}

// teamMember is one row of GET /team-schedule/members.
type teamMember struct {
	TeamKey string `json:"teamKey"`
	Role    string `json:"role"`
	// TeamType is the team's ABT: cre-abt (seven teams), sre-abt (two), or
	// cre for a team like Americas that belongs to no ABT at all.
	TeamType string `json:"teamType,omitempty"`
	// AlertTier is the standing alert-duty nomination (T1/T2/T3), empty when
	// this member holds none. A different axis from Role -- see entity-service
	// migration 0171 -- and what the ladder's first rung resolves from on the
	// rules whose Level 0 is a nominated set rather than the rota.
	AlertTier string `json:"alertTier,omitempty"`
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}

type teamMembersResponse struct {
	Members []teamMember `json:"members"`
}

// TeamMembers returns everyone holding one of roles on one of teamKeys.
//
// An empty result is not an error: a team with nobody at that rank is a rung
// that reaches nobody, which the ladder logs and climbs past.
func (c *EntityClient) TeamMembers(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]teamMember, error) {
	if len(teamKeys) == 0 && len(teamTypes) == 0 {
		return nil, nil
	}
	q := url.Values{}
	if len(teamKeys) > 0 {
		q.Set("teamKeys", strings.Join(teamKeys, ","))
	}
	if len(teamTypes) > 0 {
		q.Set("teamTypes", strings.Join(teamTypes, ","))
	}
	if len(roles) > 0 {
		q.Set("roles", strings.Join(roles, ","))
	}
	if len(alertTiers) > 0 {
		q.Set("alertTiers", strings.Join(alertTiers, ","))
	}
	raw, err := c.do(ctx, http.MethodGet, "/team-schedule/members?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var resp teamMembersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("escalation: decode team members: %w", err)
	}
	return resp.Members, nil
}

// onDutyAssignment is the part of GET /team-schedule/on-duty this needs: who,
// and which team they were rostered under.
type onDutyAssignment struct {
	Engineer struct {
		UserID string `json:"userId"`
		Name   string `json:"name"`
		Email  string `json:"email"`
	} `json:"engineer"`
	TeamKey   string `json:"teamKey"`
	ShiftCode string `json:"shiftCode"`
}

type onDutyResponse struct {
	Assignments []onDutyAssignment `json:"assignments"`
}

// OnDutyAt returns everyone whose rostered window covers at.
//
// Deliberately separate from TeamMembers rather than one endpoint answering
// both: rank lives on the membership and changes rarely, the rota changes
// daily, and folding rank into the rota response would widen a shape the Team
// Schedule page already renders.
func (c *EntityClient) OnDutyAt(ctx context.Context, at time.Time) ([]onDutyAssignment, error) {
	path := "/team-schedule/on-duty"
	if !at.IsZero() {
		path += "?at=" + url.QueryEscape(at.UTC().Format(time.RFC3339))
	}
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp onDutyResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("escalation: decode on-duty: %w", err)
	}
	return resp.Assignments, nil
}

// do executes an authenticated HTTP request against entity-service and returns
// the raw JSON response body, or an *apierror.Error for a non-2xx status.
// Mirrors slaengine.EntityClient.do exactly.
func (c *EntityClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("escalation: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setClientAssertion(ctx, req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("escalation: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("escalation: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// setClientAssertion puts the access token in x-jwt-assertion as well as in
// Authorization, which the OAuth2 transport sets on its own.
//
// entity-service decides whether a caller is internal -- and so whether it may
// read the rota at all -- from x-jwt-assertion's client_id claim, not from
// Authorization. In a Choreo deployment its gateway translates one into the
// other before entity-service sees the request, so this works in production
// whether or not the header is set here. Nothing translates it locally, so
// without this every rota lookup is refused and every rung of a real ladder
// resolves to nobody. Setting it is also what csm-portal-backend's own CORS
// allow-list describes as intended: local testing that bypasses the gateway,
// and defence in depth behind it.
//
// Best-effort: a token fetch that fails leaves the header off and lets the
// request go, so the failure surfaces as the upstream's own status rather than
// as a client-side error that hides it. The transport will fail the same fetch
// a moment later anyway.
func (c *EntityClient) setClientAssertion(ctx context.Context, req *http.Request) {
	if c.tokens == nil {
		return
	}
	tok, err := c.tokens.Token()
	if err != nil || tok == nil || tok.AccessToken == "" {
		slog.WarnContext(ctx, "escalation: could not attach the client assertion; "+
			"entity-service will not see this caller as internal")
		return
	}
	req.Header.Set("x-jwt-assertion", tok.AccessToken)
}
