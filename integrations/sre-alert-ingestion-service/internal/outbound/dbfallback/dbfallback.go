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

// Package dbfallback posts alerts that could not be written to Postgres to one Google Chat space: a DATABASE CONNECTION FAILURE card per outage, then each alert as a reply in its thread.
package dbfallback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"

	"sre-alert-ingestion-service/internal/model"
)

const (
	// maxPending caps alerts waiting to be posted; more are only counted and summarised in one reply.
	maxPending = 500
	// maxField truncates each kept alert field, in runes, so the queue and every reply stay small however large the alerts are.
	maxField = 200
	// sendGap spaces messages to stay under Chat's limit of about one message per second per space.
	sendGap = time.Second
	// sendAttempts and retryBaseDelay retry 429 and 5xx answers; other 4xx answers are not retried.
	sendAttempts   = 3
	retryBaseDelay = 500 * time.Millisecond
)

// entry is a bounded summary of one alert, never its full text.
type entry struct {
	source, requestID                                    string
	severity, service, metric, environment, category, id string
}

func newEntry(source, requestID string, a model.Alert) entry {
	return entry{
		source:      truncate(source, maxField),
		requestID:   truncate(requestID, maxField),
		severity:    truncate(a.Severity, maxField),
		service:     truncate(a.Service, maxField),
		metric:      truncate(a.MetricName, maxField),
		environment: truncate(a.Environment, maxField),
		category:    truncate(a.Category, maxField),
		id:          truncate(a.UniqueIdentifier, maxField),
	}
}

// Client posts one message at a time, sendGap apart; an outage gets one thread, which ends once Recovered is called and the queue drains.
type Client struct {
	logger  *slog.Logger
	url     string
	spaceID string
	host    string
	http    *http.Client
	gap     time.Duration

	mu         sync.Mutex
	pending    []entry
	dropped    int
	thread     string
	outages    int
	parentSent bool
	recovered  bool
	// closing makes the loop post everything still queued as one summary reply, so shutdown stays within Chat's quota.
	closing bool
	running bool
	idle    *sync.Cond
}

// New returns a Client for webhookURL, which must be https since it carries the space's key and token.
func New(logger *slog.Logger, webhookURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(webhookURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		// The URL is a credential, so it is never echoed back.
		return nil, errors.New("DB_FALLBACK_CHAT_WEBHOOK_URL must be an https Google Chat webhook URL")
	}
	// Replies join the thread named by the message's threadKey, or start one if it is gone.
	q := u.Query()
	q.Set("messageReplyOption", "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD")
	u.RawQuery = q.Encode()
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "pod"
	}
	// Redirects are never followed, so the key and token only go to the configured host.
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c := &Client{
		logger:  logger,
		url:     u.String(),
		spaceID: spaceID(webhookURL),
		host:    host,
		http:    client,
		gap:     sendGap,
	}
	c.idle = sync.NewCond(&c.mu)
	return c, nil
}

// Notify queues alerts for the Chat space without blocking.
func (c *Client) Notify(source, requestID string, alerts []model.Alert) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recovered = false
	for _, a := range alerts {
		if len(c.pending) >= maxPending {
			c.dropped++
			continue
		}
		c.pending = append(c.pending, newEntry(source, requestID, a))
	}
	if !c.running {
		c.running = true
		go c.loop()
	}
}

// Recovered marks the outage over, so the next failure opens a new thread; it is cheap enough to call after every stored batch.
func (c *Client) Recovered() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recovered = true
	if !c.running {
		c.endThread()
	}
}

func (c *Client) endThread() {
	c.thread, c.parentSent = "", false
}

// next picks the parent card, then one reply per queued alert, then a summary of any alerts past maxPending or left at shutdown; ok is false when nothing is left.
func (c *Client) next() (msg map[string]any, alerts int, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) == 0 && c.dropped == 0 {
		if c.recovered {
			c.endThread()
		}
		c.running = false
		c.idle.Broadcast()
		return nil, 0, false
	}
	if c.thread == "" {
		c.outages++
		c.thread = fmt.Sprintf("db-fallback-%s-%d-%d", c.host, time.Now().UnixMilli(), c.outages)
	}
	switch {
	case !c.parentSent:
		c.parentSent = true
		return failureCard(c.thread, time.Now()), 0, true
	case c.closing:
		n := len(c.pending) + c.dropped
		c.pending, c.dropped = nil, 0
		return summary(c.thread, n), n, true
	case len(c.pending) > 0:
		e := c.pending[0]
		c.pending = c.pending[1:]
		if len(c.pending) == 0 {
			c.pending = nil
		}
		return reply(c.thread, alertText(e)), 1, true
	default:
		n := c.dropped
		c.dropped = 0
		return summary(c.thread, n), n, true
	}
}

