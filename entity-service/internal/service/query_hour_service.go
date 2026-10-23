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
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Query-hour utilisation, ported from ServiceNow's QueryHourUtils.
//
// THE RULES, AND WHERE THEY COME FROM.
//
// Entitlement, per opportunity, over product lines in service today:
//
//	quantity x 10|25|50|100|200   for "Development Support - N hours"
//	quantity x 1                  for "Query support limit (included in
//	                                   subscription)"
//
// held in MINUTES, because ServiceNow does: `total_query_hour +=
// query_hour_tmp * 60`.
//
// Consumption, per project linked to that opportunity: approved time
// cards inside the opportunity's own service window, BILLABLE ONLY.
// ServiceNow adds `summary.total_billable` to its running total and
// computes the non-billable figure purely to display, so non-billable
// work draws nothing down. Onboarding time is then subtracted -- it is
// not query support.
//
// Thresholds:
//
//	remaining <   0        -> exceeded
//	remaining < 600 (mins) -> close to exceeding, i.e. under ten hours
//
// State ladder, by usage percentage: 100 -> CLOSURE_NOTICE_100,
// 90 -> NOTIFIED_90, 75 -> NOTIFIED_75, below -> cleared.
//
// NO MIGRATION. Every column written here already exists:
// sf_opportunity.query_hour_state (an enum whose three values are
// already ServiceNow's 1/2/3) and project's total_query_duration,
// consumed_duration and remaining_query_duration, which hold
// ServiceNow's mirrored figures today.

// QueryHourService recomputes utilisation and reports on it.
type QueryHourService interface {
	// Recompute rebuilds every opportunity's utilisation, writes the
	// figures back, and returns the projects that crossed a threshold.
	Recompute(ctx context.Context) ([]domain.QueryHourTransition, error)
	// WeeklyReport returns the exceeded and close-to-exceeding accounts,
	// with the audience their owners imply.
	WeeklyReport(ctx context.Context) (domain.QueryHourReport, error)
}

type queryHourService struct {
	repo repository.QueryHourRepository
	// internalDomain restricts derived recipients. ServiceNow checks
	// `ref_email.includes("@wso2.com")` before adding an owner, so a
	// customer contact mistakenly set as an owner is not mailed an
	// internal report.
	internalDomain string
	now            func() time.Time
}

// NewQueryHourService constructs a QueryHourService.
func NewQueryHourService(repo repository.QueryHourRepository, internalDomain string) QueryHourService {
	if internalDomain == "" {
		internalDomain = "@wso2.com"
	}
	return &queryHourService{repo: repo, internalDomain: strings.ToLower(internalDomain), now: time.Now}
}

// computed is one opportunity after the sums, before it is shaped for
// either consumer.
type computed struct {
	opp   domain.QueryHourOpportunity
	lines []repository.QueryHourLineRow
}

