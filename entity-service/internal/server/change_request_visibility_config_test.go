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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// CRVisibilityFromConfig turns CR_STRICT_VISIBILITY_FROM into the policy every
// change request repository shares: unset is "no cutover" (every change request
// is legacy -- the safe default and the rollback), a value is its instant, and an
// unparsable value (Config.Validate refuses it at startup, so this is the
// belt-and-braces) fails CLOSED -- strict for every change request -- so a typo
// can never widen what a customer sees.
func TestCRVisibilityFromConfig(t *testing.T) {
	if got := CRVisibilityFromConfig(&config.Config{}); got.StrictFrom != nil {
		t.Fatalf("unset: StrictFrom = %v, want nil (no cutover)", got.StrictFrom)
	}
	got := CRVisibilityFromConfig(&config.Config{CRStrictVisibilityFromRaw: "2026-11-01T00:00:00Z"})
	if got.StrictFrom == nil || !got.StrictFrom.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("set: StrictFrom = %v, want 2026-11-01T00:00:00Z", got.StrictFrom)
	}
	bad := CRVisibilityFromConfig(&config.Config{CRStrictVisibilityFromRaw: "whenever"})
	if bad.StrictFrom == nil {
		t.Fatal("an unparsable value must fail closed (strict), not open (no cutover)")
	}
	// A change request is strict when created at or after the instant, so strict
	// for every change request that can exist means an instant before any of them:
	// a far-FUTURE instant would make every row legacy, the opposite.
	if !bad.StrictFrom.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("fail-closed instant = %v, want one before every change request that can exist", bad.StrictFrom)
	}
}
