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

	if t.Logger.IsTraceEnabled() && req.Body != nil {
		// Read only first 10KB for logging to avoid OOM
		const logLimit = 10240
		prefix := make([]byte, logLimit)
		n, err := io.ReadFull(req.Body, prefix)

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// Body is smaller than 10KB - log it all
			t.Logger.Trace("OUTGOING REQUEST DETAILS: headers=%v body=%s", req.Header, string(prefix[:n]))
			// Restore the body with what we read
			req.Body = io.NopCloser(bytes.NewReader(prefix[:n]))
		} else if err == nil {
			// Body is larger than 10KB - log prefix and preserve rest
			t.Logger.Trace("OUTGOING REQUEST DETAILS: headers=%v body=%s [truncated, showing first %d bytes]",
				req.Header, string(prefix), logLimit)
			// Reconstruct body: prefix + remaining unread stream
			req.Body = io.NopCloser(io.MultiReader(
				bytes.NewReader(prefix),
				req.Body, // remaining unread portion
			))
		} else {
			// Error reading - restore empty body
			t.Logger.Trace("OUTGOING REQUEST DETAILS: headers=%v body=<error reading: %v>", req.Header, err)
			req.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}
}

func (t *LoggingTransport) logResponse(resp *http.Response, duration time.Duration) {
	t.Logger.Debug("INCOMING RESPONSE: status=%s duration=%v", resp.Status, duration)

	if t.Logger.IsTraceEnabled() && resp.Body != nil {
		// Read only first 10KB for logging to avoid OOM
		const logLimit = 10240
		prefix := make([]byte, logLimit)
		n, err := io.ReadFull(resp.Body, prefix)

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// Body is smaller than 10KB - log it all
			t.Logger.Trace("INCOMING RESPONSE DETAILS: headers=%v body=%s", resp.Header, string(prefix[:n]))
			// Restore the body with what we read
			resp.Body = io.NopCloser(bytes.NewReader(prefix[:n]))
		} else if err == nil {
			// Body is larger than 10KB - log prefix and preserve rest
			t.Logger.Trace("INCOMING RESPONSE DETAILS: headers=%v body=%s [truncated, showing first %d bytes]",
				resp.Header, string(prefix), logLimit)
			// Reconstruct body: prefix + remaining unread stream
			resp.Body = io.NopCloser(io.MultiReader(
				bytes.NewReader(prefix),
				resp.Body, // remaining unread portion
			))
		} else {
			// Error reading - restore empty body
			t.Logger.Trace("INCOMING RESPONSE DETAILS: headers=%v body=<error reading: %v>", resp.Header, err)
			resp.Body = io.NopCloser(bytes.NewReader(nil))
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
