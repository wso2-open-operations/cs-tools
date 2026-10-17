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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// CaseAlertSettingsService is the settings section of the Team Schedule's Case
// Alerts tab: the chat rooms, ServiceNow group routes and other ladder values
// the paging ladders use (migration 0233), so they can be changed without a
// Choreo secret or the file mount. csm-notification-service reads them
// (Runtime) only when it runs with PAGING_CONFIG_SOURCE set, and only at
// start-up.
//
// Who may do what:
//
//	see the settings     a CSM admin (AUTH_ADMIN_ROLES), a portal admin, the
//	                     CRE or SRE rota admin, the CRE or CS head
//	change a ladder's    a CSM admin or portal admin: both ladders; the CRE
//	rooms, routes and    rota admin: CRE; the SRE rota admin: SRE
//	values
//	read the opened URLs csm-notification-service's client only
//	                     (CASE_ALERTS_RUNTIME_CLIENT_IDS), never a person
type CaseAlertSettingsService interface {
	Get(ctx context.Context) (domain.CaseAlertSettings, error)
	SaveRoom(ctx context.Context, req domain.SaveCaseAlertRoomRequest) (domain.CaseAlertChatRoom, error)
	SetWebhook(ctx context.Context, roomID string, req domain.SetCaseAlertWebhookRequest) (domain.CaseAlertChatRoom, error)
	DeleteRoom(ctx context.Context, roomID string) error
	SendTestCard(ctx context.Context, roomID string) (domain.CaseAlertTestCardResponse, error)
	SaveRoute(ctx context.Context, req domain.SaveCaseAlertRouteRequest) (domain.CaseAlertGroupRoute, error)
	DeleteRoute(ctx context.Context, routeID string) error
	SaveValues(ctx context.Context, req domain.SaveCaseAlertValuesRequest) (domain.CaseAlertValuesResponse, error)
	DeleteValue(ctx context.Context, valueID string) error

	Runtime(ctx context.Context) (domain.CaseAlertRuntimeSettings, error)
	RecordApplied(ctx context.Context, report domain.CaseAlertAppliedReport) error
}

// CaseAlertSettingsConfig is the deployment's settings for this service. None
// is needed to save and use chat rooms: URLs are stored as saved (0237).
type CaseAlertSettingsConfig struct {
	AdminRoles []string // AUTH_ADMIN_ROLES
	// SecretKey and SecretKeyID only open a URL saved sealed before 0237.
	SecretKey   string // CASE_ALERTS_SECRET_KEY
	SecretKeyID string // CASE_ALERTS_SECRET_KEY_ID
	// RuntimeClientIDs narrows who may read the runtime settings; empty, any
	// internal service client may (csm-notification-service's among them).
	RuntimeClientIDs []string // CASE_ALERTS_RUNTIME_CLIENT_IDS
	Environment      string   // CASE_ALERTS_ENVIRONMENT
}

type caseAlertSettingsService struct {
	repo     repository.CaseAlertSettingsRepository
	people   repository.PagingChainRepository
	access   AccessService
	cfg      CaseAlertSettingsConfig
	sealer   *caseAlertSealer
	poster   chatPoster
	runtimes map[string]bool
	now      func() time.Time
}

// NewCaseAlertSettingsService constructs the service. A CASE_ALERTS_SECRET_KEY
// that cannot be used is logged; it only means a URL sealed before 0237 cannot
// be opened, and never stops the service.
func NewCaseAlertSettingsService(repo repository.CaseAlertSettingsRepository, people repository.PagingChainRepository, access AccessService, cfg CaseAlertSettingsConfig) CaseAlertSettingsService {
	sealer, err := newCaseAlertSealer(cfg.SecretKey, cfg.SecretKeyID)
	if err != nil {
		slog.Error("case alerts: CASE_ALERTS_SECRET_KEY cannot be used; a URL sealed before 0237 cannot be opened", "error", err)
		sealer = nil
	}
	runtimes := map[string]bool{}
	for _, id := range cfg.RuntimeClientIDs {
		if id = strings.TrimSpace(id); id != "" {
			runtimes[id] = true
		}
	}
	return &caseAlertSettingsService{
		repo: repo, people: people, access: access, cfg: cfg,
		sealer: sealer, poster: newHTTPChatPoster(), runtimes: runtimes, now: time.Now,
	}
}

