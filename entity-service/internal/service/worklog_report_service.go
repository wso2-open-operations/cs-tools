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
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The CS Worklogs daily and weekly reports.
//
// WHERE THE RULES COME FROM. This is not a ServiceNow port -- no artefact
// in the instance produces either mail, and nothing in cs-tools,
// digiops-cs or digiops-infra does either. The specification is two
// delivered mails, weekly 2026-10-05..2026-10-11 and daily 2026-10-18, and
// every rule below was derived by reconciling their printed figures:
//
//   - Total = Approved + Pending, with rejected time excluded entirely
//     (31.38 + 943.02 = 974.40 exactly).
//   - The top-level average divides by all members (974.40/79 = 12.33).
//   - A team's issue count is the sum of its developers' counts, so an
//     issue two people worked counts twice (Americas 120, Atlas 34,
//     CS Migrations 76, Castor 53 all reconcile that way and no other).
//   - Worked days are weekdays minus leave, NOT days with time cards --
//     vikt@ shows 5 worked days against 0 issues and 0.00 hours.
//   - Avg time/day is total hours over those worked days (asha@
//     25.08/5 = 5.02, hugo@ 7.83/3 = 2.61, gita@ 5.33/4 = 1.33).
//
// THE SERVICENOW SOURCE, FOUND ON PROD 2026-10-20.
//
// An earlier pass concluded no ServiceNow artefact produced these mails.
// That was wrong, and wrong for a dull reason: the discovery scripts ran
// against DEV. The flow is WeeklyReportAutomation V2 -- trigger Weekly,
// Monday 16:00 UTC (= 21:30 Asia/Colombo, which is the timestamp on the
// delivered mail) -- with four actions: DateRange, WeeklyReport,
// GetLeadsEmails_WeeklyReport, Send Email. The aggregation lives in
// global.WeeklyTimeReportUtil_V2, which extends WeeklyTimeReportUtil.
//
// DATA COMES FROM POSTGRES; THE LOGIC IS ServiceNow's. We do not
// reproduce its mechanics -- personal schedules, cmn_schedule_span,
// agent_events, the HR leave sync -- because our own tables model the
// same facts directly. We reproduce what it COMPUTES. The mapping:
//
//   sys_user_grmember + group.type        -> team_member + team.type
//   u_team_member_role.u_role = lead      -> team_member.role = 'lead'
//   u_team_member_role = team_lead        -> the cre-leadership team,
//                                            excluded by team-type scope
//   cmn_schedule_span time_off / exclude
//     / ops_night                         -> absence buckets LEAVE,
//                                            EXCLUDED
//   u_customer_engagement_allocation_
//     resource, Confirmed, >= 100%        -> absence bucket ALLOCATION,
//                                            kind ENG
//
// Two things the source settled that we had got wrong, both now fixed:
// the weekly per-team average excludes leads deliberately rather than
// being off by one, and a half-day absence costs a whole worked day.
//
// THE 791-AGAINST-574 GAP IS EXPLAINED, and reproduced. buildTimeReport
// increments nCases once per case but caseTypes[type] once per matching
// DATE, so a case worked on three days adds 1 to the issue total and 3
// to its type bucket. The two counters measure cases and case-days
// respectively and never reconciled by design. A daily report shows no
// gap only because one day cannot hold a second day.

// worklogDropWeekendCards makes the weekly ignore cards dated on a Saturday
// or Sunday, because the previous system gives weekend dates no team. It is
// a single switch: set it to false to count weekend time. Absence (leave,
// exclusion, allocation) is NOT such a switch -- absent-day time is always
// counted on the weekly.
const worklogDropWeekendCards = true

// Only two states reach the report at all. addTimeReportDetails maps
// 'Approved' to the approved bucket and 'Submitted' to the pending one,
// and leaves stateRecord null for everything else -- which skips that
// time card entirely. PENDING, RECALLED, PROCESSED and UNKNOWN therefore
// contribute nothing, not even to the pending total.
//
// REJECTED is skipped by the same rule. We still count rejected cards for
// the daily report's disapproved-entries column, but their hours stay out
// of every total, which is what ServiceNow does.
// Compared case-insensitively on purpose. The domain constants are
// lowercase ("approved") while time_card.state is a Postgres enum whose
// labels are uppercase ('APPROVED'), and the repository selects it as
// text. A direct map lookup silently matches nothing, which counted all
// approved time as pending for as long as no fixture had any.
var worklogApprovedStates = map[string]bool{
	strings.ToUpper(string(domain.TimeCardStateApproved)): true,
}

