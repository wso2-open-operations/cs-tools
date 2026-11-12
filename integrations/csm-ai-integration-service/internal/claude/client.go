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

// Package claude builds the Anthropic API client every model call in this
// service uses (POST /ai/dashboards/generate).
// It only sets the client up; what is sent and how the reply is read belong to
// each feature.
package claude

import (
	"net/http"
	"strconv"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// maxRetries is how often the SDK retries a failed call (a connection error,
// 408, 409, 429 or 5xx) before giving up. One retry absorbs a passing gateway
// hiccup; more would keep the user waiting several times Config.Timeout.
const maxRetries = 1

// maxRetryWait is the longest Retry-After the one retry waits for. The SDK
// waits whatever the gateway asks, with no cap: a 429 asking for a minute
// would sleep until the call's deadline and end as a timeout, after keeping
// the user waiting, instead of as the usage limit it is.
const maxRetryWait = 5 * time.Second

// Config is everything the client is built from. Nothing else is read: the
// SDK's own environment defaults (ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN,
// ANTHROPIC_CUSTOM_HEADERS, profiles, ...) are switched off, so a stray
// variable in the container cannot redirect calls or add a credential.
type Config struct {
	// APIKey is sent as the x-api-key header. A secret: never log it.
	APIKey string
	// BaseURL is an Anthropic-compatible gateway the calls go to instead of
	// the Anthropic API; empty means the Anthropic API. Requests go to
	// BaseURL + "/v1/messages".
	BaseURL string
	// Timeout bounds each attempt.
	Timeout time.Duration
}

// NewClient returns a client for cfg.
func NewClient(cfg Config) anthropic.Client {
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(cfg.APIKey),
		option.WithRequestTimeout(cfg.Timeout),
		option.WithMaxRetries(maxRetries),
		option.WithMiddleware(noLongRetryWait),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return anthropic.NewClient(opts...)
}

// noLongRetryWait stops the SDK's retry of a failed answer whose Retry-After
// (or Retry-After-Ms) is longer than maxRetryWait or would end after the
// call's deadline, by setting x-should-retry: false, the header the SDK reads
// before the status. The failure is then returned at once: a 429 is the usage
// limit, an overloaded gateway a failed call. A short wait, or none, keeps the
// one retry.
func noLongRetryWait(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	res, err := next(req)
	if err != nil || res == nil || res.StatusCode < http.StatusBadRequest {
		return res, err
	}
	wait, ok := retryAfter(res.Header)
	if !ok {
		return res, err
	}
	deadline, hasDeadline := req.Context().Deadline()
	if wait > maxRetryWait || (hasDeadline && time.Now().Add(wait).After(deadline)) {
		res.Header.Set("x-should-retry", "false")
	}
	return res, err
}

// retryAfter reads the wait a failed answer asks for, as the SDK does:
// Retry-After-Ms in milliseconds first, then Retry-After in seconds or as an
// HTTP date.
func retryAfter(h http.Header) (time.Duration, bool) {
	if v := h.Get("Retry-After-Ms"); v != "" {
		if ms, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(ms * float64(time.Millisecond)), true
		}
	}
	if v := h.Get("Retry-After"); v != "" {
		if s, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(s * float64(time.Second)), true
		}
		if t, err := time.Parse(time.RFC1123, v); err == nil {
			return time.Until(t), true
		}
	}
	return 0, false
}
