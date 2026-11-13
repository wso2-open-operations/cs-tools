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

package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// A gateway error's text can echo the request; only its status may be logged.
func TestModelErrorForLogKeepsTheGatewayReplyOut(t *testing.T) {
	gateway := fmt.Errorf("model call failed: %w", &anthropic.Error{StatusCode: 502})
	for err, want := range map[error]string{
		gateway:                            "gateway status 502",
		context.DeadlineExceeded:           "timeout",
		errors.New("acme.example refused"): "call failed",
	} {
		if got := modelErrorForLog(err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}