var worklogPendingStates = map[string]bool{
	strings.ToUpper(string(domain.TimeCardStateSubmitted)): true,
}

// worklogTypeOrder is the order the mails print the type breakdown in, and
// worklogTypeLabels is what they call each one. The reports say "Query"
// where the schema says CASE; every other label is the enum value in title
// case. A type absent from this map is still counted, under its raw value.
var worklogTypeOrder = []string{
	"INCIDENT", "SERVICE_REQUEST", "CASE", "SECURITY_REPORT_ANALYSIS", "ENGAGEMENT",
	"PROBLEM", "CHANGE_REQUEST", "ANNOUNCEMENT",
}

var worklogTypeLabels = map[string]string{
	"INCIDENT":                 "Incident",
	"SERVICE_REQUEST":          "Service Request",
	"CASE":                     "Query",
	"SECURITY_REPORT_ANALYSIS": "Security Report Analysis",
	"ENGAGEMENT":               "Engagement",
	"PROBLEM":                  "Problem",
	"CHANGE_REQUEST":           "Change Request",
	"ANNOUNCEMENT":             "Announcement",
}

// WorklogReportService builds the two CS Worklogs reports.
type WorklogReportService interface {
	BuildReport(ctx context.Context, req domain.WorklogReportRequest) (domain.WorklogReport, error)
}

type worklogReportService struct {
	repo repository.WorklogReportRepository
	// teamTypes restricts the roster to these team.type values. Empty
	// means every team that has a key.
	teamTypes []string
	// excludedTeamKeys drops individual teams from that scope. The
	// type filter is broad by nature -- type 'cre' catches approval
	// and review teams alongside the real delivery ones -- and a
	// deployment needs to take particular teams out without having to
	// narrow the types to the point of brittleness.
	excludedTeamKeys []string
}

// NewWorklogReportService constructs a WorklogReportService.
func NewWorklogReportService(repo repository.WorklogReportRepository,
	teamTypes, excludedTeamKeys []string) WorklogReportService {
	return &worklogReportService{
		repo:             repo,
		teamTypes:        teamTypes,
		excludedTeamKeys: excludedTeamKeys,
	}
}

// worklogTypeAgg is one work-item type's tally.
type worklogTypeAgg struct {
	approvedMin  int
	pendingMin   int
	approvedDays int
	pendingDays  int
}

// perUser is the working accumulator for one person.
type perUser struct {
	roster       repository.WorklogRosterRow
	approvedMin  int
	pendingMin   int
	approvedKeys map[string]bool
	pendingKeys  map[string]bool
	orderedKeys  []domain.WorklogIssueRef
	seenKey      map[string]bool
	disapproved  int
	leaveCode    string
	// blocked is the set of weekdays in range covered by leave of any kind
	// (a half day included), an exclusion or a confirmed allocation. It
	// drives Worked Days and Average People Per Day; it does not drop
	// logged time.
	blocked map[time.Time]bool
	// types is this person's tally per type bucket.
	types map[string]*worklogTypeAgg
	// leaveHalf is set when the absence covering the daily report's one
	// day is a half day. summerizeTimeReport treats such a person as 0.5
	// of a present member rather than an absent one.
	leaveHalf bool
	// excluded is set by an absence that removes the person from the
	// daily report altogether rather than marking them away: a
	// cmn_schedule_span of type 'exclude', or a customer engagement
	// allocation at full utilization.
	excluded bool
}

