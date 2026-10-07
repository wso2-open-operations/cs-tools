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
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// maxScheduleWindowDays bounds a single rota read. The month roster is the
// widest view the UI has, so a little over two months leaves room for a month
// either side of a boundary without letting a client ask for a decade.
const maxScheduleWindowDays = 70

// ScheduleService serves the Team Schedule. It validates the window, applies
// the reading rules the schema deliberately does not encode, and leaves the
// plain data operations to the repository.
type ScheduleService interface {
	Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error)
	SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) (domain.ScheduleAssignmentsResponse, error)
	SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) (domain.ScheduleAbsencesResponse, error)
	OnDuty(ctx context.Context, at *time.Time) (domain.ScheduleAssignmentsResponse, error)

	// The lead edit path. Each checks that the caller leads the team the slot
	// belongs to before it touches anything.
	CreateAssignment(ctx context.Context, req domain.CreateScheduleAssignmentRequest) (domain.ScheduleAssignment, error)
	UpdateAssignment(ctx context.Context, id string, req domain.UpdateScheduleAssignmentRequest) (domain.ScheduleAssignment, error)
	DeleteAssignment(ctx context.Context, id string, note *string) error
	TeamActivity(ctx context.Context, teamKey, from, to string) ([]domain.ScheduleAssignmentActivity, error)

	// MyLeadTeams is which teams this caller may edit.
	MyLeadTeams(ctx context.Context) ([]string, error)

	// ApplyRange is how the roster's picker edits: one engineer, one window,
	// across a span of days.
	ApplyRange(ctx context.Context, req domain.ApplyScheduleRangeRequest) (domain.ApplyScheduleRangeResponse, error)
	// EditMarkers is which cells in a window somebody has changed by hand.
	EditMarkers(ctx context.Context, from, to string) (domain.ScheduleEditMarkersResponse, error)

	// ApplyAbsence is the same picker marking somebody away, or bringing them
	// back, across a span.
	ApplyAbsence(ctx context.Context, req domain.ApplyScheduleAbsenceRequest) (domain.ApplyScheduleAbsenceResponse, error)
	// DeleteAbsence removes one absence -- the whole span, open-ended or not
	// -- which is what "remove this leave" means to the lead clicking it.
	DeleteAbsence(ctx context.Context, id string, note *string) error
	// CreateAbsenceKind adds a leave or allocation kind to the shared
	// catalogue. Any team lead may; every team then sees it.
	CreateAbsenceKind(ctx context.Context, req domain.CreateScheduleAbsenceKindRequest) (domain.ScheduleAbsenceKind, error)
	// DeleteAbsenceKind removes a kind a lead added, once nothing uses it.
	DeleteAbsenceKind(ctx context.Context, code string) error
}

type scheduleService struct {
	repo   repository.ScheduleRepository
	access AccessService
}

// NewScheduleService constructs a ScheduleService over the given repository.
func NewScheduleService(repo repository.ScheduleRepository, access AccessService) ScheduleService {
	return &scheduleService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted.
// The rota is staff data -- who is working, who is on leave and where -- and
// belongs to no project, so there is no narrower scope a customer could be
// given: an internal caller sees all of it and anyone else sees none. Mirrors
// sla_status_service.go's helper of the same name.
func (s *scheduleService) requireInternalCaller(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff")
}

func (s *scheduleService) Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleCatalogue{}, err
	}
	return s.repo.Catalogue(ctx)
}

// uuidPattern is the canonical 8-4-4-4-12 form user ids are issued in.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateUserID refuses a userId that is not a UUID before it reaches a
// uuid column, where Postgres would reject it as a 500 rather than the 400 a
// malformed filter deserves. Empty means "no filter" and passes.
func validateUserID(id string) error {
	if id != "" && !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("userId %q is not a UUID", id)}
	}
	return nil
}

// parseWindow validates a from/to pair and returns it normalised. Both dates
// are required: an unbounded rota read would happily return every row in the
// table, which is nobody's intent and a slow way to find that out.
func parseWindow(from, to string) (time.Time, time.Time, error) {
	if from == "" || to == "" {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "both from and to are required (YYYY-MM-DD)"}
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("from %q is not a YYYY-MM-DD date", from)}
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("to %q is not a YYYY-MM-DD date", to)}
	}
	if t.Before(f) {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "to is before from"}
	}
	if t.Sub(f) > maxScheduleWindowDays*24*time.Hour {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("window is longer than %d days", maxScheduleWindowDays)}
	}
	return f, t, nil
}

