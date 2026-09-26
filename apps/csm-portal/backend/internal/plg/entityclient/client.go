// Package entityclient is the BFF's data layer: it implements the repository
// interfaces the services are written against, over HTTP to entity-service
// rather than over a connection pool.
//
// This package is the seam between the two services. Every lifecycle rule,
// bookend and validation stays above it, in the handlers and services; all that
// changes below it is that a call crosses a network.
//
// It is also where the contract's central asymmetry is resolved. entity-service
// answers a guarded write with a row count and no opinion: RowsAffected 0 means
// "the precondition no longer held", and nothing more. Deciding that a failed
// acknowledgement is a 409, and a failed note edit is a 403, is a PLG rule — so
// it happens here, in the BFF, which is where PLG's rules live. Read the
// translations below as the BFF making those calls, not as error plumbing.
package entityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
)

// maxResponseBytes caps what one reply may be. Generous — a work queue of 500
// pairings with their runs is a large document — but not unbounded: a BFF that
// will read any length is one a misbehaving upstream can exhaust.
const maxResponseBytes = 16 << 20

// Client talks to entity-service's /plg surface.
type Client struct {
	http    *http.Client
	baseURL string
}

// Config holds what the client needs to reach entity-service.
type Config struct {
	BaseURL string
	Timeout time.Duration

	// HTTPClient, when set, is used verbatim — it carries entity-service's
	// OAuth2 credentials. Nil builds a plain client, which is right only where
	// entity-service needs no token.
	HTTPClient *http.Client
}

// New builds a Client.
//
// No authentication. entity-service has none — its own security is network
// placement, not credentials — so there is nothing here to send. When the slice
// merges and sits behind the Choreo gateway, this is where a client-credentials
// token would be attached, and the shape of the call sites would not change.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do issues one request and decodes the reply into out.
//
// A non-2xx is translated into the same apierror types the services already
// handle, so a 404 from entity-service surfaces to the caller as a 404 rather
// than as "internal error" — the status and message travel intact rather than
// being flattened at the boundary.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("entity: encode %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("entity: build %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	// Forward the correlation id so one request reads as one line in both
	// services' logs. entity-service generates its own when absent, which would
	// break the chain at exactly the hop worth tracing.
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set(correlationIDHeader, id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// A refused connection or a timeout is the downstream being unavailable,
		// not the caller being wrong. 503 says so.
		return &apierror.ServiceUnavailableError{
			Msg: "the entity service is not reachable",
		}
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("entity: read %s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return translateStatus(resp.StatusCode, payload)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("entity: decode %s %s: %w", method, path, err)
	}
	return nil
}

// errorBody is entity-service's error shape: {"code":400,"message":"..."}.
type errorBody struct {
	Message string `json:"message"`
}

// translateStatus turns entity-service's status into this service's error type.
//
// The message is passed through rather than replaced. entity-service's
// validation messages name the field that was wrong — "filters.ownerIds[0] must
// be a UUID" — and rewriting that into something generic at the boundary would
// throw away the only part a caller can act on.
func translateStatus(status int, payload []byte) error {
	var body errorBody
	_ = json.Unmarshal(payload, &body)
	msg := body.Message
	if msg == "" {
		msg = http.StatusText(status)
	}
	switch status {
	case http.StatusBadRequest:
		return &apierror.ValidationError{Msg: msg}
	case http.StatusNotFound:
		return &apierror.NotFoundError{Msg: msg}
	case http.StatusConflict:
		return &apierror.ConflictError{Msg: msg}
	case http.StatusForbidden:
		return &apierror.ForbiddenError{Msg: msg}
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusBadGateway:
		return &apierror.ServiceUnavailableError{Msg: "the entity service is temporarily unavailable"}
	default:
		// 500s and anything unexpected become an opaque server error: the
		// caller cannot act on entity-service's internals and should not see
		// them.
		return fmt.Errorf("entity service returned %d: %s", status, msg)
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) patch(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

func (c *Client) delete(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

// esc escapes a path segment. Every id reaching these methods has been
// validated as a UUID or a known code, so this is belt-and-braces — but a path
// built by concatenation is exactly where that assumption stops being checked.
func esc(s string) string { return url.PathEscape(s) }

// writeResult is the shape every guarded write answers with.
type writeResult struct {
	RowsAffected   int    `json:"rowsAffected"`
	OrganizationID string `json:"organizationId"`
	ProductCode    string `json:"productCode"`
}
