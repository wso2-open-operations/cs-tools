// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package engine folds alerts into incidents by fingerprint and delivers incidents to CSM and Chat.
package engine

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"alert-core-service/internal/model"
	"alert-core-service/internal/store"
)

// incidentStore lets tests fake *store.IncidentRepo.
type incidentStore interface {
	Fold(ctx context.Context, fp string, alertIDs []string, decide store.Decide) (store.FoldPlan, error)
	ListDue(ctx context.Context, limit int) ([]int64, error)
	TryLock(ctx context.Context, id int64) (unlock func(), ok bool, err error)
	Get(ctx context.Context, id int64) (model.Incident, bool, error)
	PendingNotes(ctx context.Context, id int64, limit int) ([]model.Note, error)
	RecordCSMAttemptStarted(ctx context.Context, id int64, attempts int) error
	RecordCSMIncident(ctx context.Context, id int64, csmID, number string) error
	RecordCSMAttemptFailure(ctx context.Context, id int64, permanent bool) error
	MarkFallback(ctx context.Context, id int64) error
	SyncStatus(ctx context.Context, id int64, status string, checkedAt time.Time) error
	ClearNotes(ctx context.Context, incident int64, noteIDs []int64, csm, chat bool) error
	SettleNotes(ctx context.Context, incident int64, csm, chat bool) error
	FinishDelivery(ctx context.Context, id, foldVersion int64, next *time.Time) error
}

// notifier lets tests fake *notify.Notifier.
type notifier interface {
	// CSMEnabled is false when CSM credentials aren't configured; incidents then reach Chat only.
	CSMEnabled() bool
	NotifyCSM(ctx context.Context, inc model.Incident, creationNote string) (incidentID, incidentNumber string, ok bool, permanent bool)
	NotifyChat(ctx context.Context, inc model.Incident) (ok bool)
	NotifyChatAnnotation(ctx context.Context, inc model.Incident, text string) (ok bool)
	PushWorkNote(ctx context.Context, incidentID, note string) error
	IncidentState(ctx context.Context, incidentID, incidentNumber string) (open bool, found bool, err error)
}

// CSMRetryConfig bounds CSM create retries: waits grow BaseDelay, BaseDelay*Multiplier, ..., capped at MaxDelay.
type CSMRetryConfig struct {
	BaseDelay  time.Duration
	Multiplier float64
	MaxDelay   time.Duration
}

// Config tunes folding and delivery.
type Config struct {
	Defaults model.Defaults
	// DedupWindow is how long after an incident's first alert later alerts still fold into it.
	DedupWindow time.Duration
	// MaxCSMAttempts caps failed CreateIncident attempts before giving up on the incident.
	MaxCSMAttempts int
	// StateCheckInterval throttles CSM status refreshes per incident.
	StateCheckInterval time.Duration
	CSMRetry           CSMRetryConfig
	// ChatFallbackDelay is how long CSM gets to confirm a new incident before the Chat fallback card is posted; zero posts on the first failure.
	ChatFallbackDelay time.Duration
	// ChatThreadingEnabled sends a digest of Duplicate/OK alerts as a thread reply while an incident is only in Chat.
	ChatThreadingEnabled bool
	// DeliveryConcurrency bounds how many incidents DeliverDue works on at once per replica.
	DeliveryConcurrency int
}

// Engine folds alert groups and delivers due incidents.
type Engine struct {
	logger    *slog.Logger
	incidents incidentStore
	notifier  notifier
	cfg       Config
}

// New wires the engine.
func New(logger *slog.Logger, incidents incidentStore, n notifier, cfg Config) *Engine {
	cfg.DeliveryConcurrency = max(cfg.DeliveryConcurrency, 1)
	return &Engine{logger: logger, incidents: incidents, notifier: n, cfg: cfg}
}

// Normalize applies configured field defaults to a freshly decoded alert.
func (e *Engine) Normalize(alert model.Alert) model.Alert {
	e.cfg.Defaults.Apply(&alert)
	return alert
}

