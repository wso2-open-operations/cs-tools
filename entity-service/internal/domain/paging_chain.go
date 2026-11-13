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

// Case Paging: the system pages people about an incident tier by tier until
// someone acknowledges. These types back the Team Schedule's Case Paging tab,
// where the people on each tier are picked. They add nothing of their own: a
// tier's people are existing team_member rows -- the three responders of Tier 1 are
// the alert_tier slots, a Team lead is role lead, the America lead (the one
// above the Americas team's three Team leads) is americas_team_lead, and the
// heads are cre_head and cs_head.
//
// The word "escalation" is kept for the customer feature and does not appear
// here.

// PagingChainMember is one membership as the Case Paging tab shows it.
type PagingChainMember struct {
	// MembershipID is team_member.id -- what an edit addresses.
	MembershipID string `json:"membershipId"`
	TeamKey      string `json:"teamKey"`
	TeamName     string `json:"teamName"`
	TeamType     string `json:"teamType"`
	// Family is CRE or SRE, the same split the Team Schedule draws.
	Family string `json:"family"`
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	// Role is the raw team_member.role: engineer, lead, americas_team_lead,
	// cre_head or cs_head.
	Role string `json:"role"`
	// ResponderRank is 1, 2 or 3 for the 1st, 2nd and 3rd responder of the
	// team's Tier 1, 0 for none. Stored as alert_tier T1-T3.
	ResponderRank int `json:"responderRank"`
	// CanEditPhone is whether the caller may set this person's paging-only
	// phone number (see PagingPhone).
	CanEditPhone bool `json:"canEditPhone"`
	// PagingPhone is the person's paging-only number; null when none is set.
	PagingPhone *PagingPhone `json:"pagingPhone"`
	// HasProfilePhone is whether the person has a callable number on their
	// own profile ("user".phone), which paging calls ahead of PagingPhone.
	// Absent when the paging numbers are not configured.
	HasProfilePhone *bool `json:"hasProfilePhone,omitempty"`
	// CanTestCall is whether the caller may place a test call to this
	// person: a portal admin, or the rota admin of a family one of the
	// person's teams is in.
	CanTestCall bool `json:"canTestCall"`
	// TestCall is the number a test call would ring -- the one Case Alerts
	// dials from CSM -- and the last test of it. Only for a caller who may
	// test it; absent when CSM holds no number to dial.
	TestCall *PagingTestCall `json:"testCall,omitempty"`
}

// PagingTestCall is the number a test call rings and its last test.
type PagingTestCall struct {
	// Masked keeps the country code and the last three digits.
	Masked string `json:"masked"`
	// Source is "profile" (the number on the person's own profile) or
	// "paging" (their paging-only number), as in GET .../paging-contacts.
	Source string `json:"source"`
	// LastTestAt and LastTestStatus are the last test call of this number:
	// pending, completed, no-answer, busy or failed. Absent until one is
	// requested, and after the number changes.
	LastTestAt     *string `json:"lastTestAt,omitempty"`
	LastTestStatus *string `json:"lastTestStatus,omitempty"`
}

// PagingPhone is a paging-only phone number: the one paging calls when the
// person's Asgardeo profile has none. Never written to Asgardeo.
type PagingPhone struct {
	// Masked keeps the country code and the last three digits:
	// "+94•••••123". Always present.
	Masked string `json:"masked"`
	// Phone is the full number, E.164; only for a caller who may edit it.
	Phone string `json:"phone,omitempty"`
	// SetBy is the email of who last set it; SetAt when, RFC3339.
	SetBy string `json:"setBy"`
	SetAt string `json:"setAt"`
	// LastTestAt and LastTestStatus are the last test call: pending,
	// completed, no-answer, busy or failed. Absent until one is requested.
	LastTestAt     *string `json:"lastTestAt,omitempty"`
	LastTestStatus *string `json:"lastTestStatus,omitempty"`
}

// SetPagingPhoneRequest is PUT /team-schedule/paging-contacts/{userId}.
type SetPagingPhoneRequest struct {
	UserID string `json:"-"`
	// Phone is E.164: a +, then 7 to 15 digits, the first not 0.
	Phone string `json:"phone"`
}

// PagingTestCallResponse is POST .../{userId}/test.
type PagingTestCallResponse struct {
	LastTestStatus string `json:"lastTestStatus"`
}

// PagingTestResultRequest is PUT .../{userId}/test-result, from the service
// that placed the call.
type PagingTestResultRequest struct {
	UserID string `json:"-"`
	// Status is completed, no-answer, busy or failed.
	Status string `json:"status"`
	// TestedAt is when the call ended, RFC3339.
	TestedAt string `json:"testedAt"`
}

// PagingContact is one person's numbers, as the service that pages reads
// them, for their email: the number to call (DialPhone), the number on their
// own profile and their paging-only number.
type PagingContact struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	// DialPhone is the number to call: the profile number when it is a
	// callable E.164 number, else the paging-only number; "" when neither.
	// DialSource says which: "profile" or "paging" ("" with no number).
	DialPhone  string `json:"dialPhone"`
	DialSource string `json:"dialSource"`
	// ProfilePhone is the number on the person's own profile ("user".phone),
	// as stored; omitted when blank.
	ProfilePhone string `json:"profilePhone,omitempty"`
	// Phone is the paging-only number; "" when none is stored. SetBy, SetAt
	// and the test fields are about it, and omitted with it.
	Phone          string  `json:"phone"`
	SetBy          string  `json:"setBy,omitempty"`
	SetAt          string  `json:"setAt,omitempty"`
	LastTestAt     *string `json:"lastTestAt,omitempty"`
	LastTestStatus *string `json:"lastTestStatus,omitempty"`
}