func (c *Client) loop() {
	for {
		msg, alerts, ok := c.next()
		if !ok {
			return
		}
		c.send(msg, alerts)
		// Always waited, shutdown included, so no burst can pass Chat's per-space quota.
		time.Sleep(c.gap)
	}
}

// Close posts what is still queued as one summary reply, at the usual pace, and waits for it or ctx; used on shutdown.
func (c *Client) Close(ctx context.Context) {
	c.mu.Lock()
	c.closing = true
	c.mu.Unlock()
	c.wait(ctx)
}

// wait blocks until nothing is queued or in flight, or ctx ends.
func (c *Client) wait(ctx context.Context) {
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
		c.logger.Warn("db fallback chat did not finish before shutdown", "chat_space_id", c.spaceID)
	}
}

func (c *Client) send(msg map[string]any, alerts int) {
	body, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("db fallback chat message could not be built", "alerts", alerts, "error", err)
		return
	}
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = retryBaseDelay
	_, err = backoff.Retry(context.Background(), func() (struct{}, error) {
		status, err := c.post(body)
		if err != nil && status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return struct{}{}, backoff.Permanent(err)
		}
		return struct{}{}, err
	}, backoff.WithBackOff(eb), backoff.WithMaxTries(sendAttempts))
	if err != nil {
		c.logger.Error("db fallback chat failed; these alerts were neither stored nor posted", "chat_space_id", c.spaceID, "alerts", alerts, "error", err)
		return
	}
	c.logger.Info("db fallback chat sent", "chat_space_id", c.spaceID, "alerts", alerts)
}

// post returns status 0 when the request never got a response.
func (c *Client) post(body []byte) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return 0, errors.New("build request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			// Strip url.Error's embedded URL so the webhook's key and token don't reach logs.
			return 0, fmt.Errorf("%s request failed: %w", uerr.Op, uerr.Err)
		}
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return resp.StatusCode, fmt.Errorf("status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return resp.StatusCode, nil
}

// spaceID names the space in logs without the URL's embedded credential.
func spaceID(webhookURL string) string {
	_, rest, ok := strings.Cut(webhookURL, "/spaces/")
	if !ok {
		return "unknown"
	}
	if id, _, ok := strings.Cut(rest, "/"); ok {
		return id
	}
	return "unknown"
}

// ist is a fixed UTC+5:30 zone, so the card's time doesn't depend on tzdata being in the image.
var ist = time.FixedZone("IST", 5*60*60+30*60)

// failureCard opens an outage's thread; every alert that could not be stored is posted as a reply to it.
func failureCard(thread string, started time.Time) map[string]any {
	return map[string]any{
		"thread": map[string]any{"threadKey": thread},
		"cardsV2": []map[string]any{{
			"cardId": thread,
			"card": map[string]any{
				"header": map[string]any{
					"title":    "<font color='#f70707'><b>DATABASE CONNECTION FAILURE</b></font>",
					"subtitle": "Alerting Component | started " + started.In(ist).Format("2006-01-02 15:04:05 IST"),
				},
				"sections": []map[string]any{{"widgets": []map[string]any{paragraph(
					"Alerts cannot be stored and no incidents will be created. Each alert is posted below in this thread.")}}},
			},
		}},
	}
}

// reply is a headerless card in thread, like core's Duplicate/OK replies.
func reply(thread, text string) map[string]any {
	return map[string]any{
		"thread": map[string]any{"threadKey": thread},
		"cardsV2": []map[string]any{{
			"cardId": thread,
			"card":   map[string]any{"sections": []map[string]any{{"widgets": []map[string]any{paragraph(text)}}}},
		}},
	}
}

// summary stands in for alerts not posted one by one; their raw bodies are in the ingestion logs.
func summary(thread string, n int) map[string]any {
	return reply(thread, fmt.Sprintf("<b>%d more alert(s) not stored.</b><br>Not posted one by one; see the ingestion logs.", n))
}

func paragraph(text string) map[string]any {
	return map[string]any{"textParagraph": map[string]any{"text": text}}
}

// alertText escapes every field since it comes from the webhook sender; empty fields are left out.
func alertText(e entry) string {
	var b strings.Builder
	b.WriteString("<b>Alert not stored.</b>")
	for _, f := range []struct{ name, value string }{
		{"Severity", e.severity},
		{"Service", e.service},
		{"Metric", e.metric},
		{"Environment", e.environment},
		{"Category", e.category},
		{"Source", e.source},
		{"Unique ID", e.id},
		{"Request ID", e.requestID},
	} {
		if f.value != "" {
			b.WriteString("<br>" + f.name + ": " + html.EscapeString(f.value))
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