const (
	caseAlertTestCooldown  = 2 * time.Minute
	caseAlertTestDelivered = "delivered"
	caseAlertTestFailed    = "failed"
	maxCaseAlertLabel      = 100
	maxCaseAlertGroupName  = 255
	maxCaseAlertSummary    = 32 << 10
)

var caseAlertLadders = []string{familyCRE, familySRE}

// caseAlertRoomLadders are the ladders with chat rooms here: CRE's and SRE's,
// and SME's team rooms (0237). SME has no routes or ladder values here.
var caseAlertRoomLadders = []string{familyCRE, familySRE, familySME}

var caseAlertModes = map[string]bool{"off": true, "shadow": true, "db": true}

// caseAlertChannels are how a team's alerts can go out, as the paging file
// spells a ladder's channel.
var caseAlertChannels = map[string]bool{"log": true, "chat": true, "call": true, "both": true}

// caseAlertCaller is a person and what they may do with the settings.
type caseAlertCaller struct {
	email  string
	view   bool
	ladder map[string]bool // ladders whose rooms and routes they may change
}

func (c caseAlertCaller) ladders() []string {
	return c.of(caseAlertLadders)
}

// roomLadders are the ladders whose rooms the caller may change.
func (c caseAlertCaller) roomLadders() []string {
	return c.of(caseAlertRoomLadders)
}

func (c caseAlertCaller) of(ladders []string) []string {
	out := []string{}
	for _, l := range ladders {
		if c.ladder[l] {
			out = append(out, l)
		}
	}
	return out
}

// whoIsAsking resolves a person's rights. Internal first, so somebody outside
// learns nothing; then a user token, since every change is a person's.
func (s *caseAlertSettingsService) whoIsAsking(ctx context.Context) (caseAlertCaller, error) {
	if err := RequireInternalCaller(ctx, s.access, "the Case Alerts settings are only available to internal staff"); err != nil {
		return caseAlertCaller{}, err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return caseAlertCaller{}, &apierror.ForbiddenError{Msg: "the Case Alerts settings need a user token, not a service credential"}
	}
	c := caseAlertCaller{email: id.UserEmail, ladder: map[string]bool{}}
	if holdsAnyRole(id.Roles, s.cfg.AdminRoles) {
		c.ladder[familyCRE], c.ladder[familySRE], c.ladder[familySME] = true, true, true
		c.view = true
		return c, nil
	}
	who, err := s.people.CallerAuthority(ctx, id.UserEmail)
	if err != nil {
		return caseAlertCaller{}, err
	}
	a := authorityOf(who)
	// Each family's settings are its own: the CRE rota admin and the CRE and
	// CS heads have CRE's, the SRE rota admin SRE's, the SME rota admin SME's.
	// Seeing a family's settings and changing them go together; another
	// family's are never sent. A CSM or portal admin has all three.
	switch {
	case a.portalAdmin:
		c.ladder[familyCRE], c.ladder[familySRE], c.ladder[familySME] = true, true, true
	default:
		c.ladder[familyCRE] = a.creRotaAdmin || a.head
		c.ladder[familySRE] = a.sreRotaAdmin
		c.ladder[familySME] = a.smeRotaAdmin
	}
	c.view = len(c.roomLadders()) > 0
	return c, nil
}

// mayChange refuses a change to ladder's settings the caller may not make.
func (c caseAlertCaller) mayChange(ladder string) error {
	if !c.ladder[ladder] {
		return &apierror.ForbiddenError{Msg: fmt.Sprintf("only a CSM admin, a portal admin or the %s rota admin can change the %s alert settings", ladder, ladder)}
	}
	return nil
}

// normRoomLadder is normLadder for a chat room, which SME has too.
func normRoomLadder(v string) (string, error) {
	if l := strings.ToUpper(strings.TrimSpace(v)); l == familySME {
		return l, nil
	}
	l, err := normLadder(v)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "ladder must be CRE, SRE or SME"}
	}
	return l, nil
}

// normValueLadder is normLadder for a ladder value, which SME has too (0238).
func normValueLadder(v string) (string, error) {
	if l := strings.ToUpper(strings.TrimSpace(v)); l == familySME {
		return l, nil
	}
	l, err := normLadder(v)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "ladder must be CRE, SRE or SME"}
	}
	return l, nil
}

