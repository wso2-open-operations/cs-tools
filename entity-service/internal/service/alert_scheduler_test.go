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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func filled(n int, prefix string) chan string {
	ch := make(chan string, n)
	for i := range n {
		ch <- prefix + string(rune('a'+i))
	}
	return ch
}

func drainPicks(t *testing.T, s *scheduler[string], n int) []domain.AlertTier {
	t.Helper()
	tiers := make([]domain.AlertTier, 0, n)
	for range n {
		_, tier, ok := s.next(context.Background())
		if !ok {
			t.Fatalf("next returned no item after %d picks", len(tiers))
		}
		tiers = append(tiers, tier)
	}
	return tiers
}

func TestScheduler_HighGoesFirstWhileStandardIsEmpty(t *testing.T) {
	s := newScheduler(filled(10, "h"), make(chan string), 4)
	for i, tier := range drainPicks(t, s, 10) {
		if tier != domain.AlertTierPriority {
			t.Fatalf("pick %d: got %s, want priority", i, tier)
		}
	}
}

func TestScheduler_StandardGetsOneTurnAfterWeightHighPicks(t *testing.T) {
	s := newScheduler(filled(12, "h"), filled(3, "s"), 4)
	got := drainPicks(t, s, 15)
	want := []domain.AlertTier{"priority", "priority", "priority", "priority", "standard", "priority", "priority", "priority", "priority", "standard", "priority", "priority", "priority", "priority", "standard"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pick order = %v, want %v", got, want)
		}
	}
}

func TestScheduler_StandardRunsFreelyWhenHighIsIdle(t *testing.T) {
	s := newScheduler(make(chan string), filled(6, "s"), 4)
	for i, tier := range drainPicks(t, s, 6) {
		if tier != domain.AlertTierStandard {
			t.Fatalf("pick %d: got %s, want default", i, tier)
		}
	}
}

func TestScheduler_HighArrivingLaterJumpsWaitingStandard(t *testing.T) {
	priority := make(chan string, 1)
	s := newScheduler(priority, filled(5, "s"), 4)
	if _, tier, _ := s.next(context.Background()); tier != domain.AlertTierStandard {
		t.Fatalf("first pick = %s, want default", tier)
	}
	priority <- "urgent"
	if item, tier, _ := s.next(context.Background()); tier != domain.AlertTierPriority || item != "urgent" {
		t.Fatalf("second pick = %s %q, want the priority item that just arrived", tier, item)
	}
}

func TestScheduler_BlocksUntilAnItemArrives(t *testing.T) {
	standard := make(chan string, 1)
	s := newScheduler(make(chan string), standard, 4)
	go func() {
		time.Sleep(20 * time.Millisecond)
		standard <- "late"
	}()
	item, tier, ok := s.next(context.Background())
	if !ok || item != "late" || tier != domain.AlertTierStandard {
		t.Fatalf("next = %q %s %v, want the late standard item", item, tier, ok)
	}
}

func TestScheduler_StopsWhenContextIsDone(t *testing.T) {
	s := newScheduler(make(chan string), make(chan string), 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, ok := s.next(ctx); ok {
		t.Fatal("next returned an item from empty lanes after the context was cancelled")
	}
}

// Weight 0 is strict priority: standard waits until no priority item is left.
func TestScheduler_WeightZeroIsStrictPriority(t *testing.T) {
	s := newScheduler(filled(6, "h"), filled(2, "s"), 0)
	got := drainPicks(t, s, 8)
	want := []domain.AlertTier{"priority", "priority", "priority", "priority", "priority", "priority", "standard", "standard"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pick order = %v, want %v", got, want)
		}
	}
}

func TestScheduler_NegativeWeightIsStrictPriority(t *testing.T) {
	s := newScheduler(filled(3, "h"), filled(1, "s"), -2)
	got := drainPicks(t, s, 4)
	want := []domain.AlertTier{"priority", "priority", "priority", "standard"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pick order = %v, want %v", got, want)
		}
	}
}

func TestScheduler_StopsWhenContextIsDoneEvenWithWaitingItems(t *testing.T) {
	priority, standard := filled(3, "h"), filled(3, "s")
	s := newScheduler(priority, standard, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, ok := s.next(ctx); ok {
		t.Fatal("next handed out an item after the context was cancelled")
	}
	if len(priority) != 3 || len(standard) != 3 {
		t.Fatalf("lanes hold %d and %d items, want both untouched so shutdown can hand them back", len(priority), len(standard))
	}
}
