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
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var alertMuteRuleTestNow = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

const alertMuteRuleTestID = "0195a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

// fakeAlertMuteRuleRepo: stored is what Get returns; written is what a guarded write returns (nil = the guard matched nothing).
type fakeAlertMuteRuleRepo struct {
	repository.AlertMuteRuleRepository
	stored   *domain.AlertMuteRule
	written  *domain.AlertMuteRule
	deleted  bool
	created  domain.AlertMuteRuleWrite
	extendTo time.Time
	calls    int
}

func (f *fakeAlertMuteRuleRepo) Get(context.Context, string) (*domain.AlertMuteRule, error) {
	return f.stored, nil
}
func (f *fakeAlertMuteRuleRepo) Create(_ context.Context, _ string, w domain.AlertMuteRuleWrite) (domain.AlertMuteRule, error) {
	f.created = w
	f.calls++
	return domain.AlertMuteRule{}, nil
}
func (f *fakeAlertMuteRuleRepo) Update(context.Context, string, string, domain.AlertMuteRuleWrite, time.Time) (*domain.AlertMuteRule, error) {
	f.calls++
	return f.written, nil
}
func (f *fakeAlertMuteRuleRepo) Extend(_ context.Context, _, _ string, endOn, _ time.Time) (*domain.AlertMuteRule, error) {
	f.calls++
	f.extendTo = endOn
	return f.written, nil
}
func (f *fakeAlertMuteRuleRepo) End(context.Context, string, string, time.Time) (*domain.AlertMuteRule, error) {
	f.calls++
	return f.written, nil
}
func (f *fakeAlertMuteRuleRepo) Delete(context.Context, string, time.Time) (bool, error) {
	f.calls++
	return f.deleted, nil
}

func newTestAlertMuteRuleService(repo repository.AlertMuteRuleRepository) *alertMuteRuleService {
	return &alertMuteRuleService{repo: repo, now: func() time.Time { return alertMuteRuleTestNow }}
}

func validAlertMuteRuleRequest() domain.SaveAlertMuteRuleRequest {
	return domain.SaveAlertMuteRuleRequest{
		Name:        "  Ballerina Central maintenance ",
		Description: " planned ",
		TimeZone:    "Asia/Colombo",
		StartOn:     "2026-11-06T15:30:00+05:30",
		EndOn:       "2026-11-06T12:00:00Z",
		Conditions: []domain.AlertMuteRuleCondition{
			{Field: "service", Value: " client-ballerinacentral-alert-integration "},
			{Field: "environment", Value: "prod"},
			{Field: "source", Value: "Site24x7"},
		},
	}
}

func isValidation(err error) bool {
	var v *apierror.ValidationError
	return errors.As(err, &v)
}

func TestAlertMuteRuleCreate_TrimsAndStoresUTC(t *testing.T) {
	repo := &fakeAlertMuteRuleRepo{}
	if _, err := newTestAlertMuteRuleService(repo).Create(context.Background(), validAlertMuteRuleRequest()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := domain.AlertMuteRuleWrite{
		Name: "Ballerina Central maintenance", Description: "planned", TimeZone: "Asia/Colombo",
		StartOn: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), EndOn: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		Conditions: []domain.AlertMuteRuleCondition{
			{Field: "service", Value: "client-ballerinacentral-alert-integration"},
			{Field: "environment", Value: "prod"},
			{Field: "source", Value: "Site24x7"},
		},
	}
	if !reflect.DeepEqual(repo.created, want) {
		t.Fatalf("stored %+v, want %+v", repo.created, want)
	}
	if repo.created.StartOn.Location() != time.UTC {
		t.Fatalf("startOn not UTC: %v", repo.created.StartOn)
	}
}

