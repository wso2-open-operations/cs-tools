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

package stream

import (
	"sync"
	"testing"
	"time"
)

func TestBroadcastHub_PublishReachesSubscriber(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")
	h.Publish("case-1", "hello")

	select {
	case got := <-ch:
		if got.Data != "hello" {
			t.Errorf("got %q, want %q", got.Data, "hello")
		}
	default:
		t.Fatal("expected a message on the channel")
	}
}

func TestBroadcastHub_PublishToDifferentCase_NotDelivered(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")
	h.Publish("case-2", "hello")

	select {
	case got := <-ch:
		t.Fatalf("unexpected message %q for an unrelated case", got.Data)
	default:
	}
}

func TestBroadcastHub_PublishWithNoSubscribers_NoOp(t *testing.T) {
	h := NewBroadcastHub()
	h.Publish("case-1", "hello") // must not panic
}

func TestBroadcastHub_MultipleSubscribersAllReceive(t *testing.T) {
	h := NewBroadcastHub()
	ch1 := h.Register("case-1")
	ch2 := h.Register("case-1")
	h.Publish("case-1", "hello")

	for _, ch := range []chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Data != "hello" {
				t.Errorf("got %q, want %q", got.Data, "hello")
			}
		default:
			t.Fatal("expected a message on the channel")
		}
	}
}

func TestBroadcastHub_UnregisterClosesChannel(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")
	h.Unregister("case-1", ch)

	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed after Unregister")
	}
}

func TestBroadcastHub_UnregisterThenPublish_NoOp(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")
	h.Unregister("case-1", ch)
	h.Publish("case-1", "hello") // must not panic (send on closed channel would)
}

func TestBroadcastHub_PublishDropsWhenBufferFull(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")

	// Publish well past the buffer's capacity; must not block.
	for i := 0; i < subscriberBuffer+5; i++ {
		h.Publish("case-1", "msg")
	}

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			if count > subscriberBuffer {
				t.Errorf("received %d messages, want at most %d (buffer size)", count, subscriberBuffer)
			}
			return
		}
	}
}

func TestBroadcastHub_ConcurrentRegisterPublishUnregister(t *testing.T) {
	h := NewBroadcastHub()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := h.Register("case-1")
			h.Publish("case-1", "x")
			h.Unregister("case-1", ch)
		}()
	}
	wg.Wait()
}

func TestBroadcastHub_CloseAllClosesEverySubscriber(t *testing.T) {
	h := NewBroadcastHub()
	a := h.Register("case-1")
	b := h.Register("case-1")
	c := h.Register("case-2")
	h.CloseAll()

	for i, ch := range []chan Event{a, b, c} {
		if _, ok := <-ch; ok {
			t.Fatalf("subscriber %d still open after CloseAll", i)
		}
	}
	// Deferred Unregister calls from the handlers must not panic
	// (double close), and Publish must be a no-op.
	h.Unregister("case-1", a)
	h.Unregister("case-2", c)
	h.Publish("case-1", "x")
	h.CloseAll() // idempotent
}

func TestBroadcastHub_RegisterAfterCloseAllReturnsClosedChannel(t *testing.T) {
	h := NewBroadcastHub()
	h.CloseAll()
	ch := h.Register("case-1")
	if _, ok := <-ch; ok {
		t.Fatal("Register after CloseAll returned an open channel")
	}
	h.Unregister("case-1", ch) // must not panic
}

func TestBroadcastHub_EventIDsAreUniqueAndIncreasing(t *testing.T) {
	h := NewBroadcastHub()
	ch := h.Register("case-1")
	h.Publish("case-1", "a")
	h.Publish("case-1", "b")
	first, second := <-ch, <-ch
	if first.ID == "" || first.ID == second.ID {
		t.Fatalf("IDs not unique: %q, %q", first.ID, second.ID)
	}
	if first.seq >= second.seq {
		t.Errorf("seq not increasing: %d then %d", first.seq, second.seq)
	}
}

func TestBroadcastHub_SubscribeReplaysMissedEventsForCase(t *testing.T) {
	h := NewBroadcastHub()
	live := h.Register("case-1")
	h.Publish("case-1", "a")
	h.Publish("case-2", "other")
	h.Publish("case-1", "b")
	h.Publish("case-1", "c")
	a := <-live

	_, missed := h.Subscribe("case-1", a.ID)
	if len(missed) != 2 || missed[0].Data != "b" || missed[1].Data != "c" {
		t.Fatalf("replay = %+v, want [b c] for case-1 only", dataOf(missed))
	}
}

func TestBroadcastHub_SubscribeIgnoresForeignOrBogusIDs(t *testing.T) {
	h := NewBroadcastHub()
	h.Publish("case-1", "a")
	for _, id := range []string{"", "nope", "otherepoch-0", h.epoch + "-x", h.epoch + "-999"} {
		if _, missed := h.Subscribe("case-1", id); len(missed) != 0 {
			t.Errorf("Subscribe(%q) replayed %v, want nothing", id, dataOf(missed))
		}
	}
	// Seq 0 of this hub is valid and means "everything retained".
	if _, missed := h.Subscribe("case-1", h.epoch+"-0"); len(missed) != 1 {
		t.Errorf("Subscribe(epoch-0) replayed %d events, want 1", len(missed))
	}
}

func TestBroadcastHub_ReplayHonoursWindow(t *testing.T) {
	h := NewBroadcastHub()
	now := time.Now()
	h.now = func() time.Time { return now }
	h.Publish("case-1", "old")
	now = now.Add(ReplayWindow + time.Second)
	h.Publish("case-1", "fresh")

	_, missed := h.Subscribe("case-1", h.epoch+"-0")
	if len(missed) != 1 || missed[0].Data != "fresh" {
		t.Fatalf("replay = %v, want only the event inside the window", dataOf(missed))
	}
}

func TestBroadcastHub_ReplayBufferIsBounded(t *testing.T) {
	h := NewBroadcastHub()
	for i := 0; i < ReplayCapacity+50; i++ {
		h.Publish("case-1", "x")
	}
	if len(h.recent) != ReplayCapacity {
		t.Fatalf("retained %d events, want %d", len(h.recent), ReplayCapacity)
	}
	_, missed := h.Subscribe("case-1", h.epoch+"-0")
	if len(missed) != ReplayCapacity || missed[0].seq != 51 {
		t.Errorf("replay len=%d first seq=%d, want %d starting at 51", len(missed), missed[0].seq, ReplayCapacity)
	}
}

func dataOf(evs []Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Data
	}
	return out
}