func normLadder(v string) (string, error) {
	l := strings.ToUpper(strings.TrimSpace(v))
	if l != familyCRE && l != familySRE {
		return "", &apierror.ValidationError{Msg: "ladder must be CRE or SRE"}
	}
	return l, nil
}

// Get implements CaseAlertSettingsService. A caller who may see nothing gets
// an empty answer rather than a refusal: the tab asks this to decide whether
// to show the section at all.
func (s *caseAlertSettingsService) Get(ctx context.Context) (domain.CaseAlertSettings, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertSettings{}, err
	}
	out := domain.CaseAlertSettings{
		CanManage: domain.CaseAlertSettingsRights{View: c.view, RoomLadders: c.roomLadders(), RouteLadders: c.ladders()},
		Rooms:     []domain.CaseAlertChatRoom{},
		Routes:    []domain.CaseAlertGroupRoute{},
		Values:    []domain.CaseAlertLadderValue{},
	}
	if !c.view {
		out.CanManage.RoomLadders, out.CanManage.RouteLadders = []string{}, []string{}
		return out, nil
	}
	out.Environment = s.cfg.Environment
	out.WebhooksEnabled = true
	rev, err := s.repo.Revision(ctx)
	if err != nil {
		return domain.CaseAlertSettings{}, err
	}
	out.Revision = revisionView(rev)
	rooms, err := s.repo.ListRooms(ctx)
	if err != nil {
		return domain.CaseAlertSettings{}, err
	}
	for _, r := range rooms {
		if c.ladder[r.Ladder] {
			out.Rooms = append(out.Rooms, roomView(r))
		}
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return domain.CaseAlertSettings{}, err
	}
	for _, r := range routes {
		if c.ladder[r.Ladder] {
			out.Routes = append(out.Routes, routeView(r))
		}
	}
	values, err := s.repo.ListValues(ctx)
	if err != nil {
		return domain.CaseAlertSettings{}, err
	}
	for _, v := range values {
		if c.ladder[v.Ladder] {
			out.Values = append(out.Values, valueView(v))
		}
	}
	out.Running = runningView(ctx, rev, c.ladder)
	if out.RouteOptions, err = s.routeOptions(ctx); err != nil {
		return domain.CaseAlertSettings{}, err
	}
	return out, nil
}

// routeOptions are the lists the tab offers for a group route.
func (s *caseAlertSettingsService) routeOptions(ctx context.Context) (*domain.CaseAlertRouteOptions, error) {
	groups, err := s.repo.RouteGroups(ctx)
	if err != nil {
		return nil, err
	}
	teams, err := s.repo.RouteTeams(ctx)
	if err != nil {
		return nil, err
	}
	rotas, err := s.repo.SRERotas(ctx)
	if err != nil {
		return nil, err
	}
	out := &domain.CaseAlertRouteOptions{
		Groups: []domain.CaseAlertTeamOption{}, Teams: []domain.CaseAlertTeamOption{}, Rotas: []domain.CaseAlertRotaOption{},
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, domain.CaseAlertTeamOption{Key: g.Key, Name: g.Name, Family: g.Family})
	}
	for _, t := range teams {
		out.Teams = append(out.Teams, domain.CaseAlertTeamOption{Key: t.Key, Name: t.Name, Family: t.Family})
	}
	for _, r := range rotas {
		out.Rotas = append(out.Rotas, domain.CaseAlertRotaOption{Code: r.Code, Label: r.Label})
	}
	return out, nil
}

// routeGroupFamilies are the families of the CSM Portal teams a ladder's
// routes can be for: a CRE case's team, or an SRE incident's SRE or SME team.
var routeGroupFamilies = map[string][]string{familyCRE: {familyCRE}, familySRE: {familySRE, familySME}}

func valueView(r repository.CaseAlertValueRow) domain.CaseAlertLadderValue {
	return domain.CaseAlertLadderValue{
		ID: r.ID, Ladder: r.Ladder, Key: r.Key, Value: json.RawMessage(r.Value),
		UpdatedBy: r.UpdatedBy, UpdatedOn: r.UpdatedOn,
	}
}

