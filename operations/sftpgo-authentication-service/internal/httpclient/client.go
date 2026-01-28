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

package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
)

// LoggingTransport implements http.RoundTripper and logs requests and responses.
type LoggingTransport struct {
	Transport http.RoundTripper
	Logger    *log.AppLogger
}

// RoundTrip executes a single HTTP transaction and logs the details.
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log Request
	t.logRequest(req)

	resp, err := t.Transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		t.Logger.Error("OUTGOING REQUEST FAILED: duration=%v error=%v", duration, err)
		return nil, err
	}

	// Log Response
	t.logResponse(resp, duration)

	return resp, nil
}

func (t *LoggingTransport) logRequest(req *http.Request) {
	t.Logger.Debug("OUTGOING REQUEST: method=%s url=%s", req.Method, req.URL.String())

	if t.Logger.IsTraceEnabled() {
		var bodyBytes []byte
		if req.Body != nil {
			// Read full body
			bodyBytes, _ = io.ReadAll(req.Body)
			// Restore full body for transmission
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		// Log only first 10KB
		logLimit := 10240
		if len(bodyBytes) > logLimit {
			t.Logger.Trace("OUTGOING REQUEST DETAILS: headers=%v body=%s [truncated, total: %d bytes]",
				req.Header, string(bodyBytes[:logLimit]), len(bodyBytes))
		} else {
			t.Logger.Trace("OUTGOING REQUEST DETAILS: headers=%v body=%s", req.Header, string(bodyBytes))
		}
	}
}

func (t *LoggingTransport) logResponse(resp *http.Response, duration time.Duration) {
	t.Logger.Debug("INCOMING RESPONSE: status=%s duration=%v", resp.Status, duration)

	if t.Logger.IsTraceEnabled() {
		var bodyBytes []byte
		if resp.Body != nil {
			// Read full body
			bodyBytes, _ = io.ReadAll(resp.Body)
			// Restore full body for caller
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		// Log only first 10KB
		logLimit := 10240
		if len(bodyBytes) > logLimit {
			t.Logger.Trace("INCOMING RESPONSE DETAILS: headers=%v body=%s [truncated, total: %d bytes]",
				resp.Header, string(bodyBytes[:logLimit]), len(bodyBytes))
		} else {
			t.Logger.Trace("INCOMING RESPONSE DETAILS: headers=%v body=%s", resp.Header, string(bodyBytes))
		}
	}
}

// NewLoggingClient returns an http.Client configured with the LoggingTransport.
func NewLoggingClient(timeout time.Duration, logger *log.AppLogger) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &LoggingTransport{
			Transport: http.DefaultTransport,
			Logger:    logger,
		},
	}
}