func (s *worklogReportService) BuildReport(ctx context.Context, req domain.WorklogReportRequest) (domain.WorklogReport, error) {
	if req.From.IsZero() || req.To.IsZero() {
		return domain.WorklogReport{}, &apierror.ValidationError{Msg: "from and to are required"}
	}
	from := dateOnly(req.From)
	to := dateOnly(req.To)
	if to.Before(from) {
		return domain.WorklogReport{}, &apierror.ValidationError{Msg: "to must not be before from"}
	}

	roster, err := s.repo.Roster(ctx, s.teamTypes, s.excludedTeamKeys)
	if err != nil {
		return domain.WorklogReport{}, err
	}
	times, err := s.repo.TimeRows(ctx, from, to)
	if err != nil {
		return domain.WorklogReport{}, err
	}
	leave, err := s.repo.Leave(ctx, from, to)
	if err != nil {
		return domain.WorklogReport{}, err
	}
	// THE WEEKLY IS NOT ROTA-DRIVEN, THE DAILY IS ONLY ON A WEEKEND.
	//
	// The weekly (WeeklyTimeReportUtil_V2.getTeamInfo) starts from every
	// member of the operational groups -- our team_member rows -- and
	// then removes dates, not people: weekend dates, leave, exclusions
	// and confirmed allocations. A person is in the report when at least
	// one weekday survives, whatever they logged. No rota is read.
	//
	// The daily is NOT rota-driven either, except on a weekend.
	// DailyTimeReportUtil_V2.__internal_getTeamInfo starts from every
	// member of the operational groups and narrows by shift only inside
	//
	//     if (this.teamScheduleUtils.isWeekend(new GlideDateTime(date)))
	//         this.__internal_excludeNonWeekendMembers(teamMembers, date);
	//
	// so a Wednesday report lists the whole roster -- most of it with
	// zeros -- and a Sunday report lists the weekend crew. Reading the
	// rota on a weekday and gating on it, as this did, silently dropped
	// everyone the rota happened not to cover: on staging for Wed
	// 2026-10-21 that was 64 of team cre-abt's 98 members.
	daily := req.Cadence == domain.WorklogReportCadenceDaily
	weekendDaily := daily && isWeekend(from)
	rotaGated := weekendDaily

	var rostered []string
	if rotaGated {
		if rostered, err = s.repo.WeekendCrewUserIDs(ctx, from, to); err != nil {
			return domain.WorklogReport{}, err
		}
	}
	// Every team, not just the ones in the report's scope: the leadership
	// team contributes recipients while appearing in no team section.
	contacts, err := s.repo.TeamContacts(ctx, nil, s.excludedTeamKeys)
	if err != nil {
		return domain.WorklogReport{}, err
	}
	isRostered := make(map[string]bool, len(rostered))
	for _, id := range rostered {
		isRostered[id] = true
	}

	users := make(map[string]*perUser, len(roster))
	for _, r := range roster {
		users[r.UserID] = &perUser{
			roster:       r,
			approvedKeys: map[string]bool{},
			pendingKeys:  map[string]bool{},
			seenKey:      map[string]bool{},
			blocked:      map[time.Time]bool{},
			types:        map[string]*worklogTypeAgg{},
		}
	}

	moves, err := s.repo.MoveSpans(ctx, from, to)
	if err != nil {
		return domain.WorklogReport{}, err
	}
	s.reseatMovedMembers(users, moves, from, to, daily)

	// Leave first: worked days depend on it, and the daily report's Leave
	// column is the kind active on the single day in range.
	for _, l := range leave {
		u, ok := users[l.UserID]
		if !ok {
			continue
		}
		covered, onFrom := coveredWeekdays(l, from, to)
		for _, d := range covered {
			u.blocked[d] = true
		}
		if !onFrom {
			continue
		}
		// EXCLUDED and the customer-engagement allocations delete the
		// member outright. __internal_excludeMembersAndMarkLeaves drops
		// anyone with an 'exclude' span, and
		// __internal_excludeAllocatedMembers drops anyone on a Confirmed
		// allocation at 100% -- neither is "away today", both are "not
		// part of this report".
		if l.Bucket != "LEAVE" {
			u.excluded = true
			continue
		}
		if u.leaveCode == "" {
			u.leaveCode = l.ShortCode
		}
		if l.ShortCode == "HD-AM" || l.ShortCode == "HD-PM" {
			u.leaveHalf = true
		}
	}

	// Time, at the (person, work item, date, state) grain the repo returns.
	// A type bucket counts CASE-DAYS, not cases, and that is not a
	// mistake. ServiceNow increments nCases once per case but
	// caseTypes[type] once per matching DATE inside the same loop, so a
	// case worked on three days adds 1 to the issue total and 3 to its
	// type. That is the whole of the 791-against-574 gap in the
	// delivered weekly mail: the two counters measure different things
	// and were never meant to reconcile. The repository already returns
	// one row per (person, item, date, state), so each row is one
	// increment.
	//
	// The per-type tallies are kept PER PERSON while the cards are read and
	// merged into byType only for people the report actually includes, so
	// the breakdown always covers exactly the listed members and its hours
	// add up to the total.
	byType := map[string]*worklogTypeAgg{}

	// Time logged by people on no team in scope: no section to put it in,
	// so the mails cannot show it. That is a DATA gap (no team_member row),
	// not a rule, so it is surfaced below rather than papered over.
	droppedMinutes := map[string]int{}

	for _, t := range times {
		state := strings.ToUpper(strings.TrimSpace(t.State))
		approved := worklogApprovedStates[state]
		u, ok := users[t.UserID]
		if !ok {
			if approved || worklogPendingStates[state] {
				droppedMinutes[t.UserID] += t.Minutes
			}
			continue
		}
		if state == strings.ToUpper(string(domain.TimeCardStateRejected)) {
			u.disapproved += t.Entries
			continue
		}
		if !approved && !worklogPendingStates[state] {
			// Neither Approved nor Submitted: ServiceNow drops the card.
			continue
		}

		// WEEKLY ATTRIBUTES TIME PER DATE, WITH ONE DELIBERATE DEPARTURE.
		// The previous system gives weekend dates an empty team list, and
		// also empties days covered by leave (a half day included), an
		// exclusion or a confirmed allocation, so cards on those days
		// vanished from its report. We keep the weekend rule
		// (worklogDropWeekendCards) but NOT the absence rule: time an
		// engineer actually logged on a day marked absent is counted, as
		// it was real work. This is a product decision, not parity. The
		// daily report is a single date and applies its own presence
		// rules further down; it is unchanged by this.
		if !daily && worklogDropWeekendCards && isWeekend(t.WorkDate) {
			continue
		}

		bucketType := worklogBucketType(t.ItemType, t.Severity)
		ta := u.types[bucketType]
		if ta == nil {
			ta = &worklogTypeAgg{}
			u.types[bucketType] = ta
		}
		// Issue identity is the (person, work item) pair, so the same
		// item worked by two people counts once for each -- which is
		// what makes the team totals in the delivered mail reconcile.
		//
		// A person's approved and pending counts are kept apart and then
		// ADDED, so an item they logged against in both states counts
		// twice. ServiceNow does the same: its team total is
		// approved.nCases + pending.nCases over two separate maps. It
		// overstates, but it is the figure the mail prints.
		if approved {
			u.approvedMin += t.Minutes
			u.approvedKeys[t.ItemID] = true
			ta.approvedMin += t.Minutes
			ta.approvedDays++
		} else {
			u.pendingMin += t.Minutes
			u.pendingKeys[t.ItemID] = true
			ta.pendingMin += t.Minutes
			ta.pendingDays++
		}
		if !u.seenKey[t.ItemID] && t.IssueKey != "" {
			u.seenKey[t.ItemID] = true
			u.orderedKeys = append(u.orderedKeys, domain.WorklogIssueRef{ID: t.ItemID, Key: t.IssueKey})
		}
	}

	if len(droppedMinutes) > 0 {
		var total int
		for _, m := range droppedMinutes {
			total += m
		}
		slog.WarnContext(ctx, "worklog report: time logged by people on no team in scope is left out",
			"cadence", string(req.Cadence), "from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"),
			"people", len(droppedMinutes), "hours", minutesToHours(total))
	}

	weekdays := weekdaysBetween(from, to)

	teams := map[string]*domain.WorklogTeamReport{}
	// Hours are accumulated as whole minutes and converted once, never by
	// adding up already-rounded hours. Summing rounded values drifts: the
	// delivered weekly mail's Americas team is 10,810 minutes, which is
	// 180.17 hours, but adding its thirteen rounded developer figures
	// gives 180.18.
	teamMinutes := map[string]*[2]int{}
	// The daily report's divisor and numerators are built from presence
	// -- see where they are applied below.
	teamPresence := map[string]float64{}
	teamDailyApprovedMin := map[string]int{}
	teamDailyMinutes := map[string]int{}
	var totalPresence float64
	var totals domain.WorklogTotals
	var totalApprovedMin, totalPendingMin int
	// AVERAGE PEOPLE PER DAY counts people who were AVAILABLE, not people
	// who logged time. summerizeTimeReport pushes one entry per
	// (member, date) still present in teamInfo -- every day the member
	// was on a team and not away -- then divides the total by
	// getRoundedDayPart(from, to), which is 6 for a Monday-to-Sunday
	// range: the gap between the endpoints, so neither the range's seven
	// days nor its five working days.
	var rosteredMemberDays float64

	for _, u := range users {
		// A person appears when they were rostered in range, logged time,
		// or were away. One rule serves both cadences, and it is the only
		// one that explains the delivered mails: the weekly one lists 79
		// people including vikt@ who logged nothing, and the daily one
		// for Sunday 2026-10-18 lists just six -- the weekend crew, two of
		// whom (dara@, bruno@) show no time, no leave and no rejected
		// entries and could only have been drawn from the rota.
		//
		// The union degrades safely: where no rota rows exist for a range,
		// a report still lists everyone who logged time or took leave
		// rather than coming back empty.
		if daily && u.excluded {
			continue
		}
		// A weekday daily applies no rota gate at all -- the roster IS
		// the membership, which is why rotaGated is false for it and
		// this whole branch is skipped. For the weekly, and for a
		// weekend daily, a person still appears if they logged time or
		// were away even without a rota row: the union degrades safely
		// where no rota rows exist for a range, rather than returning
		// an empty report.
		if rotaGated && !isRostered[u.roster.UserID] && u.approvedMin == 0 && u.pendingMin == 0 &&
			len(u.approvedKeys) == 0 && len(u.pendingKeys) == 0 &&
			u.disapproved == 0 && u.leaveCode == "" {
			continue
		}

		// Weekdays on which the person has a team: the weekly's Worked
		// Days, and what makes them a member of the weekly at all.
		workedDays := weekdays - float64(len(u.blocked))
		if workedDays < 0 {
			workedDays = 0
		}
		if !daily && workedDays == 0 && len(u.approvedKeys) == 0 && len(u.pendingKeys) == 0 {
			// A person with no surviving weekday and no counted card is
			// in no team and appears nowhere, as in the previous system.
			// Unlike it, one who logged counted time on an absent day is
			// still listed: absences feed Worked Days and the leave
			// figures but never remove logged time.
			continue
		}
		// Only now is the person certainly in the report: fold their type
		// tallies into the breakdown.
		for typ, pt := range u.types {
			ta := byType[typ]
			if ta == nil {
				ta = &worklogTypeAgg{}
				byType[typ] = ta
			}
			ta.approvedMin += pt.approvedMin
			ta.pendingMin += pt.pendingMin
			ta.approvedDays += pt.approvedDays
			ta.pendingDays += pt.pendingDays
		}
		rosteredMemberDays += workedDays
		approvedHrs := minutesToHours(u.approvedMin)
		pendingHrs := minutesToHours(u.pendingMin)
		totalHrs := minutesToHours(u.approvedMin + u.pendingMin)

		dev := domain.WorklogDeveloper{
			UserID:             u.roster.UserID,
			Email:              u.roster.Email,
			Name:               u.roster.Name,
			ApprovedIssues:     len(u.approvedKeys),
			ApprovedHours:      approvedHrs,
			PendingIssues:      len(u.pendingKeys),
			PendingHours:       pendingHrs,
			Issues:             u.orderedKeys,
			TotalHours:         totalHrs,
			DisapprovedEntries: u.disapproved,
		}
		if daily {
			dev.Leave = u.leaveCode
		} else {
			dev.WorkedDays = workedDays
			dev.AvgTimePerDay = safeDiv(totalHrs, workedDays)
		}

		tr := teams[u.roster.TeamKey]
		if tr == nil {
			tr = &domain.WorklogTeamReport{
				TeamKey:  u.roster.TeamKey,
				TeamName: u.roster.TeamName,
				TeamType: u.roster.TeamType,
			}
			teams[u.roster.TeamKey] = tr
		}
		tr.Members++
		tr.Issues += dev.ApprovedIssues + dev.PendingIssues
		tr.Developers = append(tr.Developers, dev)
		tm := teamMinutes[u.roster.TeamKey]
		if tm == nil {
			tm = &[2]int{}
			teamMinutes[u.roster.TeamKey] = tm
		}
		tm[0] += u.approvedMin
		tm[1] += u.pendingMin

		// A present member counts 1, a half-day absence 0.5 and a full-day
		// absence 0. Leads are not on the roster (the repository leaves
		// role 'lead' out), so nothing is subtracted for them.
		presence := 1.0
		if u.leaveCode != "" {
			presence = 0
			if u.leaveHalf {
				presence = 0.5
			}
		}
		teamPresence[u.roster.TeamKey] += presence
		totalPresence += presence
		// The hours behind the daily average: nobody fully absent
		// contributes, but a half-day absentee does in full.
		if presence > 0 {
			teamDailyApprovedMin[u.roster.TeamKey] += u.approvedMin
			teamDailyMinutes[u.roster.TeamKey] += u.approvedMin + u.pendingMin
		}

		totals.Members++
		totals.Issues += dev.ApprovedIssues + dev.PendingIssues
		totalApprovedMin += u.approvedMin
		totalPendingMin += u.pendingMin
		if u.leaveCode != "" {
			totals.OnLeaveMembers++
		}
	}

	totals.ApprovedHours = minutesToHours(totalApprovedMin)
	totals.PendingHours = minutesToHours(totalPendingMin)
	totals.TotalHours = minutesToHours(totalApprovedMin + totalPendingMin)
	totals.AvgHoursPerPerson = safeDiv(totals.TotalHours, float64(totals.Members))
	totals.AvgApprovedHoursPerPerson = safeDiv(totals.ApprovedHours, float64(totals.Members))
	totals.AvgPendingHoursPerPerson = safeDiv(totals.PendingHours, float64(totals.Members))
	// totNonLeave, which the daily mail prints as "Non Leave Members".
	totals.NonLeaveMembers = round2(totalPresence)
	if !daily {
		totals.AveragePeoplePerDay = safeDiv(rosteredMemberDays, to.Sub(from).Hours()/24)
	}

	report := domain.WorklogReport{
		Cadence: req.Cadence,
		From:    from.Format("2006-01-02"),
		To:      to.Format("2006-01-02"),
	}

	for key, tr := range teams {
		tm := teamMinutes[key]
		tr.ApprovedHours = minutesToHours(tm[0])
		tr.PendingHours = minutesToHours(tm[1])
		tr.TotalHours = minutesToHours(tm[0] + tm[1])
		// PER-TEAM AVERAGES. The previous system's printed averages are
		// defective and are deliberately NOT reproduced:
		//
		//   - its weekly average approved figure is 0.00 for every team,
		//     because the approved branch adds a non-lead's approved time
		//     to the pending accumulator, so approved hours / person is
		//     wrong whenever a team has approved time;
		//   - it divides by team members less one "to exclude the lead",
		//     although leads are already out of its roster, so it counts
		//     the lead twice, and divides a single-member team by zero.
		//
		// Here every average is hours / a single divisor, the number of
		// rostered members who count, and never NaN or Infinity (safeDiv
		// gives 0 for an empty team; a one-member team divides by 1).
		//
		// WEEKLY: the divisor is the team's members, the numerators the
		// team's approved and total hours.
		// DAILY: the divisor is presence (1 per present member, 0.5 for a
		// half day, 0 for a full day) and the numerators are the approved
		// and total hours of those who count -- the same shape, so the
		// two averages share one divisor on each cadence.
		avgApprovedHrs, avgTotalHrs := tr.ApprovedHours, tr.TotalHours
		tr.MembersForAverage = float64(tr.Members)
		if daily {
			tr.MembersForAverage = round2(teamPresence[key])
			avgApprovedHrs = minutesToHours(teamDailyApprovedMin[key])
			avgTotalHrs = minutesToHours(teamDailyMinutes[key])
		}
		tr.AvgApprovedHoursPerPerson = safeDiv(avgApprovedHrs, tr.MembersForAverage)
		tr.AvgTotalHoursPerPerson = safeDiv(avgTotalHrs, tr.MembersForAverage)
		sort.Slice(tr.Developers, func(i, j int) bool {
			return tr.Developers[i].Email < tr.Developers[j].Email
		})
		report.Teams = append(report.Teams, *tr)
	}
	sort.Slice(report.Teams, func(i, j int) bool {
		return report.Teams[i].TeamName < report.Teams[j].TeamName
	})

	for _, typ := range orderedTypes(byType) {
		ta := byType[typ]
		approvedHrs := minutesToHours(ta.approvedMin)
		pendingHrs := minutesToHours(ta.pendingMin)
		label := worklogTypeLabels[typ]
		if label == "" {
			label = typ
		}
		bucket := domain.WorklogTypeBucket{
			Type:          typ,
			Label:         label,
			Issues:        ta.approvedDays + ta.pendingDays,
			ApprovedHours: approvedHrs,
			PendingHours:  pendingHrs,
			TotalHours:    round2(approvedHrs + pendingHrs),
		}
		report.ByType = append(report.ByType, bucket)
		switch typ {
		case "CASE":
			totals.QueryIssues = bucket.Issues
		case "INCIDENT":
			totals.IncidentIssues = bucket.Issues
		}
	}

	report.Totals = totals
	report.Recipients = deriveWorklogRecipients(contacts, audienceRuleFor(req.Cadence))
	return report, nil
}