// runningView is what the alert service reported it runs with, from the
// summary of its last report; nil when that start was in off mode, or carried
// no such report.
// runningView is what the alert service last reported it runs with, for the
// ladders the caller may see (mine) only.
func runningView(ctx context.Context, rev repository.CaseAlertRevisionRow, mine map[string]bool) *domain.CaseAlertRunning {
	if len(rev.AppliedSummary) == 0 || rev.AppliedMode == nil || *rev.AppliedMode == "off" {
		return nil
	}
	var summary struct {
		Running *struct {
			Ladders  map[string]map[string]domain.CaseAlertRunningValue `json:"ladders"`
			FileOnly map[string]map[string]json.RawMessage              `json:"fileOnly"`
		} `json:"running"`
	}
	if err := json.Unmarshal(rev.AppliedSummary, &summary); err != nil {
		slog.WarnContext(ctx, "case alerts: the alert service's last report could not be read", "error", err)
		return nil
	}
	if summary.Running == nil || len(summary.Running.Ladders) == 0 {
		return nil
	}
	out := &domain.CaseAlertRunning{
		ReportedAt: rev.AppliedAt, Mode: *rev.AppliedMode,
		Ladders:  map[string]map[string]domain.CaseAlertRunningValue{},
		FileOnly: map[string]map[string]json.RawMessage{},
	}
	for l, v := range summary.Running.Ladders {
		if mine[l] {
			out.Ladders[l] = v
		}
	}
	for l, v := range summary.Running.FileOnly {
		if mine[l] {
			out.FileOnly[l] = v
		}
	}
	return out
}

func revisionView(r repository.CaseAlertRevisionRow) domain.CaseAlertRevision {
	v := domain.CaseAlertRevision{Saved: r.Revision, ChangedAt: r.ChangedAt, Applied: r.AppliedRevision, AppliedAt: r.AppliedAt}
	if r.AppliedMode != nil {
		v.AppliedMode = *r.AppliedMode
	}
	return v
}

func roomView(r repository.CaseAlertRoomRow) domain.CaseAlertChatRoom {
	v := domain.CaseAlertChatRoom{
		ID: r.ID, Ladder: r.Ladder, TeamKey: r.TeamKey, Label: r.Label, Enabled: r.Enabled, Teamless: r.Teamless,
		Channel: r.Channel, HasWebhook: r.HasSecret, WebhookSetAt: r.WebhookSetAt, LastTestAt: r.LastTestAt,
		UpdatedBy: r.UpdatedBy, UpdatedOn: r.UpdatedOn,
	}
	if r.HasSecret && r.WebhookHint != nil {
		v.WebhookHint = *r.WebhookHint
	}
	if r.WebhookSetBy != nil {
		v.WebhookSetBy = *r.WebhookSetBy
	}
	if r.LastTestStatus != nil {
		v.LastTestStatus = *r.LastTestStatus
	}
	return v
}

func routeView(r repository.CaseAlertRouteRow) domain.CaseAlertGroupRoute {
	return domain.CaseAlertGroupRoute{
		ID: r.ID, Ladder: r.Ladder, GroupName: r.GroupName, TeamKey: r.TeamKey, RotaCode: r.RotaCode,
		Enabled: r.Enabled, UpdatedBy: r.UpdatedBy, UpdatedOn: r.UpdatedOn,
	}
}

// SaveRoom implements CaseAlertSettingsService.
func (s *caseAlertSettingsService) SaveRoom(ctx context.Context, req domain.SaveCaseAlertRoomRequest) (domain.CaseAlertChatRoom, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	ladder, err := normRoomLadder(req.Ladder)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	if err := c.mayChange(ladder); err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	label := strings.TrimSpace(req.Label)
	if label == "" || len(label) > maxCaseAlertLabel {
		return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: fmt.Sprintf("label must be 1 to %d characters", maxCaseAlertLabel)}
	}
	w := repository.CaseAlertRoomWrite{Ladder: ladder, TeamKey: strings.ToLower(strings.TrimSpace(req.TeamKey)), Label: label, Enabled: true}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}
	if req.Teamless != nil {
		w.Teamless = *req.Teamless
	}
	if req.Channel != nil {
		w.Channel = strings.ToLower(strings.TrimSpace(*req.Channel))
		if w.Channel != "" && !caseAlertChannels[w.Channel] {
			return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "channel must be chat, call, both or log; leave it empty for the ladder's"}
		}
	}
	if ladder == familySME {
		// An SME room is a team's room and nothing more: no shared room,
		// teamless or channel (0237).
		if w.TeamKey == "" {
			return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "an SME room is an SME team's: send its teamKey"}
		}
		if w.Teamless || w.Channel != "" {
			return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "teamless and channel are for SRE team rooms; an SME room has only its URL"}
		}
	}
	if w.TeamKey != "" {
		if ladder == familyCRE {
			return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "only SRE and SME teams have rooms of their own; CRE has one shared room"}
		}
		family, ok, err := s.repo.RosteredTeamFamily(ctx, w.TeamKey)
		if err != nil {
			return domain.CaseAlertChatRoom{}, err
		}
		if !ok || family != ladder {
			return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: fmt.Sprintf("teamKey must be an %s team of the Team Schedule", ladder)}
		}
	} else if w.Teamless {
		return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "teamless is for a team's room; the shared room is not one"}
	} else if w.Channel != "" {
		return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: "a channel is for a team's row; the ladder's own channel is set in the alert service's file"}
	}
	row, err := s.repo.SaveRoom(ctx, c.email, w)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	return roomView(row), nil
}