func TestAlertMuteRuleCreate_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*domain.SaveAlertMuteRuleRequest)
		msg    string
	}{
		{"blank name", func(r *domain.SaveAlertMuteRuleRequest) { r.Name = "   " }, "name is required"},
		{"long name", func(r *domain.SaveAlertMuteRuleRequest) { r.Name = strings.Repeat("é", 256) }, "name must be at most 255"},
		{"long description", func(r *domain.SaveAlertMuteRuleRequest) { r.Description = strings.Repeat("x", 1025) }, "description must be at most 1024"},
		{"no time zone", func(r *domain.SaveAlertMuteRuleRequest) { r.TimeZone = "" }, "timeZone is required"},
		{"unknown time zone", func(r *domain.SaveAlertMuteRuleRequest) { r.TimeZone = "Mars/Olympus" }, "timeZone must be an IANA"},
		{"Local time zone", func(r *domain.SaveAlertMuteRuleRequest) { r.TimeZone = "Local" }, "timeZone must be an IANA"},
		{"bad start", func(r *domain.SaveAlertMuteRuleRequest) { r.StartOn = "2026-11-06 10:00" }, "startOn must be an RFC 3339"},
		{"bad end", func(r *domain.SaveAlertMuteRuleRequest) { r.EndOn = "" }, "endOn must be an RFC 3339"},
		{"end before start", func(r *domain.SaveAlertMuteRuleRequest) { r.EndOn = "2026-11-06T10:00:00Z" }, "endOn must be later than startOn"},
		{"longer than 72h", func(r *domain.SaveAlertMuteRuleRequest) { r.EndOn = "2026-11-09T10:00:01Z" }, "at most 72 hours"},
		{"end in the past", func(r *domain.SaveAlertMuteRuleRequest) {
			r.StartOn, r.EndOn = "2026-11-06T08:00:00Z", "2026-11-06T09:00:00Z"
		}, "endOn must be in the future"},
		{"start more than a minute ago", func(r *domain.SaveAlertMuteRuleRequest) { r.StartOn = "2026-11-06T09:58:59Z" }, "startOn must not be in the past"},
		{"unknown field", func(r *domain.SaveAlertMuteRuleRequest) {
			r.Conditions = append(r.Conditions, domain.AlertMuteRuleCondition{Field: "host", Value: "x"})
		}, "conditions[3].field must be one of"},
		{"repeated field", func(r *domain.SaveAlertMuteRuleRequest) {
			r.Conditions = append(r.Conditions, domain.AlertMuteRuleCondition{Field: "environment", Value: "staging"})
		}, "given more than once"},
		{"blank value", func(r *domain.SaveAlertMuteRuleRequest) { r.Conditions[2].Value = "  " }, "conditions[2].value is required"},
		{"long value", func(r *domain.SaveAlertMuteRuleRequest) { r.Conditions[2].Value = strings.Repeat("v", 256) }, "at most 255"},
		{"no environment", func(r *domain.SaveAlertMuteRuleRequest) { r.Conditions = r.Conditions[:1] }, "must include service and environment"},
		{"no service", func(r *domain.SaveAlertMuteRuleRequest) { r.Conditions = r.Conditions[1:] }, "must include service and environment"},
		{"no conditions", func(r *domain.SaveAlertMuteRuleRequest) { r.Conditions = nil }, "must include service and environment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validAlertMuteRuleRequest()
			tc.mutate(&req)
			repo := &fakeAlertMuteRuleRepo{}
			_, err := newTestAlertMuteRuleService(repo).Create(context.Background(), req)
			if !isValidation(err) || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("got %v, want a ValidationError containing %q", err, tc.msg)
			}
			if repo.calls != 0 {
				t.Fatal("repository was called for an invalid request")
			}
		})
	}
}

