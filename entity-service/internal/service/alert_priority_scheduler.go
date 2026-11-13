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
	"sync"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// priorityScheduler gives each free worker a waiting high item first, but after weight high items in a row a waiting standard item, so standard gets at least 1 in weight+1 picks.
type priorityScheduler[T any] struct {
	mu       sync.Mutex
	high     <-chan T
	standard <-chan T
	weight   int
	streak   int
}

func newPriorityScheduler[T any](high, standard <-chan T, weight int) *priorityScheduler[T] {
	if weight < 1 {
		weight = 1
	}
	return &priorityScheduler[T]{high: high, standard: standard, weight: weight}
}

// next blocks until an item is available and returns it with its tier, or returns false once ctx is done.
func (s *priorityScheduler[T]) next(ctx context.Context) (T, domain.AlertTier, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Once shutdown starts, buffered items stay in their lanes so they are handed back rather than processed.
	if ctx.Err() != nil {
		var zero T
		return zero, "", false
	}
	if s.streak >= s.weight {
		select {
		case item := <-s.standard:
			return s.picked(item, domain.AlertTierDefault)
		default:
		}
	}
	select {
	case item := <-s.high:
		return s.picked(item, domain.AlertTierHigh)
	default:
	}
	select {
	case item := <-s.standard:
		return s.picked(item, domain.AlertTierDefault)
	default:
	}

	// Both lanes are empty, so whichever lane delivers first is the right pick.
	select {
	case item := <-s.high:
		return s.picked(item, domain.AlertTierHigh)
	case item := <-s.standard:
		return s.picked(item, domain.AlertTierDefault)
	case <-ctx.Done():
		var zero T
		return zero, "", false
	}
}

func (s *priorityScheduler[T]) picked(item T, tier domain.AlertTier) (T, domain.AlertTier, bool) {
	if tier == domain.AlertTierHigh {
		s.streak++
	} else {
		s.streak = 0
	}
	return item, tier, true
}