// room is a room the caller may change, by id.
func (s *caseAlertSettingsService) room(ctx context.Context, c caseAlertCaller, roomID string) (repository.CaseAlertRoomRow, error) {
	if err := validateUUIDs("roomId", []string{roomID}); err != nil {
		return repository.CaseAlertRoomRow{}, err
	}
	r, err := s.repo.GetRoom(ctx, roomID)
	if err != nil {
		return repository.CaseAlertRoomRow{}, err
	}
	if r == nil {
		return repository.CaseAlertRoomRow{}, &apierror.NotFoundError{Msg: "no such chat room"}
	}
	if err := c.mayChange(r.Ladder); err != nil {
		return repository.CaseAlertRoomRow{}, err
	}
	return *r, nil
}

// SetWebhook implements CaseAlertSettingsService.
func (s *caseAlertSettingsService) SetWebhook(ctx context.Context, roomID string, req domain.SetCaseAlertWebhookRequest) (domain.CaseAlertChatRoom, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	r, err := s.room(ctx, c, roomID)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	webhook, hint, err := parseChatWebhook(req.URL)
	if err != nil {
		return domain.CaseAlertChatRoom{}, &apierror.ValidationError{Msg: err.Error()}
	}
	// Stored as saved, in its own table that is never audited or returned
	// to a browser; a sealed one saved before 0237 is replaced.
	found, err := s.repo.SetSecret(ctx, c.email, r.ID, repository.CaseAlertSecret{URL: webhook}, hint)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	if !found {
		return domain.CaseAlertChatRoom{}, &apierror.NotFoundError{Msg: "no such chat room"}
	}
	updated, err := s.repo.GetRoom(ctx, r.ID)
	if err != nil {
		return domain.CaseAlertChatRoom{}, err
	}
	if updated == nil {
		return domain.CaseAlertChatRoom{}, &apierror.NotFoundError{Msg: "no such chat room"}
	}
	return roomView(*updated), nil
}

// DeleteRoom implements CaseAlertSettingsService. With the room gone, the
// ladder uses the file's room again.
func (s *caseAlertSettingsService) DeleteRoom(ctx context.Context, roomID string) error {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return err
	}
	r, err := s.room(ctx, c, roomID)
	if err != nil {
		return err
	}
	found, err := s.repo.DeleteRoom(ctx, c.email, r.ID)
	if err != nil {
		return err
	}
	if !found {
		return &apierror.NotFoundError{Msg: "no such chat room"}
	}
	return nil
}

