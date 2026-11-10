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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The clouds the dashboard serves, and the only values the endpoints accept,
// are the active rows of the cloud registry (cloud_registry); see
// migration 0282. Seven are seeded: asgardeo, choreo, bijira, devant, moesif,
// choreo-eu and agent-manager. The previous backend's monitors script knew
// five and answered the rest with a 400; this one serves every registered
// cloud, so adding a cloud is data, not code.
type cloudStatusDashboardService struct {
	repo     repository.CloudStatusDashboardRepository
	registry *cloudRegistry
	// now is injectable so the incident window can be tested without
	// freezing a clock.
	now func() time.Time
}

// NewCloudStatusDashboardService constructs the dashboard read service.
func NewCloudStatusDashboardService(repo repository.CloudStatusDashboardRepository) CloudStatusDashboardService {
	return &cloudStatusDashboardService{repo: repo, registry: newCloudRegistry(repo.Clouds), now: time.Now}
}

// validateCloud resolves a dashboard cloud slug against the registry. An
// unknown or inactive cloud is a validation error listing the valid slugs.
func (s *cloudStatusDashboardService) validateCloud(ctx context.Context, cloud string) (domain.CloudDefinition, error) {
	catalog, err := s.registry.Catalog(ctx)
	if err != nil {
		return domain.CloudDefinition{}, err
	}
	def, ok := catalog.Lookup(cloud)
	if !ok {
		return domain.CloudDefinition{}, &apierror.ValidationError{
			Msg: "cloud must be one of " + strings.Join(catalog.Slugs(), ", "),
		}
	}
	return def, nil
}

// Monitors assembles the dashboard's monitor view for one cloud.
//
// Three reads instead of the script's one-plus-two-per-monitor: the monitors,
// then their availability and any ongoing outages batched by offering. On the
// busiest cloud that is 3 queries where ServiceNow issued 73.
func (s *cloudStatusDashboardService) Monitors(ctx context.Context, cloud string) (domain.CloudStatusMonitorsResponse, error) {
	def, err := s.validateCloud(ctx, cloud)
	if err != nil {
		return nil, err
	}

	monitors, err := s.repo.Monitors(ctx, def.OfferingKey)
	if err != nil {
		return nil, err
	}

	offerings := make([]string, 0, len(monitors))
	seen := map[string]bool{}
	for _, m := range monitors {
		if m.ServiceOfferingID != "" && !seen[m.ServiceOfferingID] {
			seen[m.ServiceOfferingID] = true
			offerings = append(offerings, m.ServiceOfferingID)
		}
	}

	avail, err := s.repo.Availabilities(ctx, offerings)
	if err != nil {
		return nil, err
	}
	byOffering := map[string][]domain.CloudStatusAvailability{}
	for _, a := range avail {
		if a.Duration == "" {
			continue
		}
		byOffering[a.ServiceOfferingID] = append(byOffering[a.ServiceOfferingID],
			domain.CloudStatusAvailability{Availability: a.Availability, Duration: a.Duration})
	}

	outages, err := s.repo.OngoingOutages(ctx, offerings)
	if err != nil {
		return nil, err
	}

	// The response is built by APPENDING in row order, which is why the
	// repository's ORDER BY matters. A map would lose that, so groups are
	// tracked by index within each region.
	resp := domain.CloudStatusMonitorsResponse{}
	groupIndex := map[string]map[string]int{}

	for _, m := range monitors {
		status := domain.DashboardStatus(domain.CloudMonitorStatus(m.Status))

		// The message only attaches when an ongoing outage's type agrees with
		// the status shown -- see domain.MessageAgreesWithStatus for why that
		// is not redundant.
		message := ""
		for _, o := range outages {
			if o.ServiceOfferingID == m.ServiceOfferingID &&
				domain.MessageAgreesWithStatus(status, o.Type) {
				message = o.ShortDescription
				break
			}
		}

		sub := domain.CloudStatusMonitorSubgroup{
			Description:  m.Description,
			DisplayName:  m.Name,
			Status:       status,
			Availability: byOffering[m.ServiceOfferingID],
			Message:      message,
		}
		if sub.Availability == nil {
			// The dashboard iterates this; a null would render as nothing at
			// best and throw at worst. The script always emits an array.
			sub.Availability = []domain.CloudStatusAvailability{}
		}

		if _, ok := groupIndex[m.Region]; !ok {
			groupIndex[m.Region] = map[string]int{}
		}
		if idx, ok := groupIndex[m.Region][m.Group]; ok {
			resp[m.Region][idx].Subgroups = append(resp[m.Region][idx].Subgroups, sub)
			continue
		}
		resp[m.Region] = append(resp[m.Region], domain.CloudStatusMonitorGroup{
			DisplayName: m.Group,
			Subgroups:   []domain.CloudStatusMonitorSubgroup{sub},
		})
		groupIndex[m.Region][m.Group] = len(resp[m.Region]) - 1
	}
	return resp, nil
}