// WorklogAudienceRule says which teams contribute recipients, and how.
// The two reports do not share an audience and must not be given one: the
// delivered weekly mail goes to five managers and two org-wide lists,
// while the delivered daily mail reaches those same managers plus roughly
// eighteen more people and four per-area lists.
//
// A team contributes by TYPE, so adding a team to an ABT adds its lead to
// the daily report without anyone editing configuration.
type WorklogAudienceRule struct {
	// MemberTeamTypes contribute every member, whatever their role. This
	// is how the leadership team puts its managers on both reports.
	MemberTeamTypes []string
	// LeadTeamTypes contribute only members with role 'lead'.
	LeadTeamTypes []string
	// GroupTeamTypes contribute the team's group_email to Cc.
	GroupTeamTypes []string
}

// The delivery teams: the two ABTs plus the standalone regional teams.
var worklogDeliveryTeamTypes = []string{"cre-abt", "sre-abt", "cre"}

// audienceRuleFor returns the rule for a cadence.
//
// WEEKLY goes to management and the two org-wide lists only -- no
// individual team leads. DAILY adds every delivery team's lead and their
// per-area group addresses. That asymmetry is the whole point: it is what
// the delivered mails do, and the two audiences differing is a
// requirement, not an accident.
func audienceRuleFor(c domain.WorklogReportCadence) WorklogAudienceRule {
	if c == domain.WorklogReportCadenceWeekly {
		return WorklogAudienceRule{
			MemberTeamTypes: []string{"cre-leadership"},
			GroupTeamTypes:  []string{"cre-leadership"},
		}
	}
	return WorklogAudienceRule{
		MemberTeamTypes: []string{"cre-leadership"},
		LeadTeamTypes:   worklogDeliveryTeamTypes,
		GroupTeamTypes:  append([]string{"cre-leadership"}, worklogDeliveryTeamTypes...),
	}
}