func (s *scheduleService) SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) (domain.ScheduleAssignmentsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if err := validateUserID(req.UserID); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if req.Family != "" && req.Family != "CRE" && req.Family != "SRE" && req.Family != "SME" {
		return domain.ScheduleAssignmentsResponse{}, &apierror.ValidationError{Msg: "family must be CRE, SRE or SME"}
	}
	rows, err := s.repo.SearchAssignments(ctx, req)
	if err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	return domain.ScheduleAssignmentsResponse{Assignments: rows, Count: len(rows)}, nil
}

func (s *scheduleService) SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) (domain.ScheduleAbsencesResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	if err := validateUserID(req.UserID); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	rows, err := s.repo.SearchAbsences(ctx, req)
	if err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	return domain.ScheduleAbsencesResponse{Absences: rows, Count: len(rows)}, nil
}

// OnDuty answers who is responsible at an instant, defaulting to now. This is
// the lookup an alert escalation needs before it decides who to ring.
func (s *scheduleService) OnDuty(ctx context.Context, at *time.Time) (domain.ScheduleAssignmentsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	moment := time.Now()
	if at != nil {
		moment = *at
	}
	rows, err := s.repo.OnDutyAt(ctx, moment)
	if err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	return domain.ScheduleAssignmentsResponse{Assignments: rows, Count: len(rows)}, nil
}

// ── lead edit ───────────────────────────────────────────────────────────────

// requireRotaWriter is the whole of the edit permission: internal, and either
// a lead of the team the slot belongs to, or a rota admin for the family that
// team belongs to.
//
// Own team only used to be the whole rule, and the reasoning stands: the blast
// radius of one team silently rewriting another's cover is worse than the
// inconvenience of not being able to. What it did not account for is that a
// rota then cannot be fixed at all while its lead is away -- not by a manager,
// not by the head of CRE, not by anyone. The answer is a named grant somebody
// has to be given and can be taken back, rather than widening what a lead
// already holds: cre_rota_admin and sre_rota_admin (migration 0156), and
// sme_rota_admin (0200), one per family, because the rotas are run by
// different people and no group has any business in another's cover.
//
// Leading is checked first because it is the common case by a wide margin --
// eleven leads against a handful of admins -- so the second read usually never
// happens.
//
// The internal check comes first so a customer never learns whether a team key
// exists by being told they do not lead it.
func (s *scheduleService) requireRotaWriter(ctx context.Context, teamKey string) error {
	if err := s.requireInternalCaller(ctx); err != nil {
		return err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "editing the rota needs a user token, not a service credential"}
	}
	ok, err := s.repo.LeadsTeam(ctx, id.UserEmail, teamKey)
	if err != nil {
		return err
	}
	if !ok {
		admin, err := s.repo.RotaAdminTeamsFor(ctx, id.UserEmail)
		if err != nil {
			return err
		}
		ok = containsFold(admin, teamKey)
	}
	if !ok {
		return &apierror.ForbiddenError{
			Msg: fmt.Sprintf("changing %s's rota needs a lead of that team, or a rota admin for its family", teamKey),
		}
	}
	return nil
}

// containsFold is a case-insensitive membership test over team keys.
//
// team.key is stored lowercase and both repository reads return it as stored,
// but a team key also arrives from a caller's request body, where nothing
// makes it lowercase -- LeadsTeam lowercases its own argument in SQL for
// exactly that reason, and this comparison has to agree with it or an admin
// would be refused a team they hold purely over capitalisation.
func containsFold(keys []string, want string) bool {
	for _, k := range keys {
		if strings.EqualFold(k, want) {
			return true
		}
	}
	return false
}

