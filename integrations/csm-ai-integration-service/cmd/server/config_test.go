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
	"reflect"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// TestLoadConfigDefaults checks that with nothing set, model calls are off and
// the defaults apply.
func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() || c.Model != defaultClaudeModel || c.Timeout != defaultClaudeTimeout ||
		c.BaseURL != "" || c.IgnoredCatalogueVars != nil || c.EmailDomainModel != defaultEmailDomainModel {
		t.Errorf("defaults = %+v", c)
	}
	if c.GatewayHost() != "none" {
		t.Errorf("GatewayHost() = %q, want none with model calls off", c.GatewayHost())
	}
}

// TestLoadConfigReadsAndTrimsEveryVariable checks that every variable is read
// and trimmed.
func TestLoadConfigReadsAndTrimsEveryVariable(t *testing.T) {
	c, err := loadConfig(env(map[string]string{
		"ANTHROPIC_API_KEY":  " key\n",
		"CLAUDE_MODEL":       " claude-opus-5-5 ",
		"CLAUDE_TIMEOUT":     "90s",
		"CLAUDE_BASE_URL":    " https://gateway.example.test/v1/anthropic?x=1 ",
		"EMAIL_DOMAIN_MODEL": " claude-sonnet-5-5 ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := config{APIKey: "key", Model: "claude-opus-5-5", Timeout: 90 * time.Second,
		BaseURL: "https://gateway.example.test/v1/anthropic?x=1", EmailDomainModel: "claude-sonnet-5-5"}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("config = %+v, want %+v", c, want)
	}
	if !c.Enabled() {
		t.Error("Enabled() = false with a key")
	}
	if c.GatewayHost() != "gateway.example.test" {
		t.Errorf("GatewayHost() = %q, want the host only", c.GatewayHost())
	}
}

func TestLoadConfigNeedsTheGatewayWithAKey(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"key, no gateway":    {"ANTHROPIC_API_KEY": "key"},
		"key, blank gateway": {"ANTHROPIC_API_KEY": "key", "CLAUDE_BASE_URL": "   "},
	} {
		_, err := loadConfig(env(vars))
		if err == nil || !strings.Contains(err.Error(), "CLAUDE_BASE_URL is required") {
			t.Errorf("%s: err = %v, want the gateway required", name, err)
		}
		if err != nil && strings.Contains(err.Error(), "key\"") {
			t.Errorf("%s: error %q holds the key", name, err)
		}
	}
	// No key at all still starts (model calls off); a gateway without a key
	// is harmless and still checked.
	if c, err := loadConfig(env(map[string]string{"CLAUDE_BASE_URL": "https://gateway.example.test"})); err != nil || c.Enabled() {
		t.Errorf("gateway without a key: config %+v err %v, want off and no error", c, err)
	}
}

// TestLoadConfigRejectsBadValues checks that a bad timeout or gateway URL is an
// error that never mentions the key.
func TestLoadConfigRejectsBadValues(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"timeout not a duration": {"CLAUDE_TIMEOUT": "soon"},
		"timeout zero":           {"CLAUDE_TIMEOUT": "0s"},
		"timeout negative":       {"CLAUDE_TIMEOUT": "-1s"},
		"base url relative":      {"CLAUDE_BASE_URL": "/v1"},
		"base url other scheme":  {"CLAUDE_BASE_URL": "ftp://gateway.example.test"},
		"base url no host":       {"CLAUDE_BASE_URL": "https://"},
	} {
		if _, err := loadConfig(env(vars)); err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), "key") {
			t.Errorf("%s: error %q mentions the key", name, err)
		}
	}
}

// TestLoadConfigIgnoresTheOldCatalogueVariables checks that CATALOGUE_DIR and
// CATALOGUE_FILE, no longer read, do not fail the start and are named for the
// startup warning; blank ones are not.
func TestLoadConfigIgnoresTheOldCatalogueVariables(t *testing.T) {
	for name, tc := range map[string]struct {
		vars map[string]string
		want []string
	}{
		"directory": {map[string]string{"CATALOGUE_DIR": "/mnt/catalogue"}, []string{"CATALOGUE_DIR"}},
		"file":      {map[string]string{"CATALOGUE_FILE": "/mnt/catalogue.md"}, []string{"CATALOGUE_FILE"}},
		"both":      {map[string]string{"CATALOGUE_DIR": "/mnt/catalogue", "CATALOGUE_FILE": "/mnt/catalogue.md"}, []string{"CATALOGUE_DIR", "CATALOGUE_FILE"}},
		"blank":     {map[string]string{"CATALOGUE_DIR": "  ", "CATALOGUE_FILE": "\n"}, nil},
	} {
		cfg, err := loadConfig(env(tc.vars))
		if err != nil {
			t.Errorf("%s: err = %v, want none", name, err)
		}
		if !reflect.DeepEqual(cfg.IgnoredCatalogueVars, tc.want) {
			t.Errorf("%s: IgnoredCatalogueVars = %v, want %v", name, cfg.IgnoredCatalogueVars, tc.want)
		}
	}
}

func TestShutdownWaitsForAnInFlightModelCall(t *testing.T) {
	for model, want := range map[time.Duration]time.Duration{
		45 * time.Second: 60 * time.Second,
		90 * time.Second: 105 * time.Second,
	} {
		if got := shutdownTimeout(model); got != want {
			t.Errorf("shutdownTimeout(%v) = %v, want %v", model, got, want)
		}
	}
}

func TestWriteTimeoutLeavesRoomForTheModelCall(t *testing.T) {
	for model, want := range map[time.Duration]time.Duration{
		45 * time.Second: 2 * time.Minute,
		90 * time.Second: 2*time.Minute + 30*time.Second,
	} {
		if got := writeTimeout(model); got != want {
			t.Errorf("writeTimeout(%v) = %v, want %v", model, got, want)
		}
	}
}