// incidentMonths is how many months of history the dashboard shows.
const incidentMonths = 6

// Incidents assembles the dashboard's incident history for one cloud.
//
// The response always carries SIX month keys, populated or not, because the
// script pre-seeds them and the frontend renders a row per key. Returning only
// the months that have incidents would silently shorten the page.
func (s *cloudStatusDashboardService) Incidents(ctx context.Context, cloud string) (domain.CloudStatusIncidentsResponse, error) {
	def, err := s.validateCloud(ctx, cloud)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()

	// *** SEED FROM THE FIRST OF THE MONTH, NOT FROM TODAY. ***
	// now.AddDate(0, -i, 0) normalises a day that the target month does not
	// have: on 2026-03-31, i=1 asks for 2026-02-31 and Go returns
	// 2026-03-03. The key "2026-3" is then written twice, "2026-2" is never
	// created, and every incident from February is dropped at the lookup
	// below -- so on the 29th, 30th and 31st the public page silently loses
	// a month and shows fewer than six rows. Anchoring to day 1 makes the
	// arithmetic exact for every month.
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	resp := domain.CloudStatusIncidentsResponse{}
	for i := 0; i < incidentMonths; i++ {
		resp[incidentMonthKey(firstOfMonth.AddDate(0, -i, 0))] = domain.CloudStatusIncidentMonth{
			Incidents: []domain.CloudStatusIncident{},
		}
	}

	// The script's window is gs.beginningOfLast2Quarters(). Six months back
	// from the first of the current month covers the same span and matches
	// the six keys above, which the quarter boundary does not always do.
	since := firstOfMonth.AddDate(0, -(incidentMonths - 1), 0)

	rows, err := s.repo.Incidents(ctx, def.IncidentPrefix, since)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		begin, err := time.Parse("2006-01-02 15:04:05", r.Begin)
		if err != nil {
			continue
		}
		key := incidentMonthKey(begin)
		month, ok := resp[key]
		if !ok {
			// Outside the six seeded months. The script drops these too --
			// its query window and its key seeding can disagree at a quarter
			// boundary, and an unkeyed incident simply vanishes.
			continue
		}
		month.Incidents = append(month.Incidents, domain.CloudStatusIncident{
			// Dashless, as ServiceNow emitted it -- see domain.SysID.
			ID:               domain.SysID(r.ID),
			Begin:            r.Begin,
			End:              r.End,
			Type:             incidentTypeLabel(r.Type),
			Status:           incidentStatus(r.End),
			ShortDescription: r.ShortDescription,
			Expanded:         false,
		})
		resp[key] = month
	}
	return resp, nil
}

// incidentMonthKey renders the "YYYY-M" key. Deliberately not zero-padded:
// the script builds it by concatenation and the frontend matches on it.
func incidentMonthKey(t time.Time) string {
	return t.Format("2006") + "-" + strings.TrimPrefix(t.Format("01"), "0")
}