func TestAlertMuteRuleCreate_AcceptsStartWithinTheGraceMinuteAndExactly72h(t *testing.T) {
	req := validAlertMuteRuleRequest()
	req.StartOn, req.EndOn = "2026-11-06T09:59:00Z", "2026-11-09T09:59:00Z"
	if _, err := newTestAlertMuteRuleService(&fakeAlertMuteRuleRepo{}).Create(context.Background(), req); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestAlertMuteRuleGuardedWrites_NotFoundVersusWrongState(t *testing.T) {
	ctx := context.Background()
	stored := &domain.AlertMuteRule{ID: alertMuteRuleTestID, StartOn: alertMuteRuleTestNow.Add(-time.Hour), EndOn: alertMuteRuleTestNow.Add(time.Hour)}
	calls := map[string]func(*alertMuteRuleService) error{
		"update": func(s *alertMuteRuleService) error {
			_, err := s.Update(ctx, alertMuteRuleTestID, validAlertMuteRuleRequest())
			return err
		},
		"end":    func(s *alertMuteRuleService) error { _, err := s.End(ctx, alertMuteRuleTestID); return err },
		"delete": func(s *alertMuteRuleService) error { return s.Delete(ctx, alertMuteRuleTestID) },
	}
	for name, call := range calls {
		var nf *apierror.NotFoundError
		if err := call(newTestAlertMuteRuleService(&fakeAlertMuteRuleRepo{})); !errors.As(err, &nf) {
			t.Errorf("%s of a missing rule: got %v, want NotFoundError", name, err)
		}
		var ce *apierror.ConflictError
		if err := call(newTestAlertMuteRuleService(&fakeAlertMuteRuleRepo{stored: stored})); !errors.As(err, &ce) {
			t.Errorf("%s in the wrong state: got %v, want ConflictError", name, err)
		}
	}
	if _, err := newTestAlertMuteRuleService(&fakeAlertMuteRuleRepo{}).Get(ctx, "not-a-uuid"); !isValidation(err) {
		t.Errorf("Get with a bad id: got %v, want ValidationError", err)
	}
}

func TestAlertMuteRuleExtend(t *testing.T) {
	ctx := context.Background()
	active := &domain.AlertMuteRule{ID: alertMuteRuleTestID, StartOn: alertMuteRuleTestNow.Add(-time.Hour), EndOn: alertMuteRuleTestNow.Add(time.Hour)}
	ended := alertMuteRuleTestNow.Add(-time.Minute)
	endedEarly := &domain.AlertMuteRule{ID: alertMuteRuleTestID, StartOn: active.StartOn, EndOn: active.EndOn, EndedOn: &ended}
	expired := &domain.AlertMuteRule{ID: alertMuteRuleTestID, StartOn: alertMuteRuleTestNow.Add(-3 * time.Hour), EndOn: alertMuteRuleTestNow.Add(-time.Hour)}

	repo := &fakeAlertMuteRuleRepo{stored: active, written: active}
	if _, err := newTestAlertMuteRuleService(repo).Extend(ctx, alertMuteRuleTestID, domain.ExtendAlertMuteRuleRequest{EndOn: "2026-11-06T13:00:00+01:00"}); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if want := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC); !repo.extendTo.Equal(want) {
		t.Fatalf("extended to %v, want %v", repo.extendTo, want)
	}

	for _, tc := range []struct {
		name   string
		stored *domain.AlertMuteRule
		endOn  string
		check  func(error) bool
	}{
		{"missing", nil, "2026-11-06T12:00:00Z", func(err error) bool { var e *apierror.NotFoundError; return errors.As(err, &e) }},
		{"ended early", endedEarly, "2026-11-06T12:00:00Z", func(err error) bool { var e *apierror.ConflictError; return errors.As(err, &e) }},
		{"expired", expired, "2026-11-06T12:00:00Z", func(err error) bool { var e *apierror.ConflictError; return errors.As(err, &e) }},
		{"not later", active, "2026-11-06T11:00:00Z", isValidation},
		{"past 72h from start", active, "2026-11-09T09:00:01Z", isValidation},
		{"bad timestamp", active, "tomorrow", isValidation},
	} {
		repo := &fakeAlertMuteRuleRepo{stored: tc.stored, written: tc.stored}
		_, err := newTestAlertMuteRuleService(repo).Extend(ctx, alertMuteRuleTestID, domain.ExtendAlertMuteRuleRequest{EndOn: tc.endOn})
		if !tc.check(err) {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if repo.calls != 0 {
			t.Errorf("%s: repository write was called", tc.name)
		}
	}
}

func TestAlertMuteRuleSearch_RefusesUnknownState(t *testing.T) {
	_, err := newTestAlertMuteRuleService(&fakeAlertMuteRuleRepo{}).Search(context.Background(), domain.SearchAlertMuteRulesRequest{
		Filters: domain.SearchAlertMuteRulesFilters{States: []domain.AlertMuteRuleState{"ended"}},
	})
	if !isValidation(err) {
		t.Fatalf("got %v, want ValidationError", err)
	}
}

func TestSNAlertMuteRuleService_RefusesWith400(t *testing.T) {
	svc := NewSNAlertMuteRuleService()
	ctx := context.Background()
	if _, err := svc.Active(ctx); !isValidation(err) {
		t.Errorf("Active: got %v, want ValidationError", err)
	}
	if _, err := svc.Create(ctx, validAlertMuteRuleRequest()); !isValidation(err) {
		t.Errorf("Create: got %v, want ValidationError", err)
	}
	if err := svc.Delete(ctx, alertMuteRuleTestID); !isValidation(err) {
		t.Errorf("Delete: got %v, want ValidationError", err)
	}
}