// requireRotaWriterOver is the team check plus the engineer being changed.
//
// Being allowed to edit a team says what the caller may change; it does not
// say whose rota they may change it on. Without this second check a lead could
// name their own team -- which they genuinely lead, so the first check passes
// -- and pass the id of somebody on another team entirely, writing a row
// against a person they have no say over. A team key in a request decides
// nothing on its own.
//
// This applies to a rota admin unchanged. It is not a limit on their
// authority -- they may edit every team in their family, and can simply name
// the right one -- it is the rule that a row belongs to the team it is filed
// under. ApplyRange's own delete is scoped by team_key on the strength of it.
func (s *scheduleService) requireRotaWriterOver(ctx context.Context, teamKey, userID string) error {
	if err := s.requireRotaWriter(ctx, teamKey); err != nil {
		return err
	}
	member, err := s.repo.UserInTeam(ctx, userID, teamKey)
	if err != nil {
		return err
	}
	if !member {
		return &apierror.ForbiddenError{
			Msg: fmt.Sprintf("that engineer is not on %s, so their rota is not yours to change", teamKey),
		}
	}
	return nil
}

// requireRotaWriterOverSpan is requireRotaWriterOver for a change over the
// dates [from, to], and returns the team the person belongs to.
//
// A member of teamKey is checked exactly as requireRotaWriterOver does. So is
// anyone with no claim on the team at all. The case between is somebody on a
// span that moved them to teamKey for the whole of [from, to] -- the Brazil
// rotation, which moves them to the Americas team for months: that team's
// lead (or a rota admin for it) may roster them while it lasts, and, when
// homeLeadMayManage is set, so may their own team's lead, who still owns the
// span itself -- ending it early, moving it, marking leave inside it.
func (s *scheduleService) requireRotaWriterOverSpan(ctx context.Context, teamKey, userID, from, to string, homeLeadMayManage bool) (string, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return "", err
	}
	member, err := s.repo.UserInTeam(ctx, userID, teamKey)
	if err != nil {
		return "", err
	}
	if member {
		return teamKey, s.requireRotaWriter(ctx, teamKey)
	}
	home, moved, err := s.repo.MovedToTeamOver(ctx, userID, teamKey, from, to)
	if err != nil {
		return "", err
	}
	if !moved {
		// Unchanged from requireRotaWriterOver, refusals and their order.
		if err := s.requireRotaWriter(ctx, teamKey); err != nil {
			return "", err
		}
		return "", &apierror.ForbiddenError{
			Msg: fmt.Sprintf("that engineer is not on %s, so their rota is not yours to change", teamKey),
		}
	}
	writerErr := s.requireRotaWriter(ctx, teamKey)
	if writerErr == nil {
		return home, nil
	}
	if homeLeadMayManage && home != "" && s.requireRotaWriter(ctx, home) == nil {
		return home, nil
	}
	return "", writerErr
}

// CreateAssignment implements ScheduleService.
func (s *scheduleService) CreateAssignment(ctx context.Context, req domain.CreateScheduleAssignmentRequest) (domain.ScheduleAssignment, error) {
	if err := validateUserID(req.UserID); err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if req.TeamKey == "" || req.ShiftCode == "" || req.RotaDate == "" {
		return domain.ScheduleAssignment{}, &apierror.ValidationError{Msg: "teamKey, shiftCode and rotaDate are required"}
	}
	if _, err := time.Parse("2006-01-02", req.RotaDate); err != nil {
		return domain.ScheduleAssignment{}, &apierror.ValidationError{Msg: "rotaDate must be YYYY-MM-DD"}
	}
	if err := s.requireRotaWriterOver(ctx, req.TeamKey, req.UserID); err != nil {
		return domain.ScheduleAssignment{}, err
	}
	return s.repo.CreateAssignment(ctx, req, auth.IdentityFromContext(ctx).UserEmail)
}

// UpdateAssignment implements ScheduleService.
//
// The team is read from the row rather than taken from the caller: otherwise a
// lead could name their own team and edit anyone's slot.
func (s *scheduleService) UpdateAssignment(ctx context.Context, id string, req domain.UpdateScheduleAssignmentRequest) (domain.ScheduleAssignment, error) {
	if req.UserID != nil {
		if err := validateUserID(*req.UserID); err != nil {
			return domain.ScheduleAssignment{}, err
		}
	}
	existing, err := s.assignmentForEdit(ctx, id)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}
	return s.repo.UpdateAssignment(ctx, existing.ID, req, auth.IdentityFromContext(ctx).UserEmail)
}