// Item is one claimed, normalized alert.
type Item struct {
	ID    string
	Alert model.Alert
}

// HandleGroup folds every alert of one fingerprint in a single transaction; a nil error means all are recorded, an error means none are and all should be retried.
func (e *Engine) HandleGroup(ctx context.Context, fp string, items []Item) error {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	plan, err := e.incidents.Fold(ctx, fp, ids, func(current *model.Incident, recorded map[string]bool) store.FoldPlan {
		return e.plan(fp, current, recorded, items)
	})
	if err != nil {
		e.logger.Warn("fold failed, will retry", "fingerprint", fp, "alerts", len(items), "error", err)
		return err
	}
	for _, n := range plan.New {
		e.logger.Info("incident created", "fingerprint", fp, "alert_id", n.Notes[0].AlertID,
			"service", n.Incident.Service, "metric_name", n.Incident.MetricName, "severity", n.Incident.Severity,
			"first_seen", n.Incident.FirstSeen, "alerts", n.Incident.AlertCount)
	}
	if c := plan.Current; c != nil {
		e.logger.Debug("alerts folded into existing incident", "fingerprint", fp, "incident", c.ID, "alerts", c.Added)
	}
	return nil
}

// plan decides, in arrival order, whether each alert opens an incident or is a Duplicate/OK note on the open one; it touches no I/O.
func (e *Engine) plan(fp string, current *model.Incident, recorded map[string]bool, items []Item) store.FoldPlan {
	sorted := slices.Clone(items)
	slices.SortStableFunc(sorted, func(a, b Item) int {
		return cmp.Or(a.Alert.EventTime().Compare(b.Alert.EventTime()), cmp.Compare(a.ID, b.ID))
	})

	var p store.FoldPlan
	target := current
	newIdx := -1
	add := func(it Item, kind, label string) {
		note := model.Note{AlertID: it.ID, Kind: kind, Text: model.BuildWorkNote(label, it.ID, it.Alert.MetricName, it.Alert.Source), ChatPending: true}
		at := it.Alert.EventTime()
		if newIdx >= 0 {
			n := &p.New[newIdx]
			n.Notes = append(n.Notes, note)
			n.Incident.AlertCount++
			n.Incident.LastSeen = latest(n.Incident.LastSeen, at)
			return
		}
		if p.Current == nil {
			p.Current = &store.CurrentFold{ID: current.ID, LastSeen: current.LastSeen}
		}
		p.Current.Added++
		p.Current.LastSeen = latest(p.Current.LastSeen, at)
		if p.Current.Category == "" {
			p.Current.Category = it.Alert.Category
		}
		p.Current.Notes = append(p.Current.Notes, note)
	}

	for _, it := range sorted {
		if recorded[it.ID] {
			continue // replayed after a crash or reclaim; already folded.
		}
		severity, ok := model.SeverityToNumeric(it.Alert.Severity)
		if !ok {
			// Log loudly: a silent default to Critical could page people for a typo.
			e.logger.Warn("unrecognized severity label, defaulting to critical", "alert_id", it.ID, "severity", it.Alert.Severity)
		}
		at := it.Alert.EventTime()
		switch {
		case model.IsResolving(severity) && target == nil:
			e.logger.Info("resolving alert with no matching incident, ignoring", "alert_id", it.ID, "fingerprint", fp)
		case model.IsResolving(severity):
			add(it, model.NoteOK, "OK")
		case target != nil && target.IsOpen(at, e.cfg.DedupWindow):
			add(it, model.NoteDuplicate, "Duplicate")
		default:
			if target != nil && !at.After(target.FirstSeen) {
				at = target.FirstSeen.Add(time.Microsecond) // keeps (fingerprint, first_seen) unique after a CSM close.
			}
			impact, urgency := model.ImpactUrgency(severity)
			p.New = append(p.New, store.NewIncident{
				Incident: model.Incident{
					Fingerprint: fp, IncidentNumber: "PENDING-" + fp[:12], Status: "new",
					Severity: severity, Impact: impact, Urgency: urgency,
					Service: it.Alert.Service, MetricName: it.Alert.MetricName, Category: it.Alert.Category,
					Environment: it.Alert.Environment, Source: it.Alert.Source,
					AlertCount: 1, FirstSeen: at, LastSeen: at,
				},
				Notes: []model.Note{{AlertID: it.ID, Kind: model.NoteCreated, Text: model.BuildCreationNote(it.ID, it.Alert)}},
			})
			newIdx = len(p.New) - 1
			target = &p.New[newIdx].Incident
			target.TakeRouting(it.Alert) // recorded for reference; entity-service assigns the group from the service
		}
	}
	return p
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// deliverySweepLimit bounds one sweep's read, longest-waiting first, so a large backlog can't make every sweep load the whole table.
const deliverySweepLimit = 2000

// notesPerDelivery bounds how many pending notes one delivery reads; the rest are picked up on the next sweep.
const notesPerDelivery = 500

// DeliverDue delivers every due incident across DeliveryConcurrency workers; a per-incident try-lock keeps workers and replicas from double-delivering.
func (e *Engine) DeliverDue(ctx context.Context) {
	due, err := e.incidents.ListDue(ctx, deliverySweepLimit)
	if err != nil {
		e.logger.Error("delivery sweep: failed to list due incidents", "error", err)
		return
	}
	jobs := make(chan int64)
	var wg sync.WaitGroup
	for range min(e.cfg.DeliveryConcurrency, len(due)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				e.deliver(ctx, id)
			}
		}()
	}
	for _, id := range due {
		if ctx.Err() != nil {
			break
		}
		jobs <- id
	}
	close(jobs)
	wg.Wait()
}

