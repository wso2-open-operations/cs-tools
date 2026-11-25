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

package servicebus

import "testing"

func TestFullyQualifiedNamespace(t *testing.T) {
	for in, want := range map[string]string{
		"sbns-example":                                "sbns-example.servicebus.windows.net",
		"sbns-example.servicebus.windows.net":         "sbns-example.servicebus.windows.net",
		" sb://sbns-example.servicebus.windows.net/ ": "sbns-example.servicebus.windows.net",
		"sbns-example.servicebus.usgovcloudapi.net":   "sbns-example.servicebus.usgovcloudapi.net",
	} {
		if got := FullyQualifiedNamespace(in); got != want {
			t.Errorf("FullyQualifiedNamespace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewClient_RequiresEveryCredential(t *testing.T) {
	full := Config{Namespace: "sbns-example", TenantID: "tenant", ClientID: "client", ClientSecret: "secret"}
	for name, cfg := range map[string]Config{
		"no namespace": {TenantID: full.TenantID, ClientID: full.ClientID, ClientSecret: full.ClientSecret},
		"no tenant":    {Namespace: full.Namespace, ClientID: full.ClientID, ClientSecret: full.ClientSecret},
		"no client":    {Namespace: full.Namespace, TenantID: full.TenantID, ClientSecret: full.ClientSecret},
		"no secret":    {Namespace: full.Namespace, TenantID: full.TenantID, ClientID: full.ClientID},
	} {
		if _, err := NewClient(cfg); err == nil {
			t.Errorf("%s: NewClient succeeded, want an error", name)
		}
	}
}
