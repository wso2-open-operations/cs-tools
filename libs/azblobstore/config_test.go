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

package azblobstore

import (
	"strings"
	"testing"
)

func TestNew_SignInSettings(t *testing.T) {
	const secret = "super-secret-value"
	for name, tc := range map[string]struct {
		cfg     Config
		wantErr string
	}{
		"no container":     {Config{AccountName: "a", AccountKey: "a2V5"}, "container is required"},
		"nothing":          {Config{Container: "c"}, "set a connection string"},
		"shared key":       {Config{Container: "c", AccountName: "a", AccountKey: "a2V5"}, ""},
		"entra app":        {Config{Container: "c", AccountName: "a", TenantID: "11111111-1111-1111-1111-111111111111", ClientID: "22222222-2222-2222-2222-222222222222", ClientSecret: secret}, ""},
		"entra no tenant":  {Config{Container: "c", AccountName: "a", ClientID: "x", ClientSecret: secret}, "needs a tenant id"},
		"entra no secret":  {Config{Container: "c", AccountName: "a", TenantID: "t", ClientID: "x"}, "needs a tenant id"},
		"entra no account": {Config{Container: "c", TenantID: "t", ClientID: "x", ClientSecret: secret}, "needs a tenant id"},
		"entra bad tenant": {Config{Container: "c", AccountName: "a", TenantID: "not a tenant!", ClientID: "x", ClientSecret: secret}, "invalid Entra app settings"},
	} {
		t.Run(name, func(t *testing.T) {
			s, err := New(tc.cfg)
			if tc.wantErr == "" {
				if err != nil || s == nil {
					t.Fatalf("New() = %v, %v", s, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the client secret leaked into the error: %v", err)
			}
		})
	}
}

func TestContainerURL(t *testing.T) {
	if got := containerURL(Config{AccountName: "acct", Container: "files"}); got != "https://acct.blob.core.windows.net/files" {
		t.Errorf("default URL = %q", got)
	}
	if got := containerURL(Config{AccountName: "acct", Container: "files", Endpoint: "http://127.0.0.1:10000/devstoreaccount1/"}); got != "http://127.0.0.1:10000/devstoreaccount1/files" {
		t.Errorf("endpoint override = %q", got)
	}
}

// A connection string wins over the other settings, so Azurite keeps working
// when an Entra app is also configured.
func TestNew_ConnectionStringWins(t *testing.T) {
	cs := "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"
	if _, err := New(Config{Container: "c", ConnectionString: cs, TenantID: "t"}); err != nil {
		t.Fatalf("New() = %v", err)
	}
}