// persistTimeout bounds recording a delivery result after its external call already completed.
const persistTimeout = 5 * time.Second

// persistCtx survives parent cancellation so results record even on shutdown.
func persistCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
}

// delivery carries one incident's state through a delivery pass.
type delivery struct {
	e     *Engine
	ctx   context.Context
	inc   model.Incident
	notes []model.Note
	// retryAt is the earliest time something left undone should be retried; zero means nothing.
	retryAt time.Time
}

func (d *delivery) retryAfter(t time.Time) {
	if d.retryAt.IsZero() || t.Before(d.retryAt) {
		d.retryAt = t
	}
}

func (d *delivery) retrySoon() { d.retryAfter(time.Now().Add(d.e.cfg.CSMRetry.BaseDelay)) }

// persist runs a store write that must land even if ctx is cancelled; on failure the pass is retried soon.
func (d *delivery) persist(what string, write func(context.Context) error) bool {
	pctx, cancel := persistCtx(d.ctx)
	defer cancel()
	if err := write(pctx); err != nil {
		d.e.logger.Error("delivery: failed to persist "+what, "incident", d.inc.ID, "incident_number", d.inc.IncidentNumber, "error", err)
		d.retrySoon()
		return false
	}
	return true
}

// deliver does everything incident id owes, then schedules its next delivery.
func (e *Engine) deliver(ctx context.Context, id int64) {
	unlock, ok, err := e.incidents.TryLock(ctx, id)
	if err != nil {
		e.logger.Error("delivery: failed to acquire incident lock, will retry next sweep", "incident", id, "error", err)
		return
	}
	if !ok {
		return
	}
	defer unlock()

	inc, found, err := e.incidents.Get(ctx, id)
	if err != nil || !found {
		if err != nil {
			e.logger.Error("delivery: failed to read incident", "incident", id, "error", err)
		}
		return
	}
	notes, err := e.incidents.PendingNotes(ctx, id, notesPerDelivery)
	if err != nil {
		e.logger.Error("delivery: failed to read pending notes", "incident", id, "error", err)
		return
	}
	d := &delivery{e: e, ctx: ctx, inc: inc, notes: notes}
	csmEnabled := e.notifier.CSMEnabled()

	if csmEnabled && !inc.CSMConfirmed && !inc.CSMPermanentlyFailed && inc.CSMRetryDue(time.Now(), e.cfg.CSMRetry.BaseDelay, e.cfg.CSMRetry.Multiplier, e.cfg.CSMRetry.MaxDelay) {
		d.createInCSM()
	}
	if !csmEnabled || d.inc.CSMPermanentlyFailed {
		d.settle(true, false)
	}
	if d.inc.CSMConfirmed {
		d.pushNotes()
	}
	d.notifyChat()
	if csmEnabled {
		d.refreshStatus()
	}

	if csmEnabled && !d.inc.CSMConfirmed && !d.inc.CSMPermanentlyFailed {
		d.retryAfter(d.inc.NextCSMRetry(e.cfg.CSMRetry.BaseDelay, e.cfg.CSMRetry.Multiplier, e.cfg.CSMRetry.MaxDelay))
	}
	if len(notes) == notesPerDelivery && slices.ContainsFunc(d.notes, func(n model.Note) bool { return !n.CSMPending && !n.ChatPending }) {
		d.retryAfter(time.Now()) // more notes than one pass reads, and this pass cleared some.
	}
	var next *time.Time
	if !d.retryAt.IsZero() {
		next = &d.retryAt
	}
	d.persist("next delivery time", func(c context.Context) error {
		return e.incidents.FinishDelivery(c, id, inc.FoldVersion, next)
	})
}

