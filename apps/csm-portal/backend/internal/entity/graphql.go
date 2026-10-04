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

package entity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
}

// graphqlEnvelope is the standard GraphQL response shape: a "data" object
// alongside an optional "errors" array, both inside a single HTTP 200 OK —
// unlike this package's REST clients (CustomerEntityClient,
// EngineeringEntityClient), a GraphQL service reports query-level failures
// in the body, not via HTTP status.
type graphqlEnvelope[T any] struct {
	Data   T              `json:"data"`
	Errors []graphqlError `json:"errors,omitempty"`
}

// doGraphQL executes a GraphQL query against baseURL using httpClient
// (expected to already carry OAuth2 client-credentials auth, as built by
// clientcredentials.Config.Client — see SalesEntityClient) and decodes the
// response's "data" field into T.
//
// A non-2xx HTTP status is reported as an *apierror.Error carrying the real
// status code, exactly like this package's REST clients. A 200 OK response
// that carries a non-empty "errors" array is also reported as an
// *apierror.Error, with StatusCode http.StatusBadGateway — there is no
// natural HTTP status for a GraphQL-level error, and 502 lets the existing
// mapUpstreamError/mapUpstreamErrorGeneric handler helpers treat it the same
// way they already treat any other upstream failure, with no new plumbing
// required in callers.
func doGraphQL[T any](ctx context.Context, httpClient *http.Client, baseURL, query string, variables map[string]any, errLabel string) (T, error) {
	var zero T

	reqBody, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return zero, fmt.Errorf("%s: encode request: %w", errLabel, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return zero, fmt.Errorf("%s: build request: %w", errLabel, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", errLabel, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return zero, fmt.Errorf("%s: read response body: %w", errLabel, err)
	}
	if len(respBody) > maxUpstreamResponseBytes {
		return zero, fmt.Errorf("%s: read response body: response exceeds %d bytes", errLabel, maxUpstreamResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return zero, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	var envelope graphqlEnvelope[T]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("%s: decode response: %w", errLabel, err)
	}

	if len(envelope.Errors) > 0 {
		return zero, &apierror.Error{StatusCode: http.StatusBadGateway, Body: envelope.Errors[0].Message}
	}

	return envelope.Data, nil
}
