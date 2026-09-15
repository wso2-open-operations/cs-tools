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

// Package outbox turns committed row changes into flow triggers.
//
// The ServiceNow flows this engine ports are record-triggered: "Change Request
// Updated where State changes to …". The native equivalent is a row changing in
// the CSM database, which an AFTER UPDATE trigger records in event_outbox
// together with the columns that actually differ (csm-sync-service migration
// 0045). This package drains those rows and hands each to the flow registry as
// an entity.changed event — the same shape a flow would see if the event had
// arrived over the bus, so a flow cannot tell the two apart and no flow needs
// changing if the source ever moves.
package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/store"
)

// Claimer is the slice of *store.Store this package needs.
type Claimer interface {
	ClaimChanges(ctx context.Context, entityTypes []string, limit int) ([]store.Change, error)
}

// Dispatcher is the slice of *flows.Registry this package needs. Taking the
// registry as an interface keeps this package from importing flows, which would
// make the dependency cycle flows -> store -> outbox -> flows.
type Dispatcher interface {
	Handle(ctx context.Context, rec eventbus.Record) error
}

// Drainer polls event_outbox and dispatches what it finds.
type Drainer struct {
	Claimer    Claimer
	Dispatcher Dispatcher
	// EntityTypes limits what is drained. Only the types some registered flow
	// actually reacts to: the outbox is shared, and another consumer's rows are
	// none of this engine's business.
	EntityTypes []string
	// Interval is how often to poll when the last drain came back empty.
	Interval time.Duration
	// BatchSize caps one claim.
	BatchSize int
}

// Run drains until ctx is cancelled.
//
// It polls rather than listening. LISTEN/NOTIFY would be lower latency, but it
// is fire-and-forget: a listener disconnected by a redeploy misses every event
// sent meanwhile with no way to discover what it missed. Rows here persist
// until claimed, so a restart resumes instead of skipping — worth a few seconds
// of latency for an approval notice.
//
// A non-empty batch is followed immediately by another claim rather than a
// sleep, so a backlog drains at full speed and the interval only governs the
// idle case.
func (d *Drainer) Run(ctx context.Context) {
	if d.Interval <= 0 {
		d.Interval = 5 * time.Second
	}
	if d.BatchSize <= 0 {
		d.BatchSize = 100
	}

	for {
		n, err := d.DrainOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "outbox: drain failed", "err", err)
		}
		if n > 0 && ctx.Err() == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(d.Interval):
		}
	}
}

// DrainOnce claims one batch and dispatches it, returning how many rows it
// took. Run calls it in a loop; it is exported so a test — or cmd/dryrun —
// can drain exactly once without starting a polling goroutine and racing it.
//
// Claiming is a WRITE: the rows it returns are marked published and will not be
// seen again, whatever the caller then does with them.
func (d *Drainer) DrainOnce(ctx context.Context) (int, error) {
	changes, err := d.Claimer.ClaimChanges(ctx, d.EntityTypes, d.BatchSize)
	if err != nil {
		return 0, err
	}
	for _, c := range changes {
		// A fresh correlation id per change, so one row's whole journey is
		// traceable the same way a consumed record's is.
		cctx := middleware.WithCorrelationID(ctx, middleware.NewCorrelationID())
		rec, err := recordFor(c)
		if err != nil {
			// Undispatchable and already claimed: log it rather than wedging
			// the drain on one malformed row forever.
			slog.ErrorContext(cctx, "outbox: undispatchable row dropped",
				"outboxId", c.ID, "entityType", c.EntityType, "err", err)
			continue
		}
		if err := d.Dispatcher.Handle(cctx, rec); err != nil {
			// The row is claimed, so this notice is lost. Retrying here would
			// mean re-running every flow that already succeeded for it, which
			// the registry explicitly cannot do safely yet.
			slog.ErrorContext(cctx, "outbox: flow dispatch failed",
				"outboxId", c.ID, "entityType", c.EntityType, "entityId", c.EntityID, "err", err)
		}
	}
	return len(changes), nil
}

// recordFor renders one outbox row as the bus record a flow expects.
//
// NoMoreRetries is true because there is no further tier: an outbox row is
// claimed once and never redelivered, so a flow that keys off that flag must
// treat this as its final chance rather than waiting for a retry that is not
// coming.
func recordFor(c store.Change) (eventbus.Record, error) {
	payload, err := json.Marshal(events.EntityChangedPayload{
		EntityType: c.EntityType,
		EntityID:   c.EntityID,
		Changes:    c.Changes,
		Snapshot:   c.Snapshot,
	})
	if err != nil {
		return eventbus.Record{}, err
	}
	value, err := json.Marshal(events.Envelope{
		Type:     events.TypeEntityChanged,
		EntityID: c.EntityID,
		Payload:  payload,
	})
	if err != nil {
		return eventbus.Record{}, err
	}
	return eventbus.Record{
		Topic:          "event_outbox",
		Offset:         c.ID,
		Key:            []byte(c.EntityID),
		Value:          value,
		IsFinalAttempt: true,
		NoMoreRetries:  true,
	}, nil
}
