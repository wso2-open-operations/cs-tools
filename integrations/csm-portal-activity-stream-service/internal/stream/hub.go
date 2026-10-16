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

// Package stream is the in-process pub-sub side of case-activity SSE: a
// BroadcastHub that fans a per-case string payload out to every open
// /cases/{id}/activities/stream connection on this replica. It carries no
// auth or transport concerns of its own — the HTTP handler that registers
// with it (internal/handler.StreamCaseActivities) sits behind the same
// middleware.Auth chain as every other endpoint.
package stream

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// subscriberBuffer bounds each subscriber channel so Publish never blocks on
// a slow reader — a client that isn't draining its channel fast enough just
// misses the newest ping (see Publish); it still gets the next one, and a
// browser SSE reconnect (or the frontend's own query staleTime) covers the
// gap either way.
const subscriberBuffer = 4

// Replay bounds. The hub remembers the most recent ReplayCapacity events
// (across all cases) for at most ReplayWindow, so a client reconnecting with
// Last-Event-ID shortly after a drop can be sent what it missed. This is
// best-effort only: memory is bounded, event IDs are local to one hub (one
// process), so a reconnect that lands on another replica — or arrives after a
// restart, or after the window — gets no replay. Clients must still refresh
// their state on (re)connect for correctness.
const (
	ReplayCapacity = 256
	ReplayWindow   = 2 * time.Minute
)

// Event is one published payload with its stream-unique ID, which the SSE
// handler writes as the `id:` field.
type Event struct {
	ID   string
	Data string

	caseID string
	seq    uint64
	at     time.Time
}

// BroadcastHub fans a payload out to every subscriber registered for a given
// case ID. Safe for concurrent use.
type BroadcastHub struct {
	mu     sync.Mutex
	subs   map[string]map[chan Event]bool
	closed bool

	epoch  string // distinguishes this hub's IDs from any other process's
	seq    uint64
	recent []Event // ring of the last ReplayCapacity events, oldest first
	now    func() time.Time
}

// NewBroadcastHub constructs an empty BroadcastHub.
func NewBroadcastHub() *BroadcastHub {
	return &BroadcastHub{
		subs:   make(map[string]map[chan Event]bool),
		epoch:  strconv.FormatInt(time.Now().UnixNano(), 36),
		recent: make([]Event, 0, ReplayCapacity),
		now:    time.Now,
	}
}

// Register opens a new live-only subscription for caseID; see Subscribe.
func (h *BroadcastHub) Register(caseID string) chan Event {
	ch, _ := h.Subscribe(caseID, "")
	return ch
}

// Subscribe opens a new subscription for caseID and returns the channel to
// read from, plus — when lastEventID is an ID this hub issued — the retained
// events for caseID published after it, oldest first (see ReplayCapacity).
// Registration and the replay snapshot happen under one lock, so no event is
// both replayed and delivered live, and none falls between the two.
//
// Call Unregister with the same caseID/channel when the caller is done
// listening (e.g. the SSE request's context is done) — the channel is closed
// there, not here. After CloseAll, Subscribe returns an already-closed
// channel and no replay, so a late subscriber sees the same shutdown signal
// as every earlier one.
func (h *BroadcastHub) Subscribe(caseID, lastEventID string) (chan Event, []Event) {
	ch := make(chan Event, subscriberBuffer)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, nil
	}
	var missed []Event
	if after, ok := h.parseID(lastEventID); ok {
		cutoff := h.now().Add(-ReplayWindow)
		for _, ev := range h.recent {
			if ev.caseID == caseID && ev.seq > after && !ev.at.Before(cutoff) {
				missed = append(missed, ev)
			}
		}
	}
	if h.subs[caseID] == nil {
		h.subs[caseID] = make(map[chan Event]bool)
	}
	h.subs[caseID][ch] = true
	return ch, missed
}

// parseID returns the sequence number of an ID this hub issued. Foreign,
// malformed or future IDs are rejected (no replay).
func (h *BroadcastHub) parseID(id string) (uint64, bool) {
	epoch, seqStr, ok := strings.Cut(id, "-")
	if !ok || epoch != h.epoch {
		return 0, false
	}
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil || seq > h.seq {
		return 0, false
	}
	return seq, true
}

// Unregister removes ch from caseID's subscriber set and closes it. Safe to
// call exactly once per channel returned by Register; call it via defer in
// the same goroutine that reads from the channel.
func (h *BroadcastHub) Unregister(caseID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subs[caseID]; ok {
		if _, ok := subs[ch]; ok {
			delete(subs, ch)
			close(ch)
		}
		if len(subs) == 0 {
			delete(h.subs, caseID)
		}
	}
}

// Publish assigns payload the next event ID, retains it for replay, and fans
// it out to every subscriber currently registered for caseID. A subscriber
// whose buffer is full (it isn't draining fast enough) is skipped rather than
// blocking every other subscriber and the caller — see subscriberBuffer.
func (h *BroadcastHub) Publish(caseID, payload string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	ev := Event{
		ID:     h.epoch + "-" + strconv.FormatUint(h.seq, 10),
		Data:   payload,
		caseID: caseID,
		seq:    h.seq,
		at:     h.now(),
	}
	if len(h.recent) == ReplayCapacity {
		copy(h.recent, h.recent[1:])
		h.recent = h.recent[:ReplayCapacity-1]
	}
	h.recent = append(h.recent, ev)

	for ch := range h.subs[caseID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

// CloseAll closes every subscriber channel and refuses further
// registrations. It is the hub's shutdown hook: a closed channel is how
// StreamCaseActivities learns the process is going away, so it can send a
// terminal event and return — which is what lets http.Server.Shutdown finish
// instead of waiting out its deadline on connections that are never idle.
// Safe to call more than once; a later Unregister of a channel closed here is
// a no-op.
func (h *BroadcastHub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for caseID, subs := range h.subs {
		for ch := range subs {
			close(ch)
		}
		delete(h.subs, caseID)
	}
}