// DeleteAssignment implements ScheduleService.
func (s *scheduleService) DeleteAssignment(ctx context.Context, id string, note *string) error {
	existing, err := s.assignmentForEdit(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.DeleteAssignment(ctx, existing.ID, auth.IdentityFromContext(ctx).UserEmail, note)
}

// assignmentForEdit loads a slot and confirms the caller leads its team.
func (s *scheduleService) assignmentForEdit(ctx context.Context, id string) (domain.ScheduleAssignment, error) {
	if err := validateUserID(id); err != nil {
		return domain.ScheduleAssignment{}, &apierror.ValidationError{Msg: fmt.Sprintf("assignment id %q is not a UUID", id)}
	}
	// Internal first: an outside caller should not be able to probe which
	// assignment ids exist by the difference between 403 and 404.
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAssignment{}, err
	}
	existing, err := s.repo.AssignmentByID(ctx, id)
	if err != nil {
		return domain.ScheduleAssignment{}, err
	}
	if err := s.requireRotaWriter(ctx, existing.TeamKey); err != nil {
		return domain.ScheduleAssignment{}, err
	}
	return existing, nil
}

// TeamActivity implements ScheduleService.
func (s *scheduleService) TeamActivity(ctx context.Context, teamKey, from, to string) ([]domain.ScheduleAssignmentActivity, error) {
	if _, _, err := parseWindow(from, to); err != nil {
		return nil, err
	}
	if err := s.requireRotaWriter(ctx, teamKey); err != nil {
		return nil, err
	}
	return s.repo.ActivityForTeam(ctx, teamKey, from, to)
}

// MyLeadTeams implements ScheduleService.
//
// No team argument and nothing to authorize beyond being internal: the answer
// is about the caller, and an empty list is a perfectly good answer for
// somebody who leads nothing.
//
// The name is older than what it returns, and is kept: this has always been
// "which teams may I edit" (its own interface comment says so) rather than
// "which teams do I lead", and the frontend uses it only to decide which rows
// get an edit control. Widening it here is what makes a rota admin's extra
// reach appear in the UI without the page changing at all -- and keeping the
// two in step matters more than the name: a team offered an edit control that
// then 403s, or withheld from somebody who may in fact edit it, are both worse
// than a method whose name has aged.
func (s *scheduleService) MyLeadTeams(ctx context.Context) ([]string, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return nil, err
	}
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return []string{}, nil
	}
	led, err := s.repo.LeadTeamsFor(ctx, email)
	if err != nil {
		return nil, err
	}
	admin, err := s.repo.RotaAdminTeamsFor(ctx, email)
	if err != nil {
		return nil, err
	}
	return mergeTeamKeys(led, admin), nil
}

