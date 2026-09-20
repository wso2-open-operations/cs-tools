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

package productconsumption

import (
	"encoding/json"
	"fmt"
	"math"
)

// jsonInt is an int that also accepts a JSON number written with a decimal
// point — 5.0 as well as 5.
//
// The product-consumption service sends its status that way, and Go's
// encoding/json refuses 5.0 into an int outright: "field \"result.status\"
// expects int, got number 5.0". The whole response then fails to decode, so a
// project that is fully provisioned looks identical to one the service has
// never heard of.
//
// The Ballerina backend declares the same field as int and does NOT hit this,
// because its data binding converts a float to an int when the conversion is
// lossless. The declarations match; the runtimes do not. That difference is
// invisible until the upstream happens to serialise a whole number as a float,
// which is exactly what happened here.
//
// A non-integral value is still an error: 5.5 is not a status, and silently
// truncating it would turn a genuinely malformed response into a plausible
// one.
type jsonInt int

func (n *jsonInt) UnmarshalJSON(b []byte) error {
	// encoding/json calls UnmarshalJSON for a JSON null too, and the documented
	// convention is to leave the value alone rather than error.
	if string(b) == "null" {
		return nil
	}

	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return fmt.Errorf("productconsumption: %v is not a whole number", f)
	}
	*n = jsonInt(f)
	return nil
}
