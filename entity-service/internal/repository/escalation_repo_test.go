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

package repository

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeRecipientDirectory maps addresses, team positions and app roles to
// "user".id values in memory, matching recipientDirectory's contract (an
// unknown address, or a position/role nobody holds, yields nobody, not an
// error) without real tables. It records every address lookup so a test can
// see which configured slots were consulted.
type fakeRecipientDirectory struct {
	idByEmail  map[string]string
	byPosition map[string][]string
	byRole     map[string][]string
	looked     []string
}

// The methods ignore q -- this fake never touches a database; q is accepted
// (a test may pass nil) to satisfy the real signatures, which thread
// CreateEscalation's own tx through.
func (f *fakeRecipientDirectory) UserIDsByEmails(_ context.Context, _ rowsQuerier, emails []string) ([]string, error) {
	var ids []string
	for _, e := range emails {
		f.looked = append(f.looked, e)
		if id, ok := f.idByEmail[e]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeRecipientDirectory) TeamPositionHolders(_ context.Context, _ rowsQuerier, position string) ([]string, error) {
	return f.byPosition[position], nil
}

func (f *fakeRecipientDirectory) AppRoleHolders(_ context.Context, _ rowsQuerier, role string) ([]string, error) {
	return f.byRole[role], nil
}

var _ recipientDirectory = (*fakeRecipientDirectory)(nil)

// --- nextEscalationLevel: level-transition math ---

func TestNextEscalationLevel_EscalateIncrements(t *testing.T) {
	for previous := 0; previous < maxEscalationLevel; previous++ {
		got, err := nextEscalationLevel(domain.EscalationActionEscalate, previous)
		if err != nil {
			t.Fatalf("previous=%d: unexpected error: %v", previous, err)
		}
		if want := previous + 1; got != want {
			t.Errorf("previous=%d: got %d, want %d", previous, got, want)
		}
	}
}

// TestNextEscalationLevel_EscalateAtEL5IsConflict: SN's createEscalation
// refuses with 409 "Case already at maximum escalation level".
func TestNextEscalationLevel_EscalateAtEL5IsConflict(t *testing.T) {
	_, err := nextEscalationLevel(domain.EscalationActionEscalate, maxEscalationLevel)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got error %v (%T), want *apierror.ConflictError", err, err)
	}
}

// TestNextEscalationLevel_DeescalateGoesToEL0: SN's deescalateToEL0 drops the
// case to EL0 from any level, not one step down.
func TestNextEscalationLevel_DeescalateGoesToEL0(t *testing.T) {
	for previous := 1; previous <= maxEscalationLevel; previous++ {
		got, err := nextEscalationLevel(domain.EscalationActionDeescalate, previous)
		if err != nil {
			t.Fatalf("previous=%d: unexpected error: %v", previous, err)
		}
		if got != 0 {
			t.Errorf("previous=%d: got EL%d, want EL0", previous, got)
		}
	}
}

// TestNextEscalationLevel_DeescalateAtEL0IsConflict: SN's 409 "Case already
// at EL0".
func TestNextEscalationLevel_DeescalateAtEL0IsConflict(t *testing.T) {
	_, err := nextEscalationLevel(domain.EscalationActionDeescalate, 0)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got error %v (%T), want *apierror.ConflictError", err, err)
	}
}

// TestNotificationLevel: an escalation notifies the level it reached, a
// de-escalation the level it left (SN passes currentLevel, not 0).
func TestNotificationLevel(t *testing.T) {
	if got := notificationLevel(domain.EscalationActionEscalate, 2, 3); got != 3 {
		t.Errorf("escalate EL2->EL3 notifies EL%d, want EL3", got)
	}
	if got := notificationLevel(domain.EscalationActionDeescalate, 3, 0); got != 3 {
		t.Errorf("de-escalate EL3->EL0 notifies EL%d, want EL3", got)
	}
}

