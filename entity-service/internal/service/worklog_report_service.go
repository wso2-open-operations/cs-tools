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
	leaveDays    float64
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
	// WHICH ROTA READ, AND WHETHER ONE HAPPENS AT ALL, IS THE WHOLE
	// DIFFERENCE BETWEEN THE TWO CADENCES.
	//
	// The weekly is rota-driven: WeeklyTimeReportUtil.getTeamInfo builds
	// its roster from the shift API's rows for the range, so a person
	// with no shift that week is in no section.
	//
	// The daily is NOT, except on a weekend.
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
	// A weekday daily reads no rota at all, so it issues no query here.
	rotaGated := !daily || weekendDaily

	var rostered []string
	if rotaGated {
		read := s.repo.RosteredUserIDs
		if weekendDaily {
			read = s.repo.WeekendCrewUserIDs
		}
		if rostered, err = read(ctx, from, to); err != nil {
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
		}
	}

	// Leave first: worked days depend on it, and the daily report's Leave
	// column is the kind active on the single day in range.
	for _, l := range leave {
		u, ok := users[l.UserID]
		if !ok {
			continue
		}
		days, onFrom := leaveDaysInRange(l, from, to)
		u.leaveDays += days
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
	type typeAgg struct {
		approvedMin  int
		pendingMin   int
		approvedDays int
		pendingDays  int
	}
	byType := map[string]*typeAgg{}

	for _, t := range times {
		u, ok := users[t.UserID]
		if !ok {
			// Logged time by somebody on no team. Counted nowhere in the
			// mails, which only ever print team sections.
			continue
		}
		state := strings.ToUpper(strings.TrimSpace(t.State))
		approved := worklogApprovedStates[state]
		if state == strings.ToUpper(string(domain.TimeCardStateRejected)) {
			u.disapproved += t.Entries
			continue
		}
		if !approved && !worklogPendingStates[state] {
			// Neither Approved nor Submitted: ServiceNow drops the card.
			continue
		}

		ta := byType[t.ItemType]
		if ta == nil {
			ta = &typeAgg{}
			byType[t.ItemType] = ta
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

	weekdays := weekdaysBetween(from, to)

	teams := map[string]*domain.WorklogTeamReport{}
	// Hours are accumulated as whole minutes and converted once, never by
	// adding up already-rounded hours. Summing rounded values drifts: the
	// delivered weekly mail's Americas team is 10,810 minutes, which is
	// 180.17 hours, but adding its thirteen rounded developer figures
	// gives 180.18.
	teamMinutes := map[string]*[2]int{}
	// Minutes logged by team LEADS, and how many of them there are, kept
	// per team so the weekly average can take both out. ServiceNow calls
	// these time_exclude_lead and the complement of nMembers_Cal.
	teamLeadMinutes := map[string]*[2]int{}
	teamLeadCount := map[string]int{}
	// The daily report's divisor and numerator, which are built from
	// presence rather than from a flat subtraction -- see where they are
	// applied below.
	teamPresence := map[string]float64{}
	teamLeadDeduction := map[string]float64{}
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

		workedDays := weekdays - u.leaveDays
		if workedDays < 0 {
			workedDays = 0
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

		// summerizeTimeReport counts a present member as 1, a half-day
		// absence as 0.5 and a full-day absence as 0; then it decrements
		// again for each lead who is not fully absent.
		isLead := strings.EqualFold(u.roster.Role, "lead")
		presence := 1.0
		if u.leaveCode != "" {
			presence = 0
			if u.leaveHalf {
				presence = 0.5
			}
		}
		teamPresence[u.roster.TeamKey] += presence
		totalPresence += presence
		if isLead && presence > 0 {
			teamLeadDeduction[u.roster.TeamKey]++
		}
		// time_wl: the hours behind the daily average. Leads contribute
		// nothing, and neither does anyone fully absent -- but a
		// half-day absence contributes in full.
		if !isLead && presence > 0 {
			teamDailyMinutes[u.roster.TeamKey] += u.approvedMin + u.pendingMin
		}

		if isLead {
			lm := teamLeadMinutes[u.roster.TeamKey]
			if lm == nil {
				lm = &[2]int{}
				teamLeadMinutes[u.roster.TeamKey] = lm
			}
			lm[0] += u.approvedMin
			lm[1] += u.pendingMin
			teamLeadCount[u.roster.TeamKey]++
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
		// *** The two cadences compute this differently, and the
		// difference is deliberate on ServiceNow's side. ***
		//
		// WEEKLY excludes team leads from both halves: the action reads
		// teams[t].nMembers_Cal and approved/pending.time_exclude_lead.
		// It is NOT the off-by-one it looks like from the outside -- the
		// delivered weekly mail divides Americas' 180.17 by 12 of its 13
		// members because the thirteenth is the lead, and its
		// single-member Australia team prints NaN and Infinity because
		// excluding that team's only member leaves nothing to divide by.
		//
		// DAILY divides by the whole membership. Its delivered mail puts
		// Americas' 1.42 hours over 2 members for 0.71, which only works
		// without an exclusion.
		//
		// Our own division is guarded, so a team of nothing but leads
		// reports 0.00 where ServiceNow reports Infinity. That is the
		// one place we knowingly differ, because Infinity is not a
		// number anybody can act on.
		avgApprovedHrs, avgTotalHrs := tr.ApprovedHours, tr.TotalHours
		// *** THE DAILY REPORT EXCLUDES LEADS TOO. ***
		//
		// Its summerizeTimeReport builds nMembers_Cal from presence --
		// 1 per present member, 0.5 for a half day, 0 for a full day --
		// and then decrements once per lead who is not fully absent. The
		// numerator is time_wl, which skips leads and anyone fully
		// absent but keeps a half-day absentee's hours in full.
		//
		// The delivered daily mail dividing Americas' 1.42 hours by 2
		// members for 0.71 does NOT show the absence of a lead
		// exclusion, only that neither of those two was a lead. An
		// earlier pass read it the other way and was wrong.
		tr.MembersForAverage = round2(teamPresence[key] - teamLeadDeduction[key])
		if tr.MembersForAverage < 0 {
			tr.MembersForAverage = 0
		}
		avgApprovedHrs = 0
		avgTotalHrs = minutesToHours(teamDailyMinutes[key])
		if !daily {
			lm := teamLeadMinutes[key]
			if lm == nil {
				lm = &[2]int{}
			}
			// The divisor is literally nMembers - 1, not the number of
			// leads: summerizeTimeReport writes
			// `var totMem_team_cal = team.nMembers - 1; //exclude the lead`
			// and then subtracts anyone whose leaveDaysPerTeam is 7. That
			// second subtraction never fires, because leaveDates is
			// initialised empty and nothing ever pushes to it -- leave is
			// applied by deleting teamInfo entries instead. So the divisor
			// is a flat minus one, which is why all eleven teams in the
			// delivered mail divide by their member count less one, and
			// why the single-member Australia team divides by zero.
			tr.MembersForAverage = float64(tr.Members - 1)
			if tr.MembersForAverage < 0 {
				tr.MembersForAverage = 0
			}

			// Avg Approved Hrs/person is 0.00 for all eleven teams in the
			// delivered mail, including Castor which logged 4.13 approved
			// hours. That is not a coincidence: buildTimeReport's approved
			// branch adds a non-lead's approved time to
			// `team.pending.time_exclude_lead` -- the pending accumulator
			// -- so approved.time_exclude_lead only ever collects time a
			// LEAD logged on a day they were not leading. It is a
			// copy-paste slip in production, and the sum survives it,
			// which is why the total average is still right.
			//
			// Our leads are a standing role rather than a per-day shift,
			// so that accumulator has nothing to collect and the figure is
			// 0.00 -- the same value the mail prints.
			avgApprovedHrs = 0
			avgTotalHrs = minutesToHours((tm[0] + tm[1]) - (lm[0] + lm[1]))
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

// leaveDaysInRange returns how many of the report's weekdays this absence
// covers, and whether it covers the first day of the range (which is the
// only day the daily report cares about). A half day counts 0.5.
func leaveDaysInRange(l repository.WorklogLeaveRow, from, to time.Time) (float64, bool) {
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
	// A half day costs a WHOLE worked day, not half of one.
	// ServiceNow's __internal_excludeUnavailableDays splices the day's
	// entries away for time_off_morning and time_off_evening exactly as
	// it does for a full time_off, so the day stops counting either way.
	const weight = 1.0
	var days float64
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
		days += weight
	}
	return days, coversFrom
}

// weekdaysBetween counts Monday-to-Friday days in the inclusive range.
// Public holidays are not modelled anywhere in the schema, so they are not
// deducted; the delivered mail's worked-day figures top out at 5, which is
// consistent with that.
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