// createInCSM attempts CreateIncident once, carrying the creation note, and records the outcome.
func (d *delivery) createInCSM() {
	e := d.e
	attempts := d.inc.CSMAttempts + 1
	// Persist the attempt first so CSMAttempts is a lower bound on creates that may have reached CSM.
	if !d.persist("csm attempt start", func(c context.Context) error {
		return e.incidents.RecordCSMAttemptStarted(c, d.inc.ID, attempts)
	}) {
		return
	}
	d.inc.CSMAttempts, d.inc.CSMLastAttemptAt = attempts, time.Now()

	creation := slices.IndexFunc(d.notes, func(n model.Note) bool { return n.Kind == model.NoteCreated && n.CSMPending })
	var creationText string
	if creation >= 0 {
		creationText = d.notes[creation].Text
	}
	csmID, number, ok, permanent := e.notifier.NotifyCSM(d.ctx, d.inc, creationText)
	if !ok {
		permanent = permanent || attempts >= e.cfg.MaxCSMAttempts
		if d.persist("csm failure", func(c context.Context) error {
			return e.incidents.RecordCSMAttemptFailure(c, d.inc.ID, permanent)
		}) && permanent {
			d.inc.CSMPermanentlyFailed = true
			e.logger.Error("csm permanently failed for incident, giving up", "incident", d.inc.ID, "incident_number", d.inc.IncidentNumber, "attempts", attempts)
		}
		return
	}
	// If this write fails, the next attempt creates the CSM incident again: CSM is not searched for a prior create.
	if !d.persist("csm confirmation", func(c context.Context) error {
		return e.incidents.RecordCSMIncident(c, d.inc.ID, csmID, number)
	}) {
		return
	}
	d.inc.CSMConfirmed, d.inc.IncidentID, d.inc.IncidentNumber = true, csmID, number
	d.inc.Status, d.inc.StateCheckedAt = "open", time.Now()
	if creation >= 0 && d.persist("creation note", func(c context.Context) error {
		return e.incidents.ClearNotes(c, d.inc.ID, []int64{d.notes[creation].ID}, true, false)
	}) {
		d.notes[creation].CSMPending = false
	}
}

// pushNotes pushes CSM-pending notes in order, stopping at the first failure.
func (d *delivery) pushNotes() {
	var pushed []int64
	for i := range d.notes {
		n := &d.notes[i]
		if !n.CSMPending {
			continue
		}
		if err := d.e.notifier.PushWorkNote(d.ctx, d.inc.IncidentID, n.Text); err != nil {
			d.e.logger.Warn("failed to push work note to csm, will retry", "incident_number", d.inc.IncidentNumber, "error", err)
			d.retrySoon()
			break
		}
		pushed = append(pushed, n.ID)
		n.CSMPending = false
	}
	if len(pushed) > 0 {
		d.persist("pushed notes", func(c context.Context) error {
			return d.e.incidents.ClearNotes(c, d.inc.ID, pushed, true, false)
		})
	}
}

