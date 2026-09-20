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

package escalation

import "context"

// StaticResolver returns a fixed set of recipients per level, ignoring the
// routing context entirely.
//
// This is the test-flow resolver: it lets the whole ladder be exercised
// end to end against dummy users before the organisation data behind the real
// rule table exists — exactly the setup the administrative guide asks for when
// it says to create a test/dummy user rather than configuring a real account.
//
// It is NOT an implementation of section 5.0's rule table and must not be used
// in production: it cannot distinguish an Integration incident from an IAM one,
// or a rotation from business hours.
type StaticResolver struct {
	// ByLevel maps each level to its recipients. A missing level resolves to
	// nobody, which BuildPlan records as a NO_RECIPIENTS issue and skips.
	ByLevel map[Level][]Recipient
}

// Resolve implements Resolver.
func (s StaticResolver) Resolve(_ context.Context, level Level, _ RoutingContext) ([]Recipient, error) {
	return s.ByLevel[level], nil
}
