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

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/constants"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
)

// LoggingTransport implements http.RoundTripper and logs requests and responses.
type LoggingTransport struct {
	// Transport is the underlying RoundTripper used to execute the request.
	Transport http.RoundTripper
	// Logger is the application-wide logger.
	Logger *log.AppLogger
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

// logRequest logs the standard details of an outgoing HTTP request.
func (t *LoggingTransport) logRequest(req *http.Request) {
	t.Logger.Debug("OUTGOING REQUEST: method=%s url=%s", req.Method, req.URL.String())

	if t.Logger.IsTraceEnabled() {
		t.Logger.Trace("OUTGOING REQUEST HEADERS: %v", sanitizeHeaders(req.Header))

		if req.Body != nil {
			const logLimit = 10240
			prefix := make([]byte, logLimit)
			n, err := io.ReadFull(req.Body, prefix)

			switch {
			case err == io.EOF || err == io.ErrUnexpectedEOF:
				t.Logger.Trace("OUTGOING REQUEST BODY: %s", string(prefix[:n]))
				req.Body = io.NopCloser(bytes.NewReader(prefix[:n]))
			case err == nil:
				t.Logger.Trace("OUTGOING REQUEST BODY (truncated, showing first %d bytes): %s", logLimit, string(prefix))
				req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), req.Body))
			default:
				t.Logger.Trace("OUTGOING REQUEST BODY (partial read, error: %v): %s", err, string(prefix[:n]))
				req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix[:n]), req.Body))
			}
		}
	}
}

// logResponse logs the details of an incoming HTTP response, including duration.
func (t *LoggingTransport) logResponse(resp *http.Response, duration time.Duration) {
	t.Logger.Debug("INCOMING RESPONSE: status=%s duration=%v", resp.Status, duration)

	if t.Logger.IsTraceEnabled() {
		t.Logger.Trace("INCOMING RESPONSE HEADERS: %v", sanitizeHeaders(resp.Header))

		if resp.Body != nil {
			const logLimit = 10240
			prefix := make([]byte, logLimit)
			n, err := io.ReadFull(resp.Body, prefix)

			switch {
			case err == io.EOF || err == io.ErrUnexpectedEOF:
				t.Logger.Trace("INCOMING RESPONSE BODY: %s", string(prefix[:n]))
				resp.Body = io.NopCloser(bytes.NewReader(prefix[:n]))
			case err == nil:
				t.Logger.Trace("INCOMING RESPONSE BODY (truncated, showing first %d bytes): %s", logLimit, string(prefix))
				resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), resp.Body))
			default:
				t.Logger.Trace("INCOMING RESPONSE BODY (partial read, error: %v): %s", err, string(prefix[:n]))
				resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix[:n]), resp.Body))
			}
		}
	}
}

// sanitizeHeaders returns a copy of the headers with the Authorization value redacted.
func sanitizeHeaders(h http.Header) http.Header {
	sanitized := h.Clone()
	if sanitized.Get(constants.HeaderAuthorization) != "" {
		sanitized.Set(constants.HeaderAuthorization, "[REDACTED]")
	}
	return sanitized
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
