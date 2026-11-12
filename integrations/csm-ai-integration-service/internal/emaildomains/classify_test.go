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

package emaildomains

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/claude"
)

// fakeDNS answers from maps; a missing name is not found.
type fakeDNS struct {
	mx    map[string]bool
	hosts map[string]bool
	fail  bool
}

func (f fakeDNS) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	name = fqdn(name)
	if f.fail {
		return nil, &net.DNSError{Err: "timeout", Name: name, IsTimeout: true}
	}
	if f.mx[name] {
		return []*net.MX{{Host: "mx." + name}}, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (f fakeDNS) LookupHost(_ context.Context, name string) ([]string, error) {
	name = fqdn(name)
	if f.fail {
		return nil, &net.DNSError{Err: "server misbehaving", Name: name, IsTemporary: true}
	}
	if f.hosts[name] {
		return []string{"192.0.2.1"}, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

// fqdn requires the trailing dot the classifier sends, and strips it.
func fqdn(name string) string {
	if !strings.HasSuffix(name, ".") {
		return "missing-trailing-dot"
	}
	return strings.TrimSuffix(name, ".")
}

// fakeModel answers every call with category, and records the user message.
type fakeModel struct {
	category string
	status   int
	calls    atomic.Int32
	lastUser atomic.Value
	lastRaw  atomic.Value
}

func (f *fakeModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	var req struct {
		Messages []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &req)
	f.lastRaw.Store(string(raw))
	f.lastUser.Store(req.Messages[0].Content[0].Text)
	w.Header().Set("Content-Type", "application/json")
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"down"}}`)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-haiku-5-5",
		"content":     []any{map[string]any{"type": "text", "text": fmt.Sprintf(`{"category":%q}`, f.category)}},
		"stop_reason": "end_turn", "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 200, "output_tokens": 10},
	})
}

func newTestClassifier(t *testing.T, dns Resolver, model *fakeModel) *Classifier {
	t.Helper()
	srv := httptest.NewServer(model)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test", BaseURL: srv.URL, Timeout: 5 * time.Second})
	return NewClassifier(NewLists(), dns, time.Second, &client, "claude-haiku-5-5", 5*time.Second)
}

func TestClassifyRules(t *testing.T) {
	model := &fakeModel{category: "corporate"}
	c := newTestClassifier(t, fakeDNS{mx: map[string]bool{"acme.example": true}}, model)
	for domain, want := range map[string]Classification{
		"wso2.com":       Internal,
		"Mail.WSO2.com.": Internal,
		"mailinator.com": Disposable,
		"gmail.com":      Personal,
		"gone.example":   Disposable, // no mail server and no address
	} {
		res, err := c.Classify(context.Background(), domain)
		if err != nil || res.Classification != want || res.Source != SourceRules {
			t.Errorf("%s: %+v %v, want %s by rules", domain, res, err, want)
		}
	}
	if model.calls.Load() != 0 {
		t.Errorf("model called %d times for rule answers", model.calls.Load())
	}
}

func TestClassifyAsksTheModelOnceAndSendsOnlyTheDomain(t *testing.T) {
	model := &fakeModel{category: "corporate"}
	c := newTestClassifier(t, fakeDNS{hosts: map[string]bool{"acme.example": true}}, model)
	for i, want := range []Source{SourceModel, SourceCache} {
		res, err := c.Classify(context.Background(), "acme.example")
		if err != nil || res.Classification != Corporate || res.Source != want {
			t.Fatalf("call %d: %+v %v, want CORPORATE from %s", i, res, err, want)
		}
	}
	if model.calls.Load() != 1 {
		t.Errorf("model calls = %d, want 1", model.calls.Load())
	}
	if got := model.lastUser.Load(); got != "Domain: acme.example" {
		t.Errorf("user message = %q, want the domain alone", got)
	}
	if raw := model.lastRaw.Load().(string); !strings.Contains(raw, `"output_config":{"format":{"schema"`) {
		t.Errorf("request has no structured output format: %s", raw)
	}
}

func TestClassifyDoesNotCacheUnknown(t *testing.T) {
	model := &fakeModel{category: "unknown"}
	c := newTestClassifier(t, fakeDNS{mx: map[string]bool{"odd.example": true}}, model)
	for range 2 {
		if res, err := c.Classify(context.Background(), "odd.example"); err != nil || res.Classification != Unknown {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if model.calls.Load() != 2 {
		t.Errorf("model calls = %d, want 2 (UNKNOWN is not cached)", model.calls.Load())
	}
}

func TestClassifyErrors(t *testing.T) {
	ctx := context.Background()
	reachable := fakeDNS{mx: map[string]bool{"acme.example": true}}

	if _, err := newTestClassifier(t, reachable, &fakeModel{}).Classify(ctx, "not a domain"); !errors.Is(err, ErrInvalidDomain) {
		t.Errorf("bad domain: %v", err)
	}
	failing := newTestClassifier(t, fakeDNS{fail: true}, &fakeModel{category: "corporate"})
	if res, err := failing.Classify(ctx, "acme.example"); err != nil || res.Classification != Corporate || res.Source != SourceModel {
		t.Errorf("DNS failing: %+v %v, want the model's answer, never DISPOSABLE", res, err)
	}
	off := NewClassifier(NewLists(), reachable, time.Second, nil, "", 0)
	if _, err := off.Classify(ctx, "acme.example"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no key: %v", err)
	}
	if res, err := off.Classify(ctx, "gmail.com"); err != nil || res.Classification != Personal {
		t.Errorf("no key, rule answer: %+v %v", res, err)
	}
	if _, err := newTestClassifier(t, reachable, &fakeModel{status: 500}).Classify(ctx, "acme.example"); !errors.Is(err, ErrCall) {
		t.Errorf("gateway 500: %v", err)
	}
	if _, err := newTestClassifier(t, reachable, &fakeModel{status: 429}).Classify(ctx, "acme.example"); !errors.Is(err, ErrUsageLimit) {
		t.Errorf("gateway 429: %v, want ErrUsageLimit", err)
	}
	if _, err := newTestClassifier(t, reachable, &fakeModel{category: "spam"}).Classify(ctx, "acme.example"); !errors.Is(err, ErrUnusable) {
		t.Errorf("unexpected category: %v", err)
	}
}

// TestTheCommittedListIsSafe checks the list loads and never names a free
// provider or a WSO2 domain: those rules run after it, so a listed one would
// abandon real signups.
func TestTheCommittedListIsSafe(t *testing.T) {
	l := NewLists()
	if l.Size() < 1000 || !l.Disposable("mailinator.com") {
		t.Fatalf("list has %d domains; want the committed list loaded", l.Size())
	}
	for d := range freeProviders {
		if l.Disposable(d) {
			t.Errorf("free provider %s is on the disposable list", d)
		}
	}
	for d := range l.disposable {
		if isInternal(d) {
			t.Errorf("WSO2 domain %s is on the disposable list", d)
		}
		if _, err := Normalize(d); err != nil {
			t.Errorf("list entry %q is not a valid domain", d)
		}
	}
}

func TestCacheExpiresAndStaysBounded(t *testing.T) {
	c := newCache(time.Hour)
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	c.set("a.example", Corporate)
	if got, ok := c.get("a.example"); !ok || got != Corporate {
		t.Fatal("fresh entry missing")
	}
	now = now.Add(2 * time.Hour)
	if _, ok := c.get("a.example"); ok {
		t.Error("expired entry returned")
	}
	for i := range maxCached + 5 {
		c.set(fmt.Sprintf("d%d.example", i), Personal)
	}
	if len(c.m) > maxCached {
		t.Errorf("cache holds %d, over %d", len(c.m), maxCached)
	}
}