// PagingContactsResponse is GET /team-schedule/paging-contacts.
type PagingContactsResponse struct {
	Contacts []PagingContact `json:"contacts"`
}

// PagingChainPermissions is what the caller may change, so the tab can lock
// what they cannot. The server checks every edit again regardless.
type PagingChainPermissions struct {
	// ResponderTeams are the teams whose 1st-3rd responders the caller may pick.
	ResponderTeams []string `json:"responderTeams"`
	// TeamLeadTeams are the teams whose Team leads the caller may set.
	TeamLeadTeams []string `json:"teamLeadTeams"`
	// AmericasTeamLead is whether the caller may set the America lead.
	AmericasTeamLead bool `json:"americasTeamLead"`
	// Heads is whether the caller may set the CRE head and CS head.
	Heads bool `json:"heads"`
}

// PagingChainResponse is one family's paging chain people, and what the
// caller may change.
type PagingChainResponse struct {
	Family  string                 `json:"family"`
	Members []PagingChainMember    `json:"members"`
	Count   int                    `json:"count"`
	CanEdit PagingChainPermissions `json:"canEdit"`
}

// UpdatePagingMemberRequest changes exactly one thing about one membership.
type UpdatePagingMemberRequest struct {
	MembershipID string `json:"-"`
	// ResponderRank 1-3 makes this person that responder of their team's
	// Tier 1, moving whoever held it off; 0 clears theirs.
	ResponderRank *int `json:"responderRank,omitempty"`
	// Role sets team_member.role: lead, engineer, americas_team_lead,
	// cre_head or cs_head. A new America lead, CRE head or CS head replaces
	// the previous one, who steps back to lead or engineer.
	Role *string `json:"role,omitempty"`
}

// PagingReadinessResponse answers "would each paging chain reach someone over
// the next days": for each chain, what is missing and who is on it. Computed
// on read from the memberships and the rota; nothing of it is stored.
//
// generatedAt keeps the At suffix the Case Paging tab was built against,
// unlike the On suffix the entity responses use.
type PagingReadinessResponse struct {
	GeneratedAt string                 `json:"generatedAt"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Chains      []PagingReadinessChain `json:"chains"`
}

// PagingReadinessChain is one chain: CRE, SRE_SAAS, SRE_IAAS, SRE_U2,
// SRE_B_CENTRAL (the last two only once their rotas are SRE rotas) or SME. Ready is
// true when it has no gap of severity error; a warning does not stop a page.
type PagingReadinessChain struct {
	Chain  string                  `json:"chain"`
	Label  string                  `json:"label"`
	Ready  bool                    `json:"ready"`
	Gaps   []PagingReadinessGap    `json:"gaps"`
	People []PagingReadinessPerson `json:"people"`
}

// PagingReadinessGap is one thing that would stop a page reaching someone.
// Code is stable for a client to branch on; Message is one sentence for a
// person. Fix names where it is fixed: responders (the Case Paging tab), rota
// (the Team Schedule roster), profile, config (deployment configuration) or
// data (records synced from elsewhere). The locating fields are set only
// where they apply.
type PagingReadinessGap struct {
	Code      string `json:"code"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Fix       string `json:"fix"`
	TeamKey   string `json:"teamKey,omitempty"`
	Date      string `json:"date,omitempty"`
	ZoneCode  string `json:"zoneCode,omitempty"`
	Tier      string `json:"tier,omitempty"`
	ShiftCode string `json:"shiftCode,omitempty"`
}

// PagingReadinessPerson is someone the chain would page. Role says why, for
// display: a position on the CRE chain ("T1 responder · Vega", "CRE head"),
// the tiers held on an SRE rota ("L1, L2"), the SME team rostered on.
type PagingReadinessPerson struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	// HasPagingPhone is whether a paging-only number is stored for them;
	// PagingPhoneLastTestStatus is its last test call's outcome, if any.
	// The number itself is never part of readiness.
	HasPagingPhone            bool    `json:"hasPagingPhone"`
	PagingPhoneLastTestStatus *string `json:"pagingPhoneLastTestStatus,omitempty"`
	// HasProfilePhone is whether they have a callable number on their own
	// profile ("user".phone), which paging calls ahead of the paging-only one.
	HasProfilePhone bool `json:"hasProfilePhone"`
	// PagingTier is the Case Paging tier at which the chain first calls this
	// person (1 = first called), so missing phone numbers can be listed in
	// the order they would be needed. Zero when unknown.
	PagingTier int `json:"pagingTier,omitempty"`
	// PhoneOptional is true when a missing number does not stop the tier from
	// reaching someone: the 2nd and 3rd responders, who are called together
	// with the 1st responder. Every other position must have a number.
	PhoneOptional bool `json:"phoneOptional,omitempty"`
}
