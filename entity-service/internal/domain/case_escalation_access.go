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

package domain

import "time"

// CaseDeescalatorGrant is one person who may de-escalate any case, granted by
// an admin (the stored role case_deescalator), besides the leads of the
// case's own ABT team.
type CaseDeescalatorGrant struct {
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	GrantedBy string    `json:"grantedBy"`
	GrantedOn time.Time `json:"grantedOn"`
	// Active is false when the holder is not active internal staff: the grant
	// is listed so it can be removed, but it lets nobody de-escalate.
	Active bool `json:"active"`
}

// CaseDeescalators is the response of GET /case-escalation-access/deescalators.
type CaseDeescalators struct {
	Deescalators []CaseDeescalatorGrant `json:"deescalators"`
}