// computeAll does the arithmetic for every opportunity with a line in
// service. It reads; it writes nothing.
func (s *queryHourService) computeAll(ctx context.Context) ([]computed, error) {
	lines, err := s.repo.ActiveLines(ctx)
	if err != nil {
		return nil, err
	}

	byOpp := map[string]*computed{}
	var order []string
	for _, l := range lines {
		c := byOpp[l.OpportunityID]
		if c == nil {
			c = &computed{opp: domain.QueryHourOpportunity{
				OpportunityID: l.OpportunityID,
				SalesforceID:  l.SalesforceID,
				Name:          l.Name,
				AccountID:     l.AccountID,
				AccountName:   l.AccountName,
			}}
			byOpp[l.OpportunityID] = c
			order = append(order, l.OpportunityID)
		}
		c.lines = append(c.lines, l)

		hours, known := domain.QueryHourProductHours[l.ProductName]
		if !known {
			// Counted, not ignored: a renamed SKU would otherwise
			// quietly reduce an entitlement to zero.
			c.opp.UnmatchedLines++
			// Counting it is not enough on its own -- nothing reads
			// the counter. A partly matched opportunity reports an
			// understated allowance and can show as exceeded; a fully
			// unmatched one never reaches the report at all.
			slog.Warn("query hours: product line matches no entitlement pack",
				"opportunityId", c.opp.OpportunityID, "product", l.ProductName)
			continue
		}
		c.opp.TotalMinutes += int(l.Quantity * float64(hours) * 60)

		start, end := l.ServiceStart, l.ServiceEnd
		if c.opp.ServiceStart == nil || start.Before(*c.opp.ServiceStart) {
			c.opp.ServiceStart = &start
		}
		if c.opp.ServiceEnd == nil || end.After(*c.opp.ServiceEnd) {
			c.opp.ServiceEnd = &end
		}
	}

	out := make([]computed, 0, len(order))
	for _, id := range order {
		c := byOpp[id]
		if c.opp.ServiceStart == nil || c.opp.ServiceEnd == nil {
			// Every line was an unmatched product, so there is no
			// window to measure consumption over and no allowance to
			// measure it against.
			out = append(out, *c)
			continue
		}

		projects, err := s.repo.ProjectConsumption(ctx, id, *c.opp.ServiceStart, *c.opp.ServiceEnd)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			consumed := p.BillableMinutes - p.OnboardingMinutes
			if consumed < 0 {
				// Onboarding exceeding billable would otherwise credit
				// the allowance, which no contract offers.
				consumed = 0
			}
			c.opp.Projects = append(c.opp.Projects, domain.QueryHourProject{
				ProjectID:          p.ProjectID,
				ProjectKey:         p.ProjectKey,
				ProjectName:        p.ProjectName,
				SalesforceID:       p.ProjectSFID,
				ConsumedMinutes:    consumed,
				NonBillableMinutes: p.NonBillableMinutes,
				OnboardingMinutes:  p.OnboardingMinutes,
			})
			c.opp.ConsumedMinutes += consumed
		}

		c.opp.RemainingMinutes = c.opp.TotalMinutes - c.opp.ConsumedMinutes
		c.opp.IsExceeded = c.opp.RemainingMinutes < 0
		c.opp.IsCloseToExceed = !c.opp.IsExceeded &&
			c.opp.RemainingMinutes < domain.QueryHourCloseToExceedMinutes

		// A zero allowance is treated as one minute rather than divided
		// by -- ServiceNow's `total == 0 ? 1 : total`. Any consumption
		// against nothing is then over 100%, which is the right answer.
		denominator := c.opp.TotalMinutes
		if denominator == 0 {
			denominator = 1
		}
		c.opp.UsagePercent = float64(c.opp.ConsumedMinutes) / float64(denominator) * 100
		if st, ok := domain.QueryHourStateFor(c.opp.UsagePercent); ok {
			c.opp.State = &st
		}
		out = append(out, *c)
	}
	return out, nil
}

func (s *queryHourService) Recompute(ctx context.Context) ([]domain.QueryHourTransition, error) {
	all, err := s.computeAll(ctx)
	if err != nil {
		return nil, err
	}
	previous, err := s.repo.OpportunityStates(ctx)
	if err != nil {
		return nil, err
	}

	var transitions []domain.QueryHourTransition
	for _, c := range all {
		before := previous[c.opp.OpportunityID]
		after := c.opp.State

		if !sameState(before, after) {
			if err := s.repo.SetOpportunityState(ctx, c.opp.OpportunityID, after); err != nil {
				return nil, err
			}
		}

		// An opportunity's allowance is shared by its projects, so each
		// one carries the same totals -- which is what the per-project
		// notification prints.
		for _, p := range c.opp.Projects {
			remaining := c.opp.TotalMinutes - c.opp.ConsumedMinutes
			if err := s.repo.SetProjectDurations(ctx, p.ProjectID,
				c.opp.TotalMinutes, c.opp.ConsumedMinutes, remaining); err != nil {
				return nil, err
			}
		}

		// Only a RISE to a threshold notifies. ServiceNow's trigger is
		// "Query Hour State changes to 1, 2 or 3", so clearing the
		// state, or dropping from 90% back to 75% because an allowance
		// grew, is not an event anybody is told about.
		if after == nil || sameState(before, after) || !isRise(before, *after) {
			continue
		}
		for _, p := range c.opp.Projects {
			transitions = append(transitions, domain.QueryHourTransition{
				ProjectID:           p.ProjectID,
				ProjectKey:          p.ProjectKey,
				ProjectName:         p.ProjectName,
				AccountID:           c.opp.AccountID,
				AccountName:         c.opp.AccountName,
				From:                before,
				To:                  *after,
				TotalText:           FormatQueryHourMinutes(c.opp.TotalMinutes),
				ConsumedText:        FormatQueryHourMinutes(c.opp.ConsumedMinutes),
				RemainingText:       FormatQueryHourMinutes(c.opp.RemainingMinutes),
				AccountManagerEmail: s.internal(firstLine(c.lines).AccountManagerEmail),
				AccountManagerName:  firstLine(c.lines).AccountManagerName,
				TechnicalOwnerEmail: s.internal(firstLine(c.lines).TechnicalOwnerEmail),
			})
		}
	}
	return transitions, nil
}