// deriveWorklogRecipients applies a rule to the team registry, each
// address appearing once and in a stable order.
func deriveWorklogRecipients(contacts []repository.WorklogTeamContacts, rule WorklogAudienceRule) domain.WorklogRecipients {
	member := typeSet(rule.MemberTeamTypes)
	lead := typeSet(rule.LeadTeamTypes)
	group := typeSet(rule.GroupTeamTypes)

	seenTo := map[string]bool{}
	seenCc := map[string]bool{}
	var out domain.WorklogRecipients

	add := func(dst *[]string, seen map[string]bool, addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" || seen[strings.ToLower(addr)] {
			return
		}
		seen[strings.ToLower(addr)] = true
		*dst = append(*dst, addr)
	}

	for _, c := range contacts {
		t := strings.ToLower(strings.TrimSpace(c.TeamType))
		if member[t] {
			for _, e := range c.MemberEmails {
				add(&out.To, seenTo, e)
			}
		}
		if lead[t] {
			for _, e := range c.LeadEmails {
				add(&out.To, seenTo, e)
			}
		}
		if group[t] {
			add(&out.Cc, seenCc, c.GroupEmail)
		}
	}
	sort.Strings(out.To)
	sort.Strings(out.Cc)
	return out
}

func typeSet(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[strings.ToLower(strings.TrimSpace(s))] = true
	}
	return out
}