// incidentTypeLabel renders the stored enum the way the dashboard prints it.
//
// *** THESE ARE NOT TITLE-CASED ENUM NAMES, AND TWO OF THEM ARE RENAMED. ***
//
//	outage      -> Outage
//	degradation -> Degraded      (NOT "Degradation")
//	planned     -> Maintenance   (NOT "Planned")
//
// An earlier version of this function title-cased the enum, which is right
// for the first and wrong for the other two. It went unnoticed because every
// incident on the dev instance is of type outage, so a diff against the live
// API exercised only the branch that happened to be correct. Both ServiceNow
// resource scripts -- incidents.js and incident.js -- agree on the renames.
func incidentTypeLabel(t string) string {
	switch strings.ToUpper(t) {
	case "OUTAGE":
		return "Outage"
	case "DEGRADATION":
		return "Degraded"
	case "PLANNED":
		return "Maintenance"
	default:
		return ""
	}
}

// incidentStatus is derived, not stored: an outage with an end is Resolved.
// incidentStatus renders the two states the incident LIST shows.
//
// *** THE ONGOING LABEL IS "In Progress", NOT "Ongoing". *** Both ServiceNow
// scripts set it the same way:
//
//	var state = 'In Progress';
//	if (end) { state = 'Resolved'; }
//
// An earlier version returned "Ongoing", which reads naturally and is not
// what the frontend receives. It survived every diff because every incident
// on the dev instance inside the six-month window had an end date, so only
// the Resolved branch was ever compared. It surfaced the moment an ongoing
// outage was published to the history.
//
// That is the third bug of this shape in this port -- the others were the
// type labels and the id format. The pattern is always the same: a branch
// the live data never exercises is a branch the diff never checks.
func incidentStatus(end string) string {
	if end == "" {
		return "In Progress"
	}
	return "Resolved"
}

// ── /availabilities ────────────────────────────────────────────────────

// Availabilities assembles one cloud's weighted uptime, region by region.
//
// One query per parent service, which is exactly the granularity ServiceNow
// works at: its query() is called once per parent and each calculateX only
// ever sees that parent's offerings. Doing it in one batched query and
// slicing in Go would be fewer round trips and would reproduce the same
// numbers -- but it would also hide the one structural fact that makes the
// weights mean anything, so the shape is kept.
func (s *cloudStatusDashboardService) Availabilities(ctx context.Context, cloud string) (domain.CloudAvailabilitiesResponse, error) {
	def, err := s.validateCloud(ctx, cloud)
	if err != nil {
		return nil, err
	}
	plan := def.Plan

	resp := domain.CloudAvailabilitiesResponse{}
	for _, region := range plan.Regions {
		rows, err := s.repo.ParentAvailabilities(ctx, region.ParentID)
		if err != nil {
			return nil, err
		}

		byWindow := map[string][]repository.ParentAvailabilityRow{}
		for _, r := range rows {
			byWindow[r.Window] = append(byWindow[r.Window], r)
		}

		windows := make([]domain.CloudAvailabilityWindow, 0, len(domain.AvailabilityWindows))
		for _, w := range domain.AvailabilityWindows {
			windows = append(windows, domain.CloudAvailabilityWindow{
				Availability: figureFor(plan, byWindow[w.Enum]),
				Duration:     w.Label,
			})
		}
		resp[region.Key] = windows
	}
	return resp, nil
}

