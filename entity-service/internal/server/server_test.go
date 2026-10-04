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

package server

import (
	"testing"
	"time"
)

// TestServerWriteTimeoutOutlastsTheRequestTimeout pins the ordering of the
// two deadlines: a handler cancelled by the request timeout must still be able
// to write its error response, and a handler finishing just inside it must
// not lose a successful response to the connection's write deadline.
func TestServerWriteTimeoutOutlastsTheRequestTimeout(t *testing.T) {
	const margin = 5 * time.Second
	if serverWriteTimeout < requestTimeout+margin {
		t.Fatalf("serverWriteTimeout = %v, want at least requestTimeout (%v) + %v", serverWriteTimeout, requestTimeout, margin)
	}
}