// mergeTeamKeys unions two team key lists, case-insensitively.
//
// A rota admin who also leads one of their own family's teams holds it by both
// routes and would otherwise be handed it twice. Sorted so the result does not
// depend on which list a key came from; both reads are already ordered, so
// this only settles the join between them.
func mergeTeamKeys(led, admin []string) []string {
	out := make([]string, 0, len(led)+len(admin))
	seen := make(map[string]bool, len(led)+len(admin))
	for _, list := range [][]string{led, admin} {
		for _, key := range list {
			folded := strings.ToLower(key)
			if seen[folded] {
				continue
			}
			seen[folded] = true
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

// ApplyRange implements ScheduleService.
func (s *scheduleService) ApplyRange(ctx context.Context, req domain.ApplyScheduleRangeRequest) (domain.ApplyScheduleRangeResponse, error) {
	if err := validateUserID(req.UserID); err != nil {
		return domain.ApplyScheduleRangeResponse{}, err
	}
	// shiftCode is the one optional field: empty means take them off the rota
	// over the span rather than put them on a window.
	if req.UserID == "" || req.TeamKey == "" || req.From == "" || req.To == "" {
		return domain.ApplyScheduleRangeResponse{}, &apierror.ValidationError{
			Msg: "userId, teamKey, from and to are all required",
		}
	}
	if req.ZoneCode != nil {
		z := strings.ToUpper(strings.TrimSpace(*req.ZoneCode))
		switch {
		case z == "":
			req.ZoneCode = nil
		case req.ShiftCode != "":
			return domain.ApplyScheduleRangeResponse{}, &apierror.ValidationError{Msg: "zoneCode only narrows a clear; leave shiftCode empty with it"}
		default:
			req.ZoneCode = &z
		}
	}
	if req.Tier != nil {
		t := strings.ToUpper(strings.TrimSpace(*req.Tier))
		switch {
		case t == "":
			req.Tier = nil
		case t != "L1" && t != "L2" && t != "L3":
			return domain.ApplyScheduleRangeResponse{}, &apierror.ValidationError{Msg: "tier must be L1, L2 or L3"}
		case req.ShiftCode == "":
			return domain.ApplyScheduleRangeResponse{}, &apierror.ValidationError{Msg: "a tier needs a shiftCode to hold it"}
		default:
			req.Tier = &t
		}
	}
	if _, err := s.requireRotaWriterOverSpan(ctx, req.TeamKey, req.UserID, req.From, req.To, false); err != nil {
		return domain.ApplyScheduleRangeResponse{}, err
	}
	return s.repo.ApplyRange(ctx, req, auth.IdentityFromContext(ctx).UserEmail)
}

// maxAllocatedToLength is team_schedule_absence.allocated_to's width.
const maxAllocatedToLength = 100

// ApplyAbsence implements ScheduleService.
func (s *scheduleService) ApplyAbsence(ctx context.Context, req domain.ApplyScheduleAbsenceRequest) (domain.ApplyScheduleAbsenceResponse, error) {
	if err := validateUserID(req.UserID); err != nil {
		return domain.ApplyScheduleAbsenceResponse{}, err
	}
	// kindCode is the one optional field: empty means bring them back over the
	// span rather than mark them away across it.
	if req.UserID == "" || req.TeamKey == "" || req.From == "" || req.To == "" {
		return domain.ApplyScheduleAbsenceResponse{}, &apierror.ValidationError{
			Msg: "userId, teamKey, from and to are all required",
		}
	}
	if req.AllocatedTo != nil {
		v := strings.TrimSpace(*req.AllocatedTo)
		switch {
		case v == "":
			req.AllocatedTo = nil
		case utf8.RuneCountInString(v) > maxAllocatedToLength:
			return domain.ApplyScheduleAbsenceResponse{}, &apierror.ValidationError{
				Msg: fmt.Sprintf("allocatedTo is at most %d characters", maxAllocatedToLength),
			}
		default:
			req.AllocatedTo = &v
		}
	}
	home, err := s.requireRotaWriterOverSpan(ctx, req.TeamKey, req.UserID, req.From, req.To, true)
	if err != nil {
		return domain.ApplyScheduleAbsenceResponse{}, err
	}
	req.HomeTeamKey = home
	return s.repo.ApplyAbsence(ctx, req, auth.IdentityFromContext(ctx).UserEmail)
}

// DeleteAbsenceKind implements ScheduleService.
//
// Any team lead may, as any team lead may add one: the tags are shared, and
// the repository refuses a built-in tag or one still in use.
func (s *scheduleService) DeleteAbsenceKind(ctx context.Context, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return &apierror.ValidationError{Msg: "a tag code is required"}
	}
	email, err := s.requireAnyTeamLead(ctx, "deleting a tag")
	if err != nil {
		return err
	}
	return s.repo.DeleteAbsenceKind(ctx, code, email)
}

// requireAnyTeamLead is the gate for the shared tag catalogue: an internal
// caller with a user token who may edit at least one rota -- a lead of a team,
// or a rota admin for a family. It returns the caller's email for the history.
func (s *scheduleService) requireAnyTeamLead(ctx context.Context, doing string) (string, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return "", err
	}
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return "", &apierror.ForbiddenError{Msg: doing + " needs a user token, not a service credential"}
	}
	teams, err := s.repo.LeadTeamsFor(ctx, email)
	if err != nil {
		return "", err
	}
	if len(teams) == 0 {
		admin, err := s.repo.RotaAdminTeamsFor(ctx, email)
		if err != nil {
			return "", err
		}
		teams = admin
	}
	if len(teams) == 0 {
		return "", &apierror.ForbiddenError{Msg: "only a team lead or a rota admin can change the tags"}
	}
	return email, nil
}