// orderedTypes lists the types present, those the mails print first in
// their printed order, anything else alphabetically after them.
func orderedTypes[T any](m map[string]T) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range worklogTypeOrder {
		if _, ok := m[t]; ok {
			seen[t] = true
			out = append(out, t)
		}
	}
	var rest []string
	for t := range m {
		if !seen[t] {
			rest = append(rest, t)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// reseatMovedMembers files a person moved to another team by a Team
// Schedule span (Brazil rotation onto Americas, Migration, India team)
// under that team instead of the one their membership names. A move never
// changes membership (migrations 0202, 0274), so without this the report
// keeps them under the team they were moved away from.
//
// Leave taken during a move is part of it: the repository returns leave filed
// under a move's team as a span on that team (see worklogMoveSpansQuery), so
// a person on leave from the Migration team is listed under Migration, with
// their leave, not under their own team.
//
// The daily report uses the move covering its day. The weekly uses the
// team a move covers for more than half of the range's weekdays; less than
// that and they stay with their own team. A move to a team outside the
// report's scope (another type, or an excluded key) is ignored, so they
// keep their own team rather than vanish. Only people already on the
// roster are moved: nobody is added or dropped.
func (s *worklogReportService) reseatMovedMembers(users map[string]*perUser,
	moves []repository.WorklogMoveRow, from, to time.Time, daily bool) {
	weekdays := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if !isWeekend(d) {
			weekdays++
		}
	}
	type target struct {
		row  repository.WorklogMoveRow
		days map[time.Time]bool
	}
	byUser := map[string]map[string]*target{}
	for _, m := range moves {
		if _, ok := users[m.UserID]; !ok || !s.teamInScope(m.TeamKey, m.TeamType) {
			continue
		}
		covered, onFrom := coveredWeekdays(repository.WorklogLeaveRow{
			StartsOn: m.StartsOn, EndsOn: m.EndsOn}, from, to)
		if daily && !onFrom {
			continue
		}
		perTeam := byUser[m.UserID]
		if perTeam == nil {
			perTeam = map[string]*target{}
			byUser[m.UserID] = perTeam
		}
		t := perTeam[m.TeamKey]
		if t == nil {
			t = &target{row: m, days: map[time.Time]bool{}}
			perTeam[m.TeamKey] = t
		}
		for _, d := range covered {
			t.days[d] = true
		}
	}
	for userID, perTeam := range byUser {
		var best *target
		for _, t := range perTeam {
			if best == nil || len(t.days) > len(best.days) ||
				(len(t.days) == len(best.days) && t.row.TeamKey < best.row.TeamKey) {
				best = t
			}
		}
		if best == nil || (!daily && 2*len(best.days) <= weekdays) {
			continue
		}
		u := users[userID]
		u.roster.TeamKey = best.row.TeamKey
		u.roster.TeamName = best.row.TeamName
		u.roster.TeamType = best.row.TeamType
	}
}

// teamInScope reports whether a team belongs in this report: its type is
// one of teamTypes (any type when none is set) and its key is not excluded,
// the same rule the roster query applies.
func (s *worklogReportService) teamInScope(key, teamType string) bool {
	for _, k := range s.excludedTeamKeys {
		if k == key {
			return false
		}
	}
	if len(s.teamTypes) == 0 {
		return true
	}
	for _, t := range s.teamTypes {
		if strings.EqualFold(strings.TrimSpace(t), teamType) {
			return true
		}
	}
	return false
}

// coveredWeekdays returns the report's weekdays this absence covers, and
// whether it covers the first day of the range (which is the only day the
// daily report cares about).
//
// A half day covers its whole day. __internal_excludeUnavailableDays
// splices the day's entries away for time_off_morning and time_off_evening
// exactly as it does for a full time_off, so on the weekly the day stops
// counting either way; the daily keeps its own half-day handling through
// leaveHalf and does not read these days.
func coveredWeekdays(l repository.WorklogLeaveRow, from, to time.Time) ([]time.Time, bool) {
	start := dateOnly(l.StartsOn)
	if start.Before(from) {
		start = from
	}
	end := to
	if !l.EndsOn.IsZero() {
		e := dateOnly(l.EndsOn)
		if e.Before(end) {
			end = e
		}
	}
	var days []time.Time
	coversFrom := false
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		// coversFrom is decided BEFORE the weekend test, deliberately.
		// Worked days only ever count weekdays, so a weekend absence
		// adds nothing to the weekly total -- but the daily report runs
		// seven days a week, and its reference mail is a Sunday. Taking
		// the weekend shortcut first meant a Saturday or Sunday daily
		// report applied no leave and honoured no exclusion at all: the
		// weekend crew would have shown up regardless of being away.
		if d.Equal(from) {
			coversFrom = true
		}
		if isWeekend(d) {
			continue
		}
		days = append(days, d)
	}
	return days, coversFrom
}