func (s *queryHourService) WeeklyReport(ctx context.Context) (domain.QueryHourReport, error) {
	all, err := s.computeAll(ctx)
	if err != nil {
		return domain.QueryHourReport{}, err
	}

	exceeded := map[string]*domain.QueryHourAccount{}
	closeTo := map[string]*domain.QueryHourAccount{}
	var recipients []string

	add := func(into map[string]*domain.QueryHourAccount, c computed) *domain.QueryHourAccount {
		a := into[c.opp.AccountID]
		if a == nil {
			l := firstLine(c.lines)
			a = &domain.QueryHourAccount{
				AccountID:           c.opp.AccountID,
				AccountName:         c.opp.AccountName,
				SalesforceID:        l.AccountSFID,
				AccountManagerEmail: s.internal(l.AccountManagerEmail),
				TechnicalOwnerEmail: s.internal(l.TechnicalOwnerEmail),
			}
			into[c.opp.AccountID] = a
		}
		a.Opportunities = append(a.Opportunities, c.opp)
		return a
	}

	for _, c := range all {
		// Owners of accounts in EITHER table are mailed. ServiceNow
		// pushes them inside both the is_exceeded and the
		// is_close_to_exceeded branch, not just the first.
		switch {
		case c.opp.IsExceeded:
			a := add(exceeded, c)
			a.IsExceeded = true
			recipients = append(recipients, a.AccountManagerEmail, a.TechnicalOwnerEmail)
		case c.opp.IsCloseToExceed:
			a := add(closeTo, c)
			a.IsCloseToExceed = true
			recipients = append(recipients, a.AccountManagerEmail, a.TechnicalOwnerEmail)
		}
	}

	report := domain.QueryHourReport{
		GeneratedOn:   s.now().UTC().Format("2006-01-02"),
		Exceeded:      sortedAccounts(exceeded),
		CloseToExceed: sortedAccounts(closeTo),
		Recipients:    dedupeNonEmpty(recipients),
	}
	return report, nil
}

// internal returns the address only when it is one of ours.
func (s *queryHourService) internal(email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || !strings.HasSuffix(e, s.internalDomain) {
		return ""
	}
	return e
}

// FormatQueryHourMinutes renders minutes the way the ServiceNow mails
// do, as "Xh Ym". Negative stays negative: an overdrawn allowance reads
// as a negative remainder rather than silently as zero.
func FormatQueryHourMinutes(mins int) string {
	sign := ""
	if mins < 0 {
		sign, mins = "-", -mins
	}
	return fmt.Sprintf("%s%dh %dm", sign, mins/60, mins%60)
}

func sameState(a, b *domain.QueryHourState) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

// queryHourRank orders the ladder so a rise can be told from a fall.
func queryHourRank(s *domain.QueryHourState) int {
	if s == nil {
		return 0
	}
	switch *s {
	case domain.QueryHourStateNotified75:
		return 1
	case domain.QueryHourStateNotified90:
		return 2
	case domain.QueryHourStateClosure100:
		return 3
	default:
		return 0
	}
}

func isRise(from *domain.QueryHourState, to domain.QueryHourState) bool {
	return queryHourRank(&to) > queryHourRank(from)
}

func firstLine(lines []repository.QueryHourLineRow) repository.QueryHourLineRow {
	if len(lines) == 0 {
		return repository.QueryHourLineRow{}
	}
	return lines[0]
}

func sortedAccounts(m map[string]*domain.QueryHourAccount) []domain.QueryHourAccount {
	out := make([]domain.QueryHourAccount, 0, len(m))
	for _, a := range m {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountName < out[j].AccountName })
	return out
}

func dedupeNonEmpty(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
