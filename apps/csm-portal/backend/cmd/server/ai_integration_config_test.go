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
	"slices"
	"testing"
	"time"
)

func aiEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLoadAIIntegrationConfig_UnsetIsOff(t *testing.T) {
	c, err := loadAIIntegrationConfig(aiEnv(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() || c.Timeout != defaultAIIntegrationTimeout {
		t.Errorf("config = %+v, want off with the default timeout", c)
	}
}

func TestLoadAIIntegrationConfig_ReadsEveryVariable(t *testing.T) {
	c, err := loadAIIntegrationConfig(aiEnv(map[string]string{
		"CSM_AI_INTEGRATION_SERVICE_BASE_URL": " https://ai.example.test/csm-ai ",
		"CSM_AI_INTEGRATION_SERVICE_SCOPES":   "a, b",
		"CSM_AI_INTEGRATION_SERVICE_TIMEOUT":  "50s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled() || c.BaseURL != "https://ai.example.test/csm-ai" || !slices.Equal(c.Scopes, []string{"a", "b"}) || c.Timeout != 50*time.Second {
		t.Errorf("config = %+v", c)
	}
}

func TestLoadAIIntegrationConfig_HTTPOnlyForALocalHost(t *testing.T) {
	for url, ok := range map[string]bool{
		"https://ai.example.test":                true,
		"http://csm-ai-integration-service:8080": true,
		"http://localhost:18090":                 true,
		"http://127.0.0.1:18090":                 true,
		"http://ai.example.test":                 false,
		"http://10.0.0.5:8080":                   false,
		"ftp://csm-ai-integration-service":       false,
		"https://user:pass@ai.example.test":      false,
		"http://user:pass@localhost:8080":        false,
		"https://ai.example.test/?q=1":           false,
		"not a url":                              false,
	} {
		_, err := loadAIIntegrationConfig(aiEnv(map[string]string{"CSM_AI_INTEGRATION_SERVICE_BASE_URL": url}))
		if (err == nil) != ok {
			t.Errorf("%q: err = %v, want ok=%v", url, err, ok)
		}
	}
}

func TestLoadAIIntegrationConfig_RejectsABadTimeout(t *testing.T) {
	for _, v := range []string{"soon", "0s", "-5s"} {
		if _, err := loadAIIntegrationConfig(aiEnv(map[string]string{"CSM_AI_INTEGRATION_SERVICE_TIMEOUT": v})); err == nil {
			t.Errorf("%q: no error", v)
		}
	}
}
