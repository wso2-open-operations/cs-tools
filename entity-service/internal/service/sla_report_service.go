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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SlaReportService builds the project SLA report from Postgres.
type SlaReportService interface {
	// GetProjectSlaReport returns the SLA report of the cases a project opened
	// between from and to (inclusive; a date-only `to` covers that whole day).
	GetProjectSlaReport(ctx context.Context, projectID, from, to string) (domain.SlaReportResponse, error)
}

type slaReportService struct {
	repo repository.SlaReportRepository
	now  func() time.Time
}

// NewSlaReportService constructs a SlaReportService backed by Postgres.
func NewSlaReportService(repo repository.SlaReportRepository) SlaReportService {
	return &slaReportService{repo: repo, now: time.Now}
}

type slaReportUnsupportedService struct{}

// NewSlaReportUnsupportedService is used when SLA data is read from
// ServiceNow (SLA_DATA_SOURCE=servicenow): the report reads the Postgres
// clocks, which are not the source of truth then.
func NewSlaReportUnsupportedService() SlaReportService {
	return slaReportUnsupportedService{}
}

func (slaReportUnsupportedService) GetProjectSlaReport(context.Context, string, string, string) (domain.SlaReportResponse, error) {
	return domain.SlaReportResponse{}, &apierror.ValidationError{Msg: "the SLA report is not supported when SLA_DATA_SOURCE=servicenow"}
}

func (s *slaReportService) GetProjectSlaReport(ctx context.Context, projectID, fromRaw, toRaw string) (domain.SlaReportResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.SlaReportResponse{}, err
	}
	if strings.TrimSpace(fromRaw) == "" || strings.TrimSpace(toRaw) == "" {
		return domain.SlaReportResponse{}, &apierror.ValidationError{Msg: "from and to are required"}
	}
	now := s.now().UTC()
	from, err := parseCaseFilterDate(domain.CaseFieldFilter{Field: "createdOn", Op: "gte"}, fromRaw, now)
	if err != nil {
		return domain.SlaReportResponse{}, err
	}
	to, err := parseCaseFilterDate(domain.CaseFieldFilter{Field: "createdOn", Op: "lte"}, toRaw, now)
	if err != nil {
		return domain.SlaReportResponse{}, err
	}
	if from.After(*to) {
		return domain.SlaReportResponse{}, &apierror.ValidationError{Msg: "from must be on or before to"}
	}

	project, err := s.repo.Project(ctx, projectID)
	if err != nil {
		return domain.SlaReportResponse{}, err
	}
	rows, err := s.repo.Clocks(ctx, projectID, *from, *to)
	if err != nil {
		return domain.SlaReportResponse{}, err
	}

	summary, cases := buildSlaReport(rows)
	return domain.SlaReportResponse{
		Project: project,
		From:    from.UTC().Format("2006-01-02"),
		To:      to.UTC().Format("2006-01-02"),
		Summary: summary,
		Cases:   cases,
	}, nil
}

// Clock statuses shown on a case row.
const (
	slaStatusMet       = "Met"
	slaStatusMissed    = "Missed"
	slaStatusRunning   = "Running"
	slaStatusPaused    = "Paused"
	slaStatusCancelled = "Cancelled"
)

const (
	slaCaseKindIncident = "incident"
	slaCaseKindQuery    = "query"
)

// slaClockTargets are the three clock targets, in the order the report shows them.
var slaClockTargets = []string{"response", "workaround", "resolution"}

type slaReportSeverity struct {
	code  string
	label string
}

var slaSeverities = []slaReportSeverity{
	{"S0", "Catastrophic (P0)"},
	{"S1", "Critical (P1)"},
	{"S2", "High (P2)"},
	{"S3", "Medium (P3)"},
	{"S4", "Low (P4)"},
}

// slaClockResult is the latest clock of one target on one case.
type slaClockResult struct {
	status        string
	finished      bool
	elapsed       *float64
	targetSeconds *float64
}

// slaCase is one case opened in the window with its clocks by target.
type slaCase struct {
	id, number, issue, severity, state string
	opened                             time.Time
	clocks                             map[string]slaClockResult
}

