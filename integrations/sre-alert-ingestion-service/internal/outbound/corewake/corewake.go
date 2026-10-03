// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package corewake nudges sre-alert-core-service to poll now instead of at its next tick; best effort, failures are only logged.
package corewake

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Client sends POST <url> with at most one call in flight per replica, coalescing wakes that arrive mid-call into a single follow-up.
type Client struct {
	logger   *slog.Logger
	url      string
	username string
	secret   string
	secure   bool
	http     *http.Client
	mu       sync.Mutex
	running  bool
	pending  bool
	idle     *sync.Cond
}

// New returns a Client; an empty url logs a warning and makes Wake a no-op (local dev). Credentials are only attached over https.
func New(logger *slog.Logger, wakeURL, username, secret string, timeout time.Duration) *Client {
	secure := false
	if wakeURL == "" {
		logger.Warn("ALERT_CORE_WAKE_URL not set; alerts-core will pick alerts up on its own poll")
	} else {
		if u, err := url.Parse(wakeURL); err == nil && u.Scheme == "https" {
			secure = true
		}
		switch {
		case username == "" || secret == "":
			logger.Warn("ALERT_CORE_WAKE_USERNAME/ALERT_CORE_WAKE_SECRET not set; wake calls will be unauthenticated")
		case !secure:
			logger.Warn("ALERT_CORE_WAKE_URL is not https; wake calls will be sent without credentials")
		}
	}
	// Redirects are never followed, since Go keeps Authorization even on an https to http redirect.
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c := &Client{logger: logger, url: wakeURL, username: username, secret: secret, secure: secure, http: client}
	c.idle = sync.NewCond(&c.mu)
	return c
}

// Wake requests a wake-up without blocking.
func (c *Client) Wake() {
	if c.url == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		c.pending = true
		return
	}
	c.running = true
	go c.loop()
}

func (c *Client) loop() {
	for {
		c.send()
		c.mu.Lock()
		if !c.pending {
			c.running = false
			c.idle.Broadcast()
			c.mu.Unlock()
			return
		}
		c.pending = false
		c.mu.Unlock()
	}
}

func (c *Client) send() {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.url, http.NoBody)
	if err != nil {
		c.logger.Warn("alerts-core wake-up request invalid", "error", err)
		return
	}
	if c.secure && c.username != "" && c.secret != "" {
		token := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.secret))
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("alerts-core wake-up failed; its poll will pick the alerts up", "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		c.logger.Warn("alerts-core wake-up returned unexpected status", "status", resp.StatusCode)
	}
}

// Wait blocks until no call is in flight or ctx ends; used on shutdown.
func (c *Client) Wait(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		c.mu.Lock()
		for c.running {
			c.idle.Wait()
		}
		c.mu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
