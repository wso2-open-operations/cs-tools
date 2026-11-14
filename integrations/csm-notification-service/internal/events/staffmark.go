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

package events

import "regexp"

// staffMarkInString matches the circled W (U+24E6, or the capital U+24CC) the
// data appends to the display name of many users, with the spaces before it,
// when it ends a JSON string value: raw UTF-8, or a \uXXXX escape. A circled W
// anywhere else in a string is left alone.
var staffMarkInString = regexp.MustCompile(
	`(?:\s|\\u0020)*(?:[\x{24CC}\x{24E6}]|\\u24[eE]6|\\u24[cC][cC])(?:\s|\\u0020)*"`,
)

// StripStaffMark removes the circled W from the end of every string in a JSON
// payload, so no name in an email or a chat card carries it. It runs on the
// raw payload before any handler decodes it, because names reach the handlers
// in dozens of fields.
func StripStaffMark(payload []byte) []byte {
	return staffMarkInString.ReplaceAll(payload, []byte(`"`))
}
