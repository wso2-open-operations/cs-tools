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

package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// snFlexibleInt decodes a numeric choice-list id that the upstream sends
// either bare (3) or quoted ("3"). A plain int field makes encoding/json abort
// the whole response on the first quoted id, failing every row of a page for
// one inconsistently encoded value. An empty string or null decodes to 0, as
// a missing field would.
type snFlexibleInt int

func (f *snFlexibleInt) UnmarshalJSON(data []byte) error {
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		if n == "" { // null
			*f = 0
			return nil
		}
		v, err := strconv.Atoi(n.String())
		if err != nil {
			return fmt.Errorf("choice id %s is not an integer", n)
		}
		*f = snFlexibleInt(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("choice id %s is neither a number nor a string", data)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("choice id %q is not an integer", s)
	}
	*f = snFlexibleInt(v)
	return nil
}
