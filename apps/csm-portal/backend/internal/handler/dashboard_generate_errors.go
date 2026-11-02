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
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// Reasons a generate call could not become a dashboard. writeGenerateError
// maps them to statuses; the text says what happened without the model's
// output, which may echo the user's prompt. The model call itself is the AI
// integration service's (serviceGenerateError turns its error codes into
// these).
var (
	// errGenerateCall: the call failed (the service or its model gateway
	// answered an error, the connection failed, or it timed out).
	errGenerateCall = errors.New("model call failed")
	// errGenerateRefused: the model refused.
	errGenerateRefused = errors.New("model refused the request")
	// errGenerateTruncated: the model's reply was cut off before finishing.
	errGenerateTruncated = errors.New("model reply was cut off")
	// errGenerateUnusable: the reply holds no usable dashboard (no JSON
	// object, or none of them is valid: wraps dashboard.ErrGeneratedInvalid).
	errGenerateUnusable = errors.New("model reply is not a usable dashboard")
)

// generateDeclinedError is the model answering, as the catalogue tells it
// to when a request cannot be met, with what is missing instead of a
// dashboard. Reason is the model's sentence.
type generateDeclinedError struct {
	Reason string
}

func (e *generateDeclinedError) Error() string {
	return "model declined: " + e.Reason
}

// generatedReply is the chat message shown with a generated dashboard.
func generatedReply(d dashboard.Dashboard) string {
	n := len(d.Widgets)
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return fmt.Sprintf("Here's %q, %d widget%s.", d.DisplayName, n, plural)
}