// figureFor computes one window's figure for one region.
//
// Two paths, because ServiceNow has two: a weighted sum for six clouds and a
// plain mean for asgardeo, differing in their wire type as well as their
// arithmetic. See domain.AvailabilityFigure.
func figureFor(plan domain.CloudAvailabilityPlan, rows []repository.ParentAvailabilityRow) domain.AvailabilityFigure {
	if !plan.Weighted() {
		// `calculate`: arr.reduce(...) / arr.length, then parseFloat of the
		// fixed string -- so a NUMBER, with trailing zeros dropped.
		//
		// Its 100 -> 99.9999 clamp is COMMENTED OUT in the deployed script,
		// unlike the monitors endpoint where the same clamp for the same
		// cloud is live. Not reproduced here, deliberately: both figures are
		// published and they genuinely differ.
		if len(rows) == 0 {
			// The script's own default before any branch runs.
			return domain.AvailabilityFigure{Value: "100", Number: true}
		}
		var sum float64
		for _, r := range rows {
			sum += r.Availability
		}
		fixed := domain.JSToFixed(sum/float64(len(rows)), domain.AvailabilityPrecision)
		return domain.AvailabilityFigure{Value: domain.TrimJSNumber(fixed), Number: true}
	}

	// calculateX: running_count += availability * weight / 100, then toFixed
	// -- so a STRING, trailing zeros kept.
	//
	// *** UNMAPPED OFFERINGS ARE SKIPPED, NOT PROPAGATED. *** In JavaScript
	// map_weights[sno] on an absent key is undefined, availability *
	// undefined is NaN, and running_count stays NaN for good: ONE unweighted
	// offering silently turns that whole region's published uptime into the
	// string "NaN". Verified 2026-09-29 that no offering under any of the 18
	// parents is currently unmapped, so this path changes nothing today --
	// it stops a future offering from blanking a customer-facing page.
	var total float64
	for _, r := range rows {
		weight, ok := plan.Weights[r.ServiceOfferingID]
		if !ok {
			continue
		}
		total += r.Availability * weight / 100
	}
	return domain.AvailabilityFigure{
		Value: domain.JSToFixed(total, domain.AvailabilityPrecision),
	}
}

// ── /history ───────────────────────────────────────────────────────────

// availabilityHistoryTZ is the timezone the daily buckets are labelled in.
//
// ServiceNow renders each date with getDate().getDisplayValue(), which uses
// the CALLING USER's timezone, so the published dates depend on who asks.
// What that resolves to in practice was MEASURED, not assumed, and the
// decisive row is this one:
//
//	start_on 2026-09-26 18:30:00+00   ->  UTC 09-26,  Colombo 09-27
//
// The live API publishes it as 2026-09-27. Colombo it is -- which is also
// exactly where the buckets are cut, 18:30Z being its midnight.
//
// This was briefly switched to UTC on the strength of one wrong inference
// (that the oldest published point implied UTC labelling); the real cause
// was the window bound below, and the boundary row above settles it.
const availabilityHistoryTZ = "Asia/Colombo"

