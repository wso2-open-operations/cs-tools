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
	"os"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func TestEmailReplyTo(t *testing.T) {
	t.Run("unset follows the support address", func(t *testing.T) {
		os.Unsetenv("EMAIL_REPLY_TO")
		if got := emailReplyTo(); got != notifications.SupportAddress() {
			t.Errorf("emailReplyTo() = %q, want the support address %q", got, notifications.SupportAddress())
		}
	})
	t.Run("set wins", func(t *testing.T) {
		t.Setenv("EMAIL_REPLY_TO", "replies@example.com")
		if got := emailReplyTo(); got != "replies@example.com" {
			t.Errorf("emailReplyTo() = %q", got)
		}
	})
	t.Run("set but empty means none", func(t *testing.T) {
		t.Setenv("EMAIL_REPLY_TO", "")
		if got := emailReplyTo(); got != "" {
			t.Errorf("emailReplyTo() = %q, want empty", got)
		}
	})
}