// notifyChat posts the fallback card once CSM has gone ChatFallbackDelay without confirming, then threads one digest reply covering every Duplicate/OK alert since the last one.
func (d *delivery) notifyChat() {
	e := d.e
	if !d.inc.CSMConfirmed && !d.inc.Fallback {
		// CSM still has a chance to confirm; without CSM, or after a permanent rejection, there is nothing to wait for.
		if e.notifier.CSMEnabled() && !d.inc.CSMPermanentlyFailed {
			if postAt := d.inc.CreatedAt.Add(e.cfg.ChatFallbackDelay); time.Now().Before(postAt) {
				d.retryAfter(postAt)
				return
			}
		}
		if !e.notifier.NotifyChat(d.ctx, d.inc) {
			d.retrySoon()
			return
		}
		if d.persist("fallback flag", func(c context.Context) error { return e.incidents.MarkFallback(c, d.inc.ID) }) {
			d.inc.Fallback = true
		}
	}

	var ids []int64
	var duplicates, oks int
	for _, n := range d.notes {
		if !n.ChatPending {
			continue
		}
		ids = append(ids, n.ID)
		switch n.Kind {
		case model.NoteDuplicate:
			duplicates++
		case model.NoteOK:
			oks++
		}
	}
	if len(ids) == 0 {
		return
	}
	if e.cfg.ChatThreadingEnabled && d.inc.Fallback && !d.inc.CSMConfirmed {
		text := model.BuildChatDigest(duplicates, oks, d.inc.MetricName, d.inc.Source)
		if !e.notifier.NotifyChatAnnotation(d.ctx, d.inc, text) {
			e.logger.Warn("chat thread reply failed", "incident", d.inc.ID, "incident_number", d.inc.IncidentNumber)
			d.retrySoon()
			return
		}
	} else if !d.inc.CSMConfirmed && e.cfg.ChatThreadingEnabled {
		return // the fallback card hasn't landed yet; keep the replies for after it.
	}
	d.persist("chat notes", func(c context.Context) error { return e.incidents.ClearNotes(c, d.inc.ID, ids, false, true) })
}

// settle drops obligations that can never be met, so their notes leave the pending index.
func (d *delivery) settle(csm, chat bool) {
	if !slices.ContainsFunc(d.notes, func(n model.Note) bool { return (csm && n.CSMPending) || (chat && n.ChatPending) }) {
		return
	}
	if !d.persist("settled notes", func(c context.Context) error { return d.e.incidents.SettleNotes(c, d.inc.ID, csm, chat) }) {
		return
	}
	for i := range d.notes {
		d.notes[i].CSMPending = d.notes[i].CSMPending && !csm
		d.notes[i].ChatPending = d.notes[i].ChatPending && !chat
	}
}

// refreshStatus re-reads a confirmed incident's CSM status while it can still absorb alerts and has had new ones since the last check.
func (d *delivery) refreshStatus() {
	e := d.e
	now := time.Now()
	inc := d.inc
	if !inc.CSMConfirmed || now.Sub(inc.FirstSeen) >= e.cfg.DedupWindow ||
		!inc.LastSeen.After(inc.StateCheckedAt) || now.Sub(inc.StateCheckedAt) < e.cfg.StateCheckInterval {
		return
	}
	open, found, err := e.notifier.IncidentState(d.ctx, inc.IncidentID, inc.IncidentNumber)
	if err != nil {
		e.logger.Warn("csm incident state check failed, using last known state", "incident_number", inc.IncidentNumber, "error", err)
		return
	}
	status := inc.Status
	if found {
		status = "closed"
		if open {
			status = "open"
		}
	}
	if d.persist("synced status", func(c context.Context) error { return e.incidents.SyncStatus(c, inc.ID, status, now) }) {
		d.inc.Status, d.inc.StateCheckedAt = status, now
	}
}