// AvailabilityHistory assembles one cloud's 90-day daily uptime chart.
func (s *cloudStatusDashboardService) AvailabilityHistory(ctx context.Context, cloud string) (domain.CloudAvailabilityHistoryResponse, error) {
	def, err := s.validateCloud(ctx, cloud)
	if err != nil {
		return nil, err
	}

	// The history script orders by group then name, and NOT by
	// group_priority, which the monitors script does. Reproduced as written:
	// the row order decides the order of groups on the page.
	monitors, err := s.repo.MonitorsForHistory(ctx, def.OfferingKey)
	if err != nil {
		return nil, err
	}

	offerings := make([]string, 0, len(monitors))
	seen := map[string]bool{}
	for _, m := range monitors {
		if m.ServiceOfferingID != "" && !seen[m.ServiceOfferingID] {
			seen[m.ServiceOfferingID] = true
			offerings = append(offerings, m.ServiceOfferingID)
		}
	}

	loc, err := time.LoadLocation(availabilityHistoryTZ)
	if err != nil {
		return nil, err
	}
	today := s.now().In(loc)
	todayKey := today.Format("2006-01-02")
	// *** THE WINDOW IS 92 CALENDAR DAYS, NOT 90, AND THAT WAS MEASURED. ***
	// gs.beginningOfLast90Days() does not mean "90 days back": diffed against
	// the live API on 2026-09-29, every monitor carried a point dated
	// 2026-06-30, which is today minus 91. Deriving the bound from the
	// function's NAME would have silently clipped the oldest two days off
	// every chart on the page.
	from := today.AddDate(0, 0, -(domain.AvailabilityHistoryDays + 1)).Format("2006-01-02")

	rows, err := s.repo.DailyAvailability(ctx, offerings, availabilityHistoryTZ, from, todayKey)
	if err != nil {
		return nil, err
	}
	byOffering := map[string][]domain.AvailabilityHistoryPoint{}
	for _, r := range rows {
		byOffering[r.ServiceOfferingID] = append(byOffering[r.ServiceOfferingID],
			domain.AvailabilityHistoryPoint{Availability: r.Availability, Date: r.Date})
	}

	resp := domain.CloudAvailabilityHistoryResponse{}
	groupIndex := map[string]map[string]int{}

	for _, m := range monitors {
		history := historyFor(byOffering[m.ServiceOfferingID], todayKey)
		sub := domain.AvailabilityHistorySubgroup{DisplayName: m.Name, History: history}

		if _, ok := groupIndex[m.Region]; !ok {
			groupIndex[m.Region] = map[string]int{}
		}
		if idx, ok := groupIndex[m.Region][m.Group]; ok {
			resp[m.Region][idx].Subgroups = append(resp[m.Region][idx].Subgroups, sub)
			continue
		}
		resp[m.Region] = append(resp[m.Region], domain.AvailabilityHistoryGroup{
			DisplayName: m.Group,
			Subgroups:   []domain.AvailabilityHistorySubgroup{sub},
		})
		groupIndex[m.Region][m.Group] = len(resp[m.Region]) - 1
	}
	return resp, nil
}

// historyFor applies the script's today-fill and its 90-point cap.
func historyFor(points []domain.AvailabilityHistoryPoint, todayKey string) []domain.AvailabilityHistoryPoint {
	if points == nil {
		// The script emits an array the frontend iterates; a null would
		// render as nothing at best.
		points = []domain.AvailabilityHistoryPoint{}
	}

	// *** THE TODAY-FILL. *** The script pushes {availability: 100, date:
	// today} when the LAST row it saw is not today, then dedups keeping the
	// first occurrence -- so the synthetic point survives only when today has
	// no real row. Its own comment calls it "a workaround to handle the
	// missing data". Reproduced: a chart that stops yesterday reads as an
	// outage today, which is the bug the workaround exists to avoid.
	//
	// Note it fires only when at least one real row exists. A monitor with no
	// daily data at all gets an empty history, not a fabricated 100.
	if len(points) > 0 {
		hasToday := false
		for _, p := range points {
			if p.Date == todayKey {
				hasToday = true
				break
			}
		}
		if !hasToday {
			points = append(points, domain.AvailabilityHistoryPoint{Availability: 100, Date: todayKey})
		}
	}

	// uniqueAvaialbility.slice(-90). The repository returns oldest first, so
	// this keeps the most recent 90 -- which is what slice(-90) means on a
	// chronological array, and what the chart is for. ServiceNow slices an
	// arbitrarily ordered array and therefore drops arbitrary days; 14
	// offerings currently carry 92 distinct days and lose two of them to
	// that. Ordering first makes the cap mean what it reads like.
	if len(points) > domain.AvailabilityHistoryDays {
		points = points[len(points)-domain.AvailabilityHistoryDays:]
	}
	return points
}

// ── /incident/{id} ─────────────────────────────────────────────────────

// incidentDetailQualifyingStates is ServiceNow's `stateNOT IN 1,3,8` in
// Postgres terms: everything except New (1), On Hold (3) and Canceled (8).
//
// An outage whose incident falls outside this set gets the attachments-only
// payload, exactly as it does today.
var incidentDetailExcludedStates = map[string]bool{
	"NEW":      true,
	"ON_HOLD":  true,
	"CANCELED": true,
}