// DeleteAbsence implements ScheduleService.
//
// The team and engineer are read from the row rather than taken from the
// caller, for the same reason UpdateAssignment does: otherwise a lead could
// name their own team and remove anybody's leave.
func (s *scheduleService) DeleteAbsence(ctx context.Context, id string, note *string) error {
	if err := validateUserID(id); err != nil {
		return &apierror.ValidationError{Msg: fmt.Sprintf("absence id %q is not a UUID", id)}
	}
	// Internal first, so an outside caller cannot probe which ids exist by
	// the difference between 403 and 404.
	if err := s.requireInternalCaller(ctx); err != nil {
		return err
	}
	existing, err := s.repo.AbsenceByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.requireRotaWriterOver(ctx, existing.TeamKey, existing.Engineer.UserID); err != nil {
		return err
	}
	return s.repo.DeleteAbsence(ctx, existing.ID, auth.IdentityFromContext(ctx).UserEmail, note)
}

// absenceKindColours are the colour tokens a new kind may use: the ones the
// portal's stylesheet draws a leave or allocation chip for. Anything else
// would render as an uncoloured chip nobody could tell apart.
var absenceKindColours = map[string]bool{
	"AL": true, "LL": true, "MAT": true, "PAT": true, "RND": true, "EXT": true,
	"INT": true, "BR": true, "MIG": true, "ONB": true, "EXC": true, "IND": true,
}

var nonCodeChars = regexp.MustCompile(`[^A-Z0-9]+`)

// absenceKindCode derives a kind's code from its label: "Customer on-call" is
// CUSTOMER_ON_CALL. Derived rather than asked for, so it is always a valid
// code and the same label cannot be added twice under two spellings.
func absenceKindCode(label string) string {
	c := strings.Trim(nonCodeChars.ReplaceAllString(strings.ToUpper(label), "_"), "_")
	if len(c) > 32 {
		c = strings.TrimRight(c[:32], "_")
	}
	return c
}

// CreateAbsenceKind implements ScheduleService.
func (s *scheduleService) CreateAbsenceKind(ctx context.Context, req domain.CreateScheduleAbsenceKindRequest) (domain.ScheduleAbsenceKind, error) {
	req.ShortCode = strings.TrimSpace(req.ShortCode)
	req.Label = strings.TrimSpace(req.Label)
	req.Bucket = strings.ToUpper(strings.TrimSpace(req.Bucket))
	req.ColourToken = strings.ToUpper(strings.TrimSpace(req.ColourToken))
	switch {
	case req.ShortCode == "" || utf8.RuneCountInString(req.ShortCode) > 12:
		return domain.ScheduleAbsenceKind{}, &apierror.ValidationError{Msg: "shortCode is required and at most 12 characters"}
	case req.Label == "" || utf8.RuneCountInString(req.Label) > 100:
		return domain.ScheduleAbsenceKind{}, &apierror.ValidationError{Msg: "label is required and at most 100 characters"}
	// EXCLUDED -- off the rota entirely -- is not a lead's to invent more of.
	case req.Bucket != "LEAVE" && req.Bucket != "ALLOCATION":
		return domain.ScheduleAbsenceKind{}, &apierror.ValidationError{Msg: "bucket must be LEAVE or ALLOCATION"}
	case !absenceKindColours[req.ColourToken]:
		return domain.ScheduleAbsenceKind{}, &apierror.ValidationError{Msg: fmt.Sprintf("colourToken %q is not one the rota can draw", req.ColourToken)}
	}
	code := absenceKindCode(req.Label)
	if code == "" {
		return domain.ScheduleAbsenceKind{}, &apierror.ValidationError{Msg: "label needs at least one letter or digit"}
	}

	email, err := s.requireAnyTeamLead(ctx, "adding a tag")
	if err != nil {
		return domain.ScheduleAbsenceKind{}, err
	}
	return s.repo.CreateAbsenceKind(ctx, code, req, email)
}

// EditMarkers implements ScheduleService.
func (s *scheduleService) EditMarkers(ctx context.Context, from, to string) (domain.ScheduleEditMarkersResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleEditMarkersResponse{}, err
	}
	if _, _, err := parseWindow(from, to); err != nil {
		return domain.ScheduleEditMarkersResponse{}, err
	}
	rows, err := s.repo.EditMarkers(ctx, from, to)
	if err != nil {
		return domain.ScheduleEditMarkersResponse{}, err
	}
	return domain.ScheduleEditMarkersResponse{Markers: rows, Count: len(rows)}, nil
}
