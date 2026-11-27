// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package jsonnum decodes source payloads keeping numbers as json.Number, so large numeric ids aren't rounded through float64.
package jsonnum

import (
	"bytes"
	"encoding/json"
)

// Unmarshal is json.Unmarshal with UseNumber, so invalid input still returns json.Unmarshal's own error.
func Unmarshal(raw []byte, v any) error {
	if !json.Valid(raw) {
		return json.Unmarshal(raw, v)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v)
}
