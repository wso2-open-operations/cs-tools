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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/handler"
)

// config is the validated runtime configuration. It is built once at startup by
// loadConfig, which fails on anything missing or malformed so a bad deploy never
// serves traffic.
type config struct {
	Port string

	// Entity-service access: base URL plus an OAuth2 client-credentials app (a
	// service identity; there is no end-user token on this public API).
	EntityBaseURL string
	TokenURL      string
	ClientID      string
	ClientSecret  string
	EntityScopes  []string

	// Timeouts are REST_READ_TIMEOUT, REST_WRITE_TIMEOUT and ENTITY_SERVICE_TIMEOUT.
	Timeouts timeouts

	// PineconeHost and PineconeKey enable semantic related-article search.
	// Both set or both empty.
	PineconeHost string
	PineconeKey  string
}

// loadConfig reads and validates the configuration through getenv.
func loadConfig(getenv func(string) string) (config, error) {
	var c config
	var errs []error
	req := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}

	c.EntityBaseURL = strings.TrimRight(req("ENTITY_SERVICE_BASE_URL"), "/")
	if c.EntityBaseURL != "" {
		if err := checkEntityURL("ENTITY_SERVICE_BASE_URL", c.EntityBaseURL); err != nil {
			errs = append(errs, err)
		}
	}
	c.ClientID = req("OAUTH2_CLIENT_ID")
	c.ClientSecret = req("OAUTH2_CLIENT_SECRET")
	c.TokenURL = req("OAUTH2_TOKEN_URL")
	if c.TokenURL != "" {
		if err := checkHTTPSURL("OAUTH2_TOKEN_URL", c.TokenURL); err != nil {
			errs = append(errs, err)
		}
	}
	c.EntityScopes = splitComma(getenv("ENTITY_SERVICE_SCOPES"))

	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be a plain port number (e.g. \"8080\"), got %q", port))
	}
	c.Port = port

	t, err := loadTimeouts(getenv)
	if err != nil {
		errs = append(errs, err)
	}
	c.Timeouts = t
	if err == nil && t.RESTWrite < handler.RelatedBudget {
		errs = append(errs, fmt.Errorf("REST_WRITE_TIMEOUT (%s) must be at least the related-articles budget (%s), or that route is cut off before it can answer",
			t.RESTWrite, handler.RelatedBudget))
	}

	c.PineconeHost = strings.TrimRight(strings.TrimSpace(getenv("PINECONE_HOST")), "/")
	c.PineconeKey = strings.TrimSpace(getenv("PINECONE_KEY"))
	if (c.PineconeHost == "") != (c.PineconeKey == "") {
		errs = append(errs, errors.New("PINECONE_HOST and PINECONE_KEY must be set together or both left empty"))
	}
	if c.PineconeHost != "" {
		if err := checkHTTPSURL("PINECONE_HOST", c.PineconeHost); err != nil {
			errs = append(errs, err)
		}
	}

	return c, errors.Join(errs...)
}

// parseAbsoluteURL parses raw as an absolute URL with a host.
func parseAbsoluteURL(key, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("%s must be an absolute http(s) URL, got %q", key, raw)
	}
	return u, nil
}

// checkHTTPSURL requires an absolute https URL with a host. Used for the token
// endpoint (client credentials travel to it) and the vector index (an API key
// travels to it): neither may be reached in clear text.
func checkHTTPSURL(key, raw string) error {
	u, err := parseAbsoluteURL(key, raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s must use https, got %q", key, raw)
	}
	return nil
}

// checkEntityURL requires an absolute https URL, except that plain http is
// allowed for a loopback host (localhost, 127.0.0.0/8, ::1) so local
// development against a local entity-service still works.
func checkEntityURL(key, raw string) error {
	u, err := parseAbsoluteURL(key, raw)
	if err != nil {
		return err
	}
	if u.Scheme == "https" || isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s must use https unless the host is loopback (localhost, 127.0.0.1, ::1), got %q", key, raw)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// Server limits that are not operator-tunable.
const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 15 * time.Second
	maxHeaderBytes    = 16 << 10
)