// IncidentDetail returns one outage's public detail view.
//
// Returns nil when no outage with that id belongs to that cloud, which the
// handler renders as 404.
func (s *cloudStatusDashboardService) IncidentDetail(ctx context.Context, id, cloud string) (any, error) {
	def, err := s.validateCloud(ctx, cloud)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, &apierror.ValidationError{Msg: "id is required"}
	}

	// *** A MALFORMED ID IS A 404, NOT A 500. *** UUIDFromSysID returns
	// anything that is not 32 hex characters unchanged, so /incidents/abc
	// reached Postgres as $1::uuid and raised "invalid input syntax for type
	// uuid" -- which writeServiceError maps to 500 and logs as an internal
	// error. On a public endpoint that means every stale link and every
	// scanner probe is recorded as a server fault. An id that cannot name a
	// row is simply not found.
	outageID := domain.UUIDFromSysID(id)
	if !domain.LooksLikeUUID(outageID) {
		return nil, nil
	}

	row, err := s.repo.IncidentDetail(ctx, outageID, def.IncidentPrefix)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}

	// *** THE ATTACHMENTS-ONLY PAYLOAD IS THE COMMON CASE, NOT AN ERROR. ***
	// The script assigns attachments onto whatever its gated incident lookup
	// returned, so an outage with no linked incident -- or one whose
	// incident is still New, On Hold or Canceled -- yields an object with no
	// id, no begin and no type. That is 449 of 637 outages today, plus 17
	// more behind the state gate. Confirmed against the live API, which
	// answers an asgardeo outage with no work item with exactly
	// {"attachments":[]}.
	if row.IncidentState == "" || incidentDetailExcludedStates[row.IncidentState] {
		return domain.CloudStatusIncidentAttachmentsOnly{
			Attachments: []domain.CloudStatusIncidentAttachment{},
		}, nil
	}

	comments, err := s.repo.OutageComments(ctx, outageID)
	if err != nil {
		return nil, err
	}

	return domain.CloudStatusIncidentDetail{
		ID:               domain.SysID(row.ID),
		Begin:            row.Begin,
		End:              row.End,
		Type:             incidentTypeLabel(row.Type),
		Status:           domain.IncidentDetailStatus(row.End),
		ShortDescription: row.ShortDescription,
		Comments:         incidentComments(comments),

		// *** ATTACHMENTS ARE ALWAYS EMPTY, AND THAT IS NOT A GAP. ***
		// ServiceNow read PDFs off sys_attachment for the outage. Postgres
		// holds no outage attachments, nothing writes any, and -- decisively
		// -- the dashboard frontend never reads the field: it appears in no
		// component. Emitting the empty array keeps the payload shape the
		// contract promises without inventing a store for something with
		// neither a producer nor a consumer.
		Attachments: []domain.CloudStatusIncidentAttachment{},
	}, nil
}

// incidentComments turns the stored updates into the list the frontend
// iterates.
//
// *** THESE ARE THE OUTAGE'S EXTERNAL COMMUNICATIONS, NOT THE INCIDENT'S
// COMMENTS, AND THE DIFFERENCE IS A DISCLOSURE BOUNDARY. *** ServiceNow
// reads sys_journal_field for element `u_external_outage_communications`
// against the OUTAGE; the line that once read the incident's `comments` is
// commented out in the source. Postgres does hold incident comments -- 923
// on outage-linked incidents, plus work notes and approval history -- and
// serving those here would publish internal commentary on a public status
// page. Both live-served incidents carry such comments and the live API
// returns none of them.
//
// Always an array, never null: the frontend maps over it.
func incidentComments(rows []repository.OutageCommentRow) []domain.CloudStatusIncidentComment {
	out := make([]domain.CloudStatusIncidentComment, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.CloudStatusIncidentComment{
			Comment:   r.Comment,
			CreatedOn: r.CreatedOn,
		})
	}
	return out
}
