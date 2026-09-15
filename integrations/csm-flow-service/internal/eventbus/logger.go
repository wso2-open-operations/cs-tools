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

package eventbus

import (
	"fmt"
	"log/slog"
)

// logDebug and logError bridge kafka.Writer/kafka.Reader's Logger/ErrorLogger
// (a bare Printf-style func) to slog. Routing Logger to slog.Debug and
// ErrorLogger to slog.Error matches kafka-go's own severity split — genuine
// problems stay visible at the default Info level, routine protocol chatter is
// available only when explicitly enabling Debug.
func logDebug(msg string, args ...any) {
	slog.Debug(fmt.Sprintf(msg, args...))
}

func logError(msg string, args ...any) {
	slog.Error(fmt.Sprintf(msg, args...))
}
