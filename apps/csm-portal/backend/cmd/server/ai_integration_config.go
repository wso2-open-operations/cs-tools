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

package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// defaultAIIntegrationTimeout bounds one generate call to the AI integration
// service: its own model timeout (CLAUDE_TIMEOUT there, 45s by default) plus
// room for the network, and under REST_WRITE_TIMEOUT's 60s default.
const defaultAIIntegrationTimeout = 55 * time.Second

// aiIntegrationConfig configures the client for
// integrations/csm-ai-integration-service, which makes the dashboard
// generator's model calls. An empty BaseURL means the service is not used.
type aiIntegrationConfig struct {
	BaseURL string
	Scopes  []string
	Timeout time.Duration
}

// Enabled reports whether the service is configured.
func (c aiIntegrationConfig) Enabled() bool { return c.BaseURL != "" }

// loadAIIntegrationConfig reads CSM_AI_INTEGRATION_SERVICE_BASE_URL
// (optional), CSM_AI_INTEGRATION_SERVICE_SCOPES (comma separated) and
// CSM_AI_INTEGRATION_SERVICE_TIMEOUT (a Go duration, default
// defaultAIIntegrationTimeout, must be > 0). The base URL must be https, like
// every other upstream's, except for a local host (localhost, a loopback
// address, or a one-label name such as a compose service), which may be
// http: the local stack runs the service without TLS. Pure, so tests pass
// their own getenv.
func loadAIIntegrationConfig(getenv func(string) string) (aiIntegrationConfig, error) {
	timeout, err := durationEnv(getenv, "CSM_AI_INTEGRATION_SERVICE_TIMEOUT", defaultAIIntegrationTimeout)
	if err != nil {
		return aiIntegrationConfig{}, err
	}
	base := strings.TrimSpace(getenv("CSM_AI_INTEGRATION_SERVICE_BASE_URL"))
	if base != "" {
		if err := validateAIIntegrationBaseURL(base); err != nil {
			return aiIntegrationConfig{}, fmt.Errorf("CSM_AI_INTEGRATION_SERVICE_BASE_URL %w", err)
		}
	}
	return aiIntegrationConfig{BaseURL: base, Scopes: splitComma(getenv("CSM_AI_INTEGRATION_SERVICE_SCOPES")), Timeout: timeout}, nil
}

// validateAIIntegrationBaseURL accepts an https base URL (the same rules as
// every other upstream), or an http one only for a local host (isLocalHost).
func validateAIIntegrationBaseURL(value string) error {
	u, err := url.Parse(value)
	if err != nil {
		return errors.New("is not a valid URL")
	}
	if u.Scheme == "http" && isLocalHost(u.Hostname()) {
		u.Scheme = "https" // the remaining checks are the same
		return validateHTTPSBaseURL(u.String())
	}
	if err := validateHTTPSBaseURL(value); err != nil {
		return fmt.Errorf("%w (http is accepted only for a local host)", err)
	}
	return nil
}

// isLocalHost reports whether host is localhost, a loopback address, or a
// name with no dot (a compose service such as csm-ai-integration-service).
func isLocalHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return !strings.Contains(host, ".")
}