// SendTestCard implements CaseAlertSettingsService: it posts a short card to
// the room's saved URL, now, and records whether Google Chat took it. One
// test per room every two minutes, as test calls.
func (s *caseAlertSettingsService) SendTestCard(ctx context.Context, roomID string) (domain.CaseAlertTestCardResponse, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertTestCardResponse{}, err
	}
	r, err := s.room(ctx, c, roomID)
	if err != nil {
		return domain.CaseAlertTestCardResponse{}, err
	}
	sec, err := s.repo.Secret(ctx, r.ID)
	if err != nil {
		return domain.CaseAlertTestCardResponse{}, err
	}
	if sec == nil {
		return domain.CaseAlertTestCardResponse{}, &apierror.ConflictError{Msg: "this room has no webhook URL saved here to test"}
	}
	webhook, err := s.webhookOf(r.ID, *sec)
	if err != nil {
		return domain.CaseAlertTestCardResponse{}, &apierror.ConflictError{Msg: err.Error()}
	}
	now := s.now().UTC().Truncate(time.Second)
	if r.LastTestAt != nil && now.Sub(*r.LastTestAt) < caseAlertTestCooldown {
		return domain.CaseAlertTestCardResponse{}, caseAlertCooldownError(now, *r.LastTestAt)
	}
	claimed, err := s.repo.ClaimTest(ctx, c.email, r.ID, now, now.Add(-caseAlertTestCooldown))
	if err != nil {
		return domain.CaseAlertTestCardResponse{}, err
	}
	if !claimed {
		return domain.CaseAlertTestCardResponse{}, &apierror.TooManyRequestsError{Msg: "a test card was sent less than two minutes ago"}
	}

	text := fmt.Sprintf("Test card for %s, sent from the Team Schedule's Alert Settings tab by %s. No action needed.", r.Label, c.email)
	status := caseAlertTestDelivered
	if perr := s.poster.Post(ctx, webhook, text); perr != nil {
		status = caseAlertTestFailed
		// perr never carries the URL (httpChatPoster).
		slog.WarnContext(ctx, "case alerts: test card not delivered", "roomId", r.ID, "reason", perr.Error())
	}
	if err := s.repo.RecordTest(ctx, c.email, r.ID, status, now); err != nil {
		return domain.CaseAlertTestCardResponse{}, err
	}
	return domain.CaseAlertTestCardResponse{LastTestStatus: status, LastTestAt: now}, nil
}

func caseAlertCooldownError(now, last time.Time) error {
	wait := caseAlertTestCooldown - now.Sub(last)
	return &apierror.TooManyRequestsError{Msg: fmt.Sprintf("a test card was sent less than two minutes ago; try again in %d seconds", int(wait.Seconds())+1)}
}

// SaveRoute implements CaseAlertSettingsService.
func (s *caseAlertSettingsService) SaveRoute(ctx context.Context, req domain.SaveCaseAlertRouteRequest) (domain.CaseAlertGroupRoute, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertGroupRoute{}, err
	}
	ladder, err := normLadder(req.Ladder)
	if err != nil {
		return domain.CaseAlertGroupRoute{}, err
	}
	if err := c.mayChange(ladder); err != nil {
		return domain.CaseAlertGroupRoute{}, err
	}
	w := repository.CaseAlertRouteWrite{
		Ladder:    ladder,
		GroupName: strings.ToLower(strings.TrimSpace(req.GroupName)),
		TeamKey:   strings.ToLower(strings.TrimSpace(req.TeamKey)),
		RotaCode:  strings.ToUpper(strings.TrimSpace(req.RotaCode)),
		Enabled:   true,
	}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}
	if w.GroupName == "" || len(w.GroupName) > maxCaseAlertGroupName {
		return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: fmt.Sprintf("groupName must be 1 to %d characters", maxCaseAlertGroupName)}
	}
	// A route matches a case's or incident's assignment group by name. A CRE
	// case's group is always a CSM Portal team. An SRE incident's may be a
	// ServiceNow group no CSM Portal team has ("WSO2 IaaS Team"), so the SRE
	// ladder also takes a name no team has -- but never a CRE team's, since
	// CRE and SRE never page for each other.
	groupFamily, ok, err := s.repo.TeamFamilyByName(ctx, w.GroupName)
	if err != nil {
		return domain.CaseAlertGroupRoute{}, err
	}
	allowed := routeGroupFamilies[ladder]
	if ok && !slices.Contains(allowed, groupFamily) || !ok && ladder != familySRE {
		return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: fmt.Sprintf("groupName must be the name of a CSM Portal %s team", strings.Join(allowed, " or "))}
	}
	if !ok {
		cre, err := s.repo.IsCRETeamName(ctx, w.GroupName)
		if err != nil {
			return domain.CaseAlertGroupRoute{}, err
		}
		if cre {
			return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: "groupName is a CRE team's; an SRE route cannot take a CRE team's incidents"}
		}
	}
	if (w.TeamKey == "") == (w.RotaCode == "") {
		return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: "send exactly one of teamKey or rotaCode"}
	}
	if w.TeamKey != "" {
		family, ok, err := s.repo.RosteredTeamFamily(ctx, w.TeamKey)
		if err != nil {
			return domain.CaseAlertGroupRoute{}, err
		}
		if !ok || family != ladder {
			return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: fmt.Sprintf("teamKey must be one of the Team Schedule's %s teams", ladder)}
		}
	} else {
		if ladder != familySRE {
			return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: "only SRE groups can go to a default rota"}
		}
		family, ok, err := s.repo.ActiveRotaFamily(ctx, w.RotaCode)
		if err != nil {
			return domain.CaseAlertGroupRoute{}, err
		}
		if !ok || family != familySRE {
			return domain.CaseAlertGroupRoute{}, &apierror.ValidationError{Msg: "rotaCode must be an active SRE rota"}
		}
	}
	row, err := s.repo.SaveRoute(ctx, c.email, w)
	if err != nil {
		return domain.CaseAlertGroupRoute{}, err
	}
	return routeView(row), nil
}

