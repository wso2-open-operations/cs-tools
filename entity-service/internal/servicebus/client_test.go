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

import (
	"context"
	"strings"
	"testing"
)

func TestNewClient_RequiresConnectionString(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Error("NewClient with no connection string succeeded, want an error")
	}
}

const testConnectionString = "Endpoint=sb://sbns-example.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=a2V5"

func TestNewClient_ConnectionString(t *testing.T) {
	client, err := NewClient(Config{ConnectionString: testConnectionString})
	if err != nil {
		t.Fatalf("NewClient with a connection string: %v", err)
	}
	_ = client.Close(context.Background())
}

func TestNewClient_RejectsBadConnectionString(t *testing.T) {
	for name, cfg := range map[string]Config{
		"not a connection string": {ConnectionString: "not-a-connection-string"},
		"no key":                  {ConnectionString: "Endpoint=sb://sbns-example.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey"},
	} {
		_, err := NewClient(cfg)
		if err == nil {
			t.Errorf("%s: NewClient succeeded, want an error", name)
		} else if strings.Contains(err.Error(), "a2V5") {
			t.Errorf("%s: error %q leaks the shared access key", name, err)
		}
	}
}

func TestConfigHost(t *testing.T) {
	for name, tt := range map[string]struct {
		cfg  Config
		want string
	}{
		"connection string": {Config{ConnectionString: testConnectionString}, "sbns-example.servicebus.windows.net"},
		"no endpoint":       {Config{ConnectionString: "SharedAccessKeyName=x;SharedAccessKey=y"}, ""},
		"user information":  {Config{ConnectionString: "Endpoint=sb://user:hunter2@sbns-example.servicebus.windows.net:443/;SharedAccessKeyName=x;SharedAccessKey=y"}, "sbns-example.servicebus.windows.net"},
		"unparsable":        {Config{ConnectionString: "Endpoint=sb://%zz/;SharedAccessKey=y"}, ""},
	} {
		if got := tt.cfg.Host(); got != tt.want {
			t.Errorf("%s: Host() = %q, want %q", name, got, tt.want)
		}
	}
}
