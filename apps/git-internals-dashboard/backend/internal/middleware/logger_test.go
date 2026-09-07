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

package middleware

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestLoggerResponseWriterUnwrapsForResponseController guards
// AUDIT-FINDINGS A4: PostSyncRuns extends its write deadline via
// http.NewResponseController(w).SetWriteDeadline, but w is Logger's wrapped
// responseWriter by the time a handler sees it. Without an Unwrap() method,
// NewResponseController cannot reach the underlying connection and
// SetWriteDeadline silently no-ops, so a slow handler still gets cut off by
// the server's short WriteTimeout. This test proves the extension actually
// reaches the connection through Logger's wrapper.
func TestLoggerResponseWriterUnwrapsForResponseController(t *testing.T) {
	const shortServerTimeout = 100 * time.Millisecond
	const handlerDelay = 300 * time.Millisecond

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Errorf("SetWriteDeadline through Logger's wrapper: %v", err)
		}
		time.Sleep(handlerDelay) // longer than the server's own WriteTimeout
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{
		Handler:      handler,
		WriteTimeout: shortServerTimeout,
		ReadTimeout:  shortServerTimeout,
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("request failed (deadline extension did not take effect): %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Errorf("expected 200 \"ok\", got %d %q", resp.StatusCode, body)
	}
}