func TestEscalationLevelInt(t *testing.T) {
	el := func(s string) *string { return &s }
	cases := []struct {
		name string
		raw  *string
		want int
	}{
		{"nil (never escalated) is EL0", nil, 0},
		{"EL0", el("EL0"), 0},
		{"EL3", el("EL3"), 3},
		{"EL5", el("EL5"), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escalationLevelInt(tc.raw); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// --- resolveEscalationRecipients: cumulative EL1..EL5 recipient rule ---

// escalationTestConfig is every EL1/EL2 slot configured, one user per
// address, the way ServiceNow's x_wso2_customer_0.escalation.* properties
// are, plus a CRE head and the EL4/EL5 role holders.
func escalationTestConfig() (EscalationNotificationConfig, *fakeRecipientDirectory) {
	cfg := EscalationNotificationConfig{
		EL1AmericasTLEmails:           []string{"tl-a@wso2.com", "tl-b@wso2.com"},
		EL2AmericasTUEmails:           []string{"tu@wso2.com"},
		EL2ProductServiceEmail:        "service@wso2.com",
		EL2ProductIdentityServerEmail: "is@wso2.com",
		EL2ProductDefaultEmail:        "default@wso2.com",
	}
	dir := &fakeRecipientDirectory{
		idByEmail: map[string]string{
			"tl-a@wso2.com": "user-tl-a", "tl-b@wso2.com": "user-tl-b", "tu@wso2.com": "user-tu",
			"service@wso2.com": "user-service", "is@wso2.com": "user-is", "default@wso2.com": "user-default",
		},
		byPosition: map[string][]string{"cre_head": {"user-cre-head"}},
		byRole: map[string][]string{
			"case_escalation_el4": {"user-cco", "user-cro"},
			"case_escalation_el5": {"user-ceo"},
		},
	}
	return cfg, dir
}

// TestResolveEscalationRecipients_CumulativeInSNOrder pins the whole
// EscalationNotificationUtils rule, level by level and in ServiceNow's own
// order: team leads, Americas TLs, account owner, technical owner (EL1);
// Americas TU, product contact (EL2); cre_head holders, CSM (EL3);
// case_escalation_el4 holders (EL4); case_escalation_el5 holders (EL5). Each
// level keeps everything below it.
func TestResolveEscalationRecipients_CumulativeInSNOrder(t *testing.T) {
	cfg, dir := escalationTestConfig()
	r := &escalationRepo{dir: dir, notifyCfg: cfg}
	cc := escalationCaseContext{
		teamLeadIDs:      []string{"user-lead-a", "user-lead-b"},
		accountManagerID: strPtr("user-account-owner"),
		technicalOwnerID: strPtr("user-tech-owner"),
		csmID:            strPtr("user-csm"),
	}
	el1 := []string{"user-lead-a", "user-lead-b", "user-tl-a", "user-tl-b", "user-account-owner", "user-tech-owner"}
	el2 := append(append([]string{}, el1...), "user-tu", "user-default")
	el3 := append(append([]string{}, el2...), "user-cre-head", "user-csm")
	el4 := append(append([]string{}, el3...), "user-cco", "user-cro")
	el5 := append(append([]string{}, el4...), "user-ceo")

	for level, want := range map[int][]string{1: el1, 2: el2, 3: el3, 4: el4, 5: el5} {
		got, err := r.resolveEscalationRecipients(context.Background(), nil, level, cc)
		if err != nil {
			t.Fatalf("level %d: unexpected error: %v", level, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("level %d:\n got %v\nwant %v", level, got, want)
		}
	}
}

// TestResolveEscalationRecipients_DedupesKeepingFirst: one person in two
// roles (here the technical owner is also the CSM and an Americas TL) is
// notified once, at the first place SN would have added them.
func TestResolveEscalationRecipients_DedupesKeepingFirst(t *testing.T) {
	cfg, dir := escalationTestConfig()
	r := &escalationRepo{dir: dir, notifyCfg: cfg}
	cc := escalationCaseContext{technicalOwnerID: strPtr("user-tl-b"), csmID: strPtr("user-tl-b")}

	got, err := r.resolveEscalationRecipients(context.Background(), nil, 3, cc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"user-tl-a", "user-tl-b", "user-tu", "user-default", "user-cre-head"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveEscalationRecipients_LevelZeroReturnsEmpty guards the fact that
// none of the "if newLevel >= N" branches fire at level 0 (a DEESCALATE that
// brought the case back down to EL0): the notification list must come back
// empty. Every slot is configured and the case carries every per-case id, so
// a future edit that adds a recipient outside a level guard fails here.
func TestResolveEscalationRecipients_LevelZeroReturnsEmpty(t *testing.T) {
	cfg, dir := escalationTestConfig()
	r := &escalationRepo{dir: dir, notifyCfg: cfg}
	cc := escalationCaseContext{technicalOwnerID: strPtr("user-owner"), accountManagerID: strPtr("user-am"), csmID: strPtr("user-csm")}

	got, err := r.resolveEscalationRecipients(context.Background(), nil, 0, cc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("level 0 must produce no recipients, got %v", got)
	}
}

// TestResolveEscalationRecipients_ProductRouting pins SN's
// _resolveProductBasedUser: exactly one EL2 product contact, and the default
// one when the case has no product at all.
func TestResolveEscalationRecipients_ProductRouting(t *testing.T) {
	cfg, dir := escalationTestConfig()
	cfg.EL1AmericasTLEmails, cfg.EL2AmericasTUEmails = nil, nil
	r := &escalationRepo{dir: dir, notifyCfg: cfg}

	for _, tc := range []struct {
		name string
		cc   escalationCaseContext
		want string
	}{
		{"no product falls to the default (SN defaults, it does not skip)", escalationCaseContext{}, "user-default"},
		{"a service product", escalationCaseContext{productCategory: strPtr("SERVICE"), productCode: strPtr("wso2is")}, "user-service"},
		{"Identity Server by code", escalationCaseContext{productCategory: strPtr("SOFTWARE"), productCode: strPtr("wso2is")}, "user-is"},
		{"Identity Server by name", escalationCaseContext{productCategory: strPtr("SOFTWARE"), productName: strPtr("WSO2 Identity Server")}, "user-is"},
		{"another software product", escalationCaseContext{productCategory: strPtr("SOFTWARE"), productCode: strPtr("wso2am"), productName: strPtr("WSO2 API Manager")}, "user-default"},
		{"a near-miss name is not Identity Server", escalationCaseContext{productCategory: strPtr("SOFTWARE"), productName: strPtr("WSO2 Identity Server as Key Manager")}, "user-default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.resolveEscalationRecipients(context.Background(), nil, 2, tc.cc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, []string{tc.want}) {
				t.Errorf("got %v, want [%s]", got, tc.want)
			}
		})
	}
}

// TestResolveEscalationRecipients_UnconfiguredOrUnknownIsNotFatal: an unset
// slot is never looked up, an address with no user contributes nobody, and a
// position or role nobody holds contributes nobody -- SN logs "user not
// found" and carries on.
func TestResolveEscalationRecipients_UnconfiguredOrUnknownIsNotFatal(t *testing.T) {
	dir := &fakeRecipientDirectory{}
	r := &escalationRepo{dir: dir, notifyCfg: EscalationNotificationConfig{EL2ProductDefaultEmail: "nobody@wso2.com"}}

	got, err := r.resolveEscalationRecipients(context.Background(), nil, 5, escalationCaseContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want no recipients", got)
	}
	if want := []string{"nobody@wso2.com"}; !reflect.DeepEqual(dir.looked, want) {
		t.Errorf("looked up %v, want only the one configured address %v", dir.looked, want)
	}
}

// TestResolveEscalationRecipients_EL5IncludesEL4Holders: escalation is
// cumulative, so an EL5 escalation reaches the EL4 role holders too, and a
// person holding both roles is notified once.
func TestResolveEscalationRecipients_EL5IncludesEL4Holders(t *testing.T) {
	dir := &fakeRecipientDirectory{byRole: map[string][]string{
		"case_escalation_el4": {"user-cco", "user-exec"},
		"case_escalation_el5": {"user-exec"},
	}}
	r := &escalationRepo{dir: dir}
	got, err := r.resolveEscalationRecipients(context.Background(), nil, 5, escalationCaseContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"user-cco", "user-exec"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