// classifySlaClock turns a live stage and breach flag into a status. A clock
// that finished within its target is Met, one that went past it (finished late
// or still running and already over) is Missed. Stages the report does not
// know give an empty status.
func classifySlaClock(stage string, hasBreached bool) (status string, finished bool) {
	switch strings.ToUpper(stage) {
	case "ACHIEVED", "COMPLETED":
		if hasBreached {
			return slaStatusMissed, true
		}
		return slaStatusMet, true
	case "BREACHED":
		return slaStatusMissed, false
	case "IN_PROGRESS":
		if hasBreached {
			return slaStatusMissed, false
		}
		return slaStatusRunning, false
	case "PAUSED":
		if hasBreached {
			return slaStatusMissed, false
		}
		return slaStatusPaused, false
	case "CANCELLED":
		return slaStatusCancelled, false
	default:
		return "", false
	}
}

func buildSlaReport(rows []repository.SlaReportClockRow) ([]domain.SlaReportSummaryRow, []domain.SlaReportCase) {
	order := make([]string, 0)
	byID := map[string]*slaCase{}
	for _, row := range rows {
		c := byID[row.CaseID]
		if c == nil {
			c = &slaCase{
				id: row.CaseID, number: row.CaseNumber, issue: row.IssueType, severity: row.Severity,
				state: row.State, opened: row.Opened, clocks: map[string]slaClockResult{},
			}
			byID[row.CaseID] = c
			order = append(order, row.CaseID)
		}
		if row.Target == nil || row.Stage == nil {
			continue
		}
		breached := row.HasBreached != nil && *row.HasBreached
		status, finished := classifySlaClock(*row.Stage, breached)
		c.clocks[strings.ToLower(*row.Target)] = slaClockResult{
			status: status, finished: finished, elapsed: row.ElapsedSeconds, targetSeconds: row.TargetSeconds,
		}
	}

	cases := make([]domain.SlaReportCase, 0, len(order))
	for _, id := range order {
		c := byID[id]
		cases = append(cases, domain.SlaReportCase{
			CaseID:     c.id,
			CaseNumber: c.number,
			CaseType:   slaCaseTypeLabel(c.issue),
			Severity:   slaReportSeverityLabel(c.severity),
			State:      slaReportStateLabel(c.state),
			Opened:     c.opened.UTC().Format(time.RFC3339),
			Response:   slaCaseClock(c.clocks["response"]),
			Workaround: slaCaseClock(c.clocks["workaround"]),
			Resolution: slaCaseClock(c.clocks["resolution"]),
		})
	}
	return buildSlaSummary(byID, order), cases
}

type slaGroupKey struct{ kind, severity string }

// buildSlaSummary groups the cases by case type and severity. Only groups with
// at least one case in the window get a row: Incident first, then Query, each by
// severity from S0.
func buildSlaSummary(byID map[string]*slaCase, order []string) []domain.SlaReportSummaryRow {
	groups := map[slaGroupKey][]*slaCase{}
	for _, id := range order {
		c := byID[id]
		key := slaGroupKey{kind: slaCaseKind(c.issue), severity: strings.ToUpper(strings.TrimSpace(c.severity))}
		groups[key] = append(groups[key], c)
	}

	keys := make([]slaGroupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind == slaCaseKindIncident
		}
		ri, rj := slaSeverityRank(keys[i].severity), slaSeverityRank(keys[j].severity)
		if ri != rj {
			return ri < rj
		}
		return keys[i].severity < keys[j].severity
	})

	summary := make([]domain.SlaReportSummaryRow, 0, len(keys))
	for _, key := range keys {
		members := groups[key]
		kindLabel := "Incident"
		if key.kind == slaCaseKindQuery {
			kindLabel = "Query"
		}
		severityLabel := "No severity"
		if key.severity != "" {
			severityLabel = slaReportSeverityLabel(key.severity)
		}
		summary = append(summary, domain.SlaReportSummaryRow{
			Label:      kindLabel + " - " + severityLabel,
			CaseType:   key.kind,
			Severity:   key.severity,
			Cases:      len(members),
			Response:   slaMetric(members, "response"),
			Workaround: slaMetric(members, "workaround"),
			Resolution: slaMetric(members, "resolution"),
		})
	}
	return summary
}