// worklogBucketType is the type bucket a time card's work item falls in.
//
// The previous system buckets case time by the case's own type, where a
// high-severity case (S0-S3) is an Incident and an S4 case is a Query.
// The split reproduces that system's printed Incident and Query figures
// from the same cards. Without it every case lands under Query and the
// Incident bucket reads zero. Work items that are not cases keep their own
// type.
func worklogBucketType(itemType, severity string) string {
	if itemType != "CASE" {
		return itemType
	}
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "S0", "S1", "S2", "S3":
		return "INCIDENT"
	}
	return itemType
}

// weekdaysBetween counts Monday-to-Friday days in the inclusive range.
// Public holidays are not deducted. They are stored per country since
// migration 0283 (team_schedule_public_holiday, PublicHolidayService), but
// whether the worklog should deduct them is still to be decided; until then
// the delivered mail's worked-day figures top out at 5, as before.
func weekdaysBetween(from, to time.Time) float64 {
	var n float64
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if !isWeekend(d) {
			n++
		}
	}
	return n
}

func isWeekend(d time.Time) bool {
	return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func minutesToHours(m int) float64 {
	return round2(float64(m) / 60)
}

// safeDiv never returns NaN or Infinity. The delivered weekly mail does
// both, for a team with one member; see the package notes above.
func safeDiv(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return round2(num / den)
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