// DeleteRoute implements CaseAlertSettingsService. With the route gone, the
// group is routed by the file again.
func (s *caseAlertSettingsService) DeleteRoute(ctx context.Context, routeID string) error {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return err
	}
	if err := validateUUIDs("routeId", []string{routeID}); err != nil {
		return err
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return err
	}
	for _, r := range routes {
		if r.ID != routeID {
			continue
		}
		if err := c.mayChange(r.Ladder); err != nil {
			return err
		}
		found, err := s.repo.DeleteRoute(ctx, c.email, r.ID)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		break
	}
	return &apierror.NotFoundError{Msg: "no such route"}
}

// SaveValues implements CaseAlertSettingsService: every value is checked
// against its key's limits first, then all are saved together, so a refusal
// saves none of them.
func (s *caseAlertSettingsService) SaveValues(ctx context.Context, req domain.SaveCaseAlertValuesRequest) (domain.CaseAlertValuesResponse, error) {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return domain.CaseAlertValuesResponse{}, err
	}
	ladder, err := normValueLadder(req.Ladder)
	if err != nil {
		return domain.CaseAlertValuesResponse{}, err
	}
	if err := c.mayChange(ladder); err != nil {
		return domain.CaseAlertValuesResponse{}, err
	}
	if len(req.Values) == 0 || len(req.Values) > maxCaseAlertValues {
		return domain.CaseAlertValuesResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("send 1 to %d values", maxCaseAlertValues)}
	}
	keys := make([]string, 0, len(req.Values))
	for k := range req.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	checked := make(map[string][]byte, len(keys))
	for _, k := range keys {
		v, err := s.checkCaseAlertValue(ctx, ladder, k, req.Values[k])
		if err != nil {
			return domain.CaseAlertValuesResponse{}, err
		}
		checked[k] = v
	}
	if err := s.repo.SaveValues(ctx, c.email, ladder, checked); err != nil {
		return domain.CaseAlertValuesResponse{}, err
	}
	all, err := s.repo.ListValues(ctx)
	if err != nil {
		return domain.CaseAlertValuesResponse{}, err
	}
	out := domain.CaseAlertValuesResponse{Values: []domain.CaseAlertLadderValue{}}
	for _, v := range all {
		if v.Ladder == ladder {
			out.Values = append(out.Values, valueView(v))
		}
	}
	return out, nil
}

// DeleteValue implements CaseAlertSettingsService. With the value gone, the
// ladder uses the file's again.
func (s *caseAlertSettingsService) DeleteValue(ctx context.Context, valueID string) error {
	c, err := s.whoIsAsking(ctx)
	if err != nil {
		return err
	}
	if err := validateUUIDs("valueId", []string{valueID}); err != nil {
		return err
	}
	v, err := s.repo.GetValue(ctx, valueID)
	if err != nil {
		return err
	}
	if v == nil {
		return &apierror.NotFoundError{Msg: "no such saved setting"}
	}
	if err := c.mayChange(v.Ladder); err != nil {
		return err
	}
	found, err := s.repo.DeleteValue(ctx, c.email, v.ID)
	if err != nil {
		return err
	}
	if !found {
		return &apierror.NotFoundError{Msg: "no such saved setting"}
	}
	return nil
}

// webhookOf is a stored room's URL: as saved, or a URL sealed before 0237
// opened with CASE_ALERTS_SECRET_KEY. The error never quotes the URL.
func (s *caseAlertSettingsService) webhookOf(roomID string, sec repository.CaseAlertSecret) (string, error) {
	if sec.URL != "" {
		return sec.URL, nil
	}
	if s.sealer == nil {
		return "", errors.New("its URL was saved sealed under a key this deployment does not have; save the URL again")
	}
	return s.sealer.open(roomID, sec)
}