// slaMetric builds one cell: the 95th percentile of the finished clocks, the
// policy target, and the Met / Missed / Running counts.
func slaMetric(members []*slaCase, target string) domain.SlaReportMetric {
	metric := domain.SlaReportMetric{Elapsed: "—", Target: "Not applicable"}
	var finished []float64
	targets := map[int64]int{}
	for _, c := range members {
		clock, ok := c.clocks[target]
		if !ok {
			continue
		}
		switch clock.status {
		case slaStatusMet:
			metric.Met++
		case slaStatusMissed:
			metric.Missed++
		case slaStatusRunning, slaStatusPaused:
			metric.Running++
		}
		if clock.finished && clock.elapsed != nil {
			finished = append(finished, *clock.elapsed)
		}
		if clock.targetSeconds != nil && *clock.targetSeconds > 0 {
			targets[int64(math.Round(*clock.targetSeconds))]++
		}
	}
	if seconds, ok := mostCommonDuration(targets); ok {
		metric.Target = formatReportDuration(float64(seconds))
	}
	if len(finished) > 0 {
		sort.Float64s(finished)
		metric.Elapsed = formatReportDuration(percentileCont(finished, 0.95))
	}
	return metric
}

// mostCommonDuration returns the duration most clocks run against. Policies
// can differ inside one severity (for example by product), so the report shows
// the one that applies to most of the cases; the larger wins a tie.
func mostCommonDuration(counts map[int64]int) (int64, bool) {
	var best int64
	bestCount := 0
	for seconds, count := range counts {
		if count > bestCount || (count == bestCount && seconds > best) {
			best, bestCount = seconds, count
		}
	}
	return best, bestCount > 0
}

func slaCaseClock(clock slaClockResult) domain.SlaReportClock {
	if clock.status == "" && clock.elapsed == nil {
		return domain.SlaReportClock{Elapsed: "—"}
	}
	elapsed := "—"
	if clock.elapsed != nil {
		elapsed = formatReportDuration(*clock.elapsed)
	}
	return domain.SlaReportClock{Elapsed: elapsed, Status: clock.status}
}

func slaCaseKind(issueType string) string {
	if strings.EqualFold(strings.TrimSpace(issueType), "QUESTION") {
		return slaCaseKindQuery
	}
	return slaCaseKindIncident
}

func slaCaseTypeLabel(issueType string) string {
	if slaCaseKind(issueType) == slaCaseKindQuery {
		return "Query"
	}
	return "Incident"
}

func slaSeverityRank(severity string) int {
	for i, item := range slaSeverities {
		if strings.EqualFold(item.code, severity) {
			return i
		}
	}
	return len(slaSeverities)
}

func slaReportSeverityLabel(severity string) string {
	severity = strings.TrimSpace(severity)
	if severity == "" {
		return "—"
	}
	for _, item := range slaSeverities {
		if strings.EqualFold(item.code, severity) {
			return item.label
		}
	}
	return severity
}

func slaReportStateLabel(state string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return "—"
	}
	words := strings.Split(strings.ToLower(state), "_")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

// percentileCont matches Postgres percentile_cont: linear interpolation
// between the two nearest ranks. sorted must be ascending.
func percentileCont(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	weight := rank - float64(lo)
	return sorted[lo]*(1-weight) + sorted[hi]*weight
}

// formatReportDuration renders seconds as the report shows them: "2 Days",
// "1 Hour", "30 Minutes", or the parts joined ("1 Day 4 Hours 30 Minutes").
func formatReportDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int64(seconds + 0.5)
	if total == 0 {
		return "0 Minutes"
	}
	if total%86400 == 0 {
		return durationUnit(total/86400, "Day")
	}
	if total%3600 == 0 {
		return durationUnit(total/3600, "Hour")
	}
	if total%60 == 0 && total < 3600 {
		return durationUnit(total/60, "Minute")
	}
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, durationUnit(days, "Day"))
	}
	if hours > 0 {
		parts = append(parts, durationUnit(hours, "Hour"))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, durationUnit(minutes, "Minute"))
	}
	return strings.Join(parts, " ")
}

func durationUnit(n int64, label string) string {
	if n == 1 {
		return "1 " + label
	}
	return strconv.FormatInt(n, 10) + " " + label + "s"
}