// runtimeCaller refuses a person, and any service but those
// CASE_ALERTS_RUNTIME_CLIENT_IDS names when it names any: with it unset, an
// internal service client (csm-notification-service's) may read.
func (s *caseAlertSettingsService) runtimeCaller(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, "the Case Alerts runtime settings are only for internal services"); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail != "" {
		return "", &apierror.ForbiddenError{Msg: "the Case Alerts runtime settings are read by csm-notification-service, not by a person"}
	}
	if id.ClientID == "" || (len(s.runtimes) > 0 && !s.runtimes[id.ClientID]) {
		return "", &apierror.ForbiddenError{Msg: "this client may not read the Case Alerts runtime settings (CASE_ALERTS_RUNTIME_CLIENT_IDS)"}
	}
	return id.ClientID, nil
}

// Runtime implements CaseAlertSettingsService: the enabled rooms whose URL
// opens, the enabled routes and the saved ladder values. A room whose URL
// does not open (no key here, or sealed under another one) is left out and
// logged, so the ladder keeps the file's room for it.
func (s *caseAlertSettingsService) Runtime(ctx context.Context) (domain.CaseAlertRuntimeSettings, error) {
	if _, err := s.runtimeCaller(ctx); err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	rev, err := s.repo.Revision(ctx)
	if err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	out := domain.CaseAlertRuntimeSettings{
		Revision: rev.Revision, Rooms: []domain.CaseAlertRuntimeRoom{},
		Routes: []domain.CaseAlertGroupRoute{}, Values: []domain.CaseAlertRuntimeValue{},
	}
	rooms, err := s.repo.ListRooms(ctx)
	if err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	secrets, err := s.repo.Secrets(ctx)
	if err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	for _, r := range rooms {
		if !r.Enabled {
			continue
		}
		// A team row saved for its channel alone goes without a URL; the
		// alert service then keeps the file's room for it.
		webhook := ""
		if r.HasSecret {
			if sec, ok := secrets[r.ID]; !ok {
				slog.WarnContext(ctx, "case alerts: room's URL left out of the runtime settings: it is not stored", "roomId", r.ID)
			} else if opened, err := s.webhookOf(r.ID, sec); err != nil {
				slog.WarnContext(ctx, "case alerts: room's URL left out of the runtime settings", "roomId", r.ID, "reason", err.Error())
			} else {
				webhook = opened
			}
		}
		if webhook == "" && r.Channel == "" {
			continue
		}
		out.Rooms = append(out.Rooms, domain.CaseAlertRuntimeRoom{
			Ladder: r.Ladder, TeamKey: r.TeamKey, Label: r.Label, Teamless: r.Teamless, WebhookURL: webhook, Channel: r.Channel,
		})
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	for _, r := range routes {
		if r.Enabled {
			out.Routes = append(out.Routes, routeView(r))
		}
	}
	values, err := s.repo.ListValues(ctx)
	if err != nil {
		return domain.CaseAlertRuntimeSettings{}, err
	}
	for _, v := range values {
		out.Values = append(out.Values, domain.CaseAlertRuntimeValue{Ladder: v.Ladder, Key: v.Key, Value: json.RawMessage(v.Value)})
	}
	return out, nil
}

// RecordApplied implements CaseAlertSettingsService: what
// csm-notification-service loaded at start-up, shown on the tab as "running".
func (s *caseAlertSettingsService) RecordApplied(ctx context.Context, report domain.CaseAlertAppliedReport) error {
	client, err := s.runtimeCaller(ctx)
	if err != nil {
		return err
	}
	mode := strings.ToLower(strings.TrimSpace(report.Mode))
	if !caseAlertModes[mode] {
		return &apierror.ValidationError{Msg: "mode must be off, shadow or db"}
	}
	if report.Revision < 0 {
		return &apierror.ValidationError{Msg: "revision must not be negative"}
	}
	if report.Summary != nil {
		b, err := json.Marshal(report.Summary)
		if err != nil || len(b) > maxCaseAlertSummary {
			return &apierror.ValidationError{Msg: fmt.Sprintf("summary must be JSON of at most %d bytes", maxCaseAlertSummary)}
		}
	}
	return s.repo.RecordApplied(ctx, "service:"+client, report.Revision, mode, report.Summary)
}
