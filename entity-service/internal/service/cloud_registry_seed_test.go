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
	"errors"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The seven clouds that existed before the registry became data, kept here as
// the golden fixture: the availability and routing tests assert against the
// same plans the code used to hold, so a change to how the registry is read
// shows up as a failing expectation. The data itself is loaded per environment
// by a seed step kept outside the repository; this fixture is the reference it
// was written from.

// seededCloudOfferings maps each seeded slug to its stored cloud_offering value.
var seededCloudOfferings = map[string]string{
	"asgardeo":      "ASGARDEO",
	"choreo":        "CHOREO",
	"bijira":        "BIJIRA",
	"devant":        "DEVANT",
	"moesif":        "MOESIF",
	"choreo-eu":     "CHOREO_EU",
	"agent-manager": "AGENT_MANAGER",
}

// seededCloudOrder is the order the registry returns them (sort_order).
var seededCloudOrder = []string{"asgardeo", "choreo", "bijira", "devant", "moesif", "choreo-eu", "agent-manager"}

var seededCloudPlans = map[string]domain.CloudAvailabilityPlan{
	"asgardeo": {
		Regions: []domain.AvailabilityRegion{
			{Key: "us", ParentID: "492c649b-471e-f510-a0a2-9cd3846d43e3"},
			{Key: "eu", ParentID: "40ae701b-479e-f510-a0a2-9cd3846d43cb"},
		},
		// asgardeo alone dispatches to the unweighted `calculate`.
		Weights: nil,
	},
	"choreo": {
		Regions: []domain.AvailabilityRegion{
			{Key: "cp", ParentID: "aedc9983-47e4-8e90-a0a2-9cd3846d43e5"},
			{Key: "us - dp", ParentID: "550d9d83-47e4-8e90-a0a2-9cd3846d43ef"},
			{Key: "eu - dp", ParentID: "5e1dd983-47e4-8e90-a0a2-9cd3846d4317"},
		},
		// 26 offerings across 3 region(s); sums to 300 = 3 x 100.
		Weights: map[string]float64{
			"ea5dd5c3-47e4-8e90-a0a2-9cd3846d43cf": 10,
			"1d0e9907-47e4-8e90-a0a2-9cd3846d430e": 10,
			"b85e1147-47e4-8e90-a0a2-9cd3846d4371": 10,
			"d78e5947-47e4-8e90-a0a2-9cd3846d435d": 10,
			"2bbe9187-47e4-8e90-a0a2-9cd3846d43dd": 10,
			"52ee1587-47e4-8e90-a0a2-9cd3846d4331": 10,
			"271f9d87-47e4-8e90-a0a2-9cd3846d432b": 25,
			"2e4fd1c7-47e4-8e90-a0a2-9cd3846d43f9": 15,
			"41ef950b-47e4-8e90-a0a2-9cd3846d433d": 10,
			"de10214b-47e4-8e90-a0a2-9cd3846d43d4": 10,
			"9a81a9cb-47e4-8e90-a0a2-9cd3846d4333": 10,
			"56b1654b-47e4-8e90-a0a2-9cd3846d433d": 13.75,
			"61e1a50f-47e4-8e90-a0a2-9cd3846d430f": 13.75,
			"9912e90f-47e4-8e90-a0a2-9cd3846d43cc": 13.75,
			"231d89d2-1b5d-4210-609e-8666624bcb56": 13.75,
			"0e42e90f-47e4-8e90-a0a2-9cd3846d43ed": 7.5,
			"0792e94f-47e4-8e90-a0a2-9cd3846d43b2": 7.5,
			"e3dc694f-4728-8e90-a0a2-9cd3846d43e9": 10,
			"8ebce54f-4728-8e90-a0a2-9cd3846d43c2": 10,
			"435ca1cb-4728-8e90-a0a2-9cd3846d43b7": 10,
			"c03c618b-4728-8e90-a0a2-9cd3846d433a": 13.75,
			"800bad8b-4728-8e90-a0a2-9cd3846d4368": 13.75,
			"34da2547-4728-8e90-a0a2-9cd3846d4308": 13.75,
			"3d4fcbbd-4751-8a10-a0a2-9cd3846d43f7": 13.75,
			"cfaae18b-4728-8e90-a0a2-9cd3846d43dd": 7.5,
			"9d8a2547-4728-8e90-a0a2-9cd3846d4311": 7.5,
		},
	},
	"choreo-eu": {
		Regions: []domain.AvailabilityRegion{
			{Key: "eu - cp", ParentID: "cfede2d7-1b38-3e90-0bb3-da47b04bcbae"},
			{Key: "eu - dp - eu", ParentID: "246f6e5b-1b38-3e90-0bb3-da47b04bcb45"},
		},
		// 17 offerings across 2 region(s); sums to 200 = 2 x 100.
		Weights: map[string]float64{
			"bc6f59eb-1bf8-7e90-0bb3-da47b04bcb08": 10,
			"987b99a3-1bf8-7e90-0bb3-da47b04bcb14": 10,
			"95feddab-1bf8-7e90-0bb3-da47b04bcbbb": 10,
			"293d112b-1bf8-7e90-0bb3-da47b04bcb02": 10,
			"10181528-1b01-f6d0-a002-c9d3604bcba7": 10,
			"537c5d67-1bf8-7e90-0bb3-da47b04bcba4": 10,
			"a28e15ab-1bf8-7e90-0bb3-da47b04bcb4f": 25,
			"b5dc51e7-1bf8-7e90-0bb3-da47b04bcbd7": 15,
			"eebb7daf-1bbc-7e90-0bb3-da47b04bcb05": 13.75,
			"0fad6924-1bc1-f6d0-a002-c9d3604bcb8c": 10,
			"cdac25a0-1bc1-f6d0-a002-c9d3604bcb7e": 7.5,
			"f76de524-1bc1-f6d0-a002-c9d3604bcb1d": 13.75,
			"52f9216c-1b81-f6d0-a002-c9d3604bcbac": 13.75,
			"f95131e8-1bc1-f6d0-a002-c9d3604bcb5e": 10,
			"3589a52c-1b81-f6d0-a002-c9d3604bcba8": 13.75,
			"2e65eda0-1b81-f6d0-a002-c9d3604bcb3e": 10,
			"430da5e0-1bc1-f6d0-a002-c9d3604bcb63": 7.5,
		},
	},
	"bijira": {
		Regions: []domain.AvailabilityRegion{
			{Key: "cp - us", ParentID: "b02a0119-1bca-e210-a002-c9d3604bcb1a"},
			{Key: "dp - us", ParentID: "d716af3b-1b03-a210-a002-c9d3604bcb5d"},
			{Key: "cp - eu", ParentID: "a0c22ca0-1b4c-be10-182c-0dc5604bcbb6"},
			{Key: "dp - eu", ParentID: "3133e8e0-1b4c-be10-182c-0dc5604bcb66"},
		},
		// 30 offerings across 4 region(s); sums to 400 = 4 x 100.
		Weights: map[string]float64{
			"db9efeb3-1b8f-6210-0bb3-da47b04bcb21": 4.5,
			"1a7df673-1b8f-6210-0bb3-da47b04bcb63": 30,
			"116ef673-1b8f-6210-0bb3-da47b04bcb8f": 10,
			"c0de3ef3-1b8f-6210-0bb3-da47b04bcbd0": 10.5,
			"fd2e3ab3-1b8f-6210-0bb3-da47b04bcb4c": 15,
			"6a06b2b3-1b4f-6210-0bb3-da47b04bcbbf": 10,
			"b0ed7e73-1b8f-6210-0bb3-da47b04bcb2b": 20,
			"ada7e7bf-1b03-a210-a002-c9d3604bcbc9": 30,
			"3b56effb-1b03-a210-a002-c9d3604bcb53": 7.5,
			"aec66b3f-1b03-a210-a002-c9d3604bcbd7": 7.5,
			"3627e77f-1b03-a210-a002-c9d3604bcbd1": 5,
			"4678e333-1b43-a210-a002-c9d3604bcbf6": 5,
			"5018ebbf-1b03-a210-a002-c9d3604bcbcd": 7.5,
			"64482fff-1b03-a210-a002-c9d3604bcb64": 7.5,
			"4bd76bbf-1b03-a210-a002-c9d3604bcb72": 30,
			"54b7a8a8-1b4c-be10-182c-0dc5604bcb43": 4.5,
			"3b04116c-1b40-fe10-182c-0dc5604bcb53": 30,
			"ae3515ec-1b40-fe10-182c-0dc5604bcbba": 10,
			"f5a499ac-1b40-fe10-182c-0dc5604bcb59": 10.5,
			"85f4d5ac-1b40-fe10-182c-0dc5604bcb6a": 15,
			"b2341d6c-1b40-fe10-182c-0dc5604bcb2f": 10,
			"ffb5d520-1b80-fe10-182c-0dc5604bcb36": 20,
			"53265160-1b80-fe10-182c-0dc5604bcbc1": 30,
			"58865d20-1b80-fe10-182c-0dc5604bcbb6": 7.5,
			"76c6d1a0-1b80-fe10-182c-0dc5604bcb34": 7.5,
			"df67d1e0-1b80-fe10-182c-0dc5604bcb36": 5,
			"7b175da0-1b80-fe10-182c-0dc5604bcb11": 5,
			"25b7d9e0-1b80-fe10-182c-0dc5604bcb07": 7.5,
			"0df79de0-1b80-fe10-182c-0dc5604bcba2": 7.5,
			"52289524-1b80-fe10-182c-0dc5604bcbd7": 30,
		},
	},
	"moesif": {
		Regions: []domain.AvailabilityRegion{
			{Key: "us", ParentID: "9458cdd5-1bdf-b6d0-a002-c9d3604bcbf3"},
		},
		// 11 offerings across 1 region(s); sums to 100 = 1 x 100.
		Weights: map[string]float64{
			"6e45d951-1b93-f6d0-a002-c9d3604bcb02": 30,
			"3e18cb4b-1b1b-7a14-a002-c9d3604bcb63": 2.5,
			"5243d3c7-1b9b-7a14-a002-c9d3604bcbb5": 2.5,
			"2da35b4b-1b9b-7a14-a002-c9d3604bcb48": 15,
			"8668c7c7-1b1b-7a14-a002-c9d3604bcb08": 15,
			"2f884f4b-1b1b-7a14-a002-c9d3604bcb3f": 10,
			"47d3934b-1b9b-7a14-a002-c9d3604bcb8e": 5,
			"b4f35f0b-1b9b-7a14-a002-c9d3604bcb03": 5,
			"cb04db4b-1b9b-7a14-a002-c9d3604bcbed": 5,
			"78d84b4b-1b1b-7a14-a002-c9d3604bcb93": 5,
			"9af80b0f-1b1b-7a14-a002-c9d3604bcb1a": 5,
		},
	},
	"devant": {
		Regions: []domain.AvailabilityRegion{
			{Key: "cp - us", ParentID: "bf0e1e82-1bcc-be50-0bb3-da47b04bcb9a"},
			{Key: "dp - us", ParentID: "f55e16c2-1bcc-be50-0bb3-da47b04bcb40"},
			{Key: "cp - eu", ParentID: "62d30e53-1b00-b290-a002-c9d3604bcbcc"},
			{Key: "dp - eu", ParentID: "815402d3-1b00-b290-a002-c9d3604bcb42"},
		},
		// 36 offerings across 4 region(s); sums to 400 = 4 x 100.
		Weights: map[string]float64{
			"8d5fda82-1bcc-be50-0bb3-da47b04bcb9e": 10,
			"482842d7-1b00-b290-a002-c9d3604bcb92": 12.5,
			"243b4e5f-1b00-b290-a002-c9d3604bcbca": 12.5,
			"9d5ec653-1b40-b290-a002-c9d3604bcb6b": 10,
			"f96f4ad3-1b40-b290-a002-c9d3604bcb0e": 7.5,
			"69ef0217-1b40-b290-a002-c9d3604bcb7f": 7.5,
			"4b709a57-1b40-b290-a002-c9d3604bcb1a": 25,
			"a9fe4ad3-1b40-b290-a002-c9d3604bcb07": 15,
			"b31122d3-1bc0-b290-a002-c9d3604bcbb8": 10,
			"07f22a17-1bc0-b290-a002-c9d3604bcbc8": 10,
			"dec32697-1bc0-b290-a002-c9d3604bcb27": 5,
			"3354221b-1bc0-b290-a002-c9d3604bcb18": 12,
			"4df4ee1b-1bc0-b290-a002-c9d3604bcb75": 12,
			"1a65e2d7-1bc0-b290-a002-c9d3604bcba1": 12,
			"3fa5ee5b-1bc0-b290-a002-c9d3604bcb95": 12,
			"79386a9b-1bc0-b290-a002-c9d3604bcb46": 12,
			"14692213-1b04-b290-a002-c9d3604bcbb6": 7.5,
			"c2c96653-1b04-b290-a002-c9d3604bcb88": 7.5,
			"12151a9f-1b40-b290-a002-c9d3604bcb95": 10,
			"42e55adf-1b40-b290-a002-c9d3604bcbe6": 12.5,
			"51111297-1b40-b290-a002-c9d3604bcb85": 12.5,
			"13e69693-1b80-b290-a002-c9d3604bcb0f": 10,
			"64ab9e5b-1b80-b290-a002-c9d3604bcb48": 7.5,
			"fe0cdad7-1b80-b290-a002-c9d3604bcb0e": 7.5,
			"559c92db-1b80-b290-a002-c9d3604bcb21": 25,
			"78665653-1b80-b290-a002-c9d3604bcbcd": 15,
			"7e9e221f-1b04-b290-a002-c9d3604bcb86": 10,
			"072af25b-1b44-b290-a002-c9d3604bcb95": 10,
			"5b9ca297-1b04-b290-a002-c9d3604bcb1d": 5,
			"db0c2a57-1b04-b290-a002-c9d3604bcbad": 12,
			"a25b2ed3-1b04-b290-a002-c9d3604bcb3a": 12,
			"251e2a9b-1b04-b290-a002-c9d3604bcb0c": 12,
			"800d22d7-1b04-b290-a002-c9d3604bcb2d": 12,
			"488deed7-1b04-b290-a002-c9d3604bcb7d": 12,
			"eeaa72df-1b44-b290-a002-c9d3604bcbe2": 7.5,
			"853b7a13-1b84-b290-a002-c9d3604bcb4d": 7.5,
		},
	},
	"agent-manager": {
		Regions: []domain.AvailabilityRegion{
			{Key: "cp - us", ParentID: "1cf18e12-3b5b-8710-3e1e-088aa4e45ac0"},
			{Key: "dp - us", ParentID: "a7b24ed2-3b5b-8710-3e1e-088aa4e45a17"},
		},
		// 14 offerings across 2 region(s); sums to 200 = 2 x 100.
		Weights: map[string]float64{
			"91d606d2-3b9b-8710-3e1e-088aa4e45a69": 20,
			"c07f392a-3b53-8b10-3e1e-088aa4e45a69": 10,
			"d750022e-3b53-8b10-3e1e-088aa4e45a1a": 20,
			"1400caaa-3b53-8b10-3e1e-088aa4e45af1": 10,
			"03f086ae-3b53-8b10-3e1e-088aa4e45af9": 10,
			"d3314aee-3b53-8b10-3e1e-088aa4e45aef": 5,
			"cc810e22-3b93-8b10-3e1e-088aa4e45a4b": 5,
			"96b14e62-3b93-8b10-3e1e-088aa4e45a54": 5,
			"2f388296-3b9b-8710-3e1e-088aa4e45a15": 5,
			"3a22cae2-3b93-8b10-3e1e-088aa4e45ad6": 5,
			"09720e26-3b93-8b10-3e1e-088aa4e45acb": 5,
			"69980a5a-3b9b-8710-3e1e-088aa4e45aad": 30,
			"6ed84a9a-3b9b-8710-3e1e-088aa4e45aa1": 55,
			"a8194ada-3b9b-8710-3e1e-088aa4e45a8b": 15,
		},
	},
}

// seededDefinitions builds the registry rows the migration seeds.
func seededDefinitions() []domain.CloudDefinition {
	defs := make([]domain.CloudDefinition, 0, len(seededCloudOrder))
	for _, slug := range seededCloudOrder {
		defs = append(defs, domain.CloudDefinition{
			Slug:           slug,
			DisplayName:    slug,
			OfferingKey:    seededCloudOfferings[slug],
			IncidentPrefix: slug,
			Plan:           seededCloudPlans[slug],
		})
	}
	return defs
}

// fixtureClouds is a registry loader double. defs is what the "database"
// holds; err, when set, makes the read fail. calls counts reads.
type fixtureClouds struct {
	defs  []domain.CloudDefinition
	err   error
	calls int
}

func (f *fixtureClouds) Clouds(context.Context) ([]domain.CloudDefinition, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.defs, nil
}

var errFixtureDB = errors.New("fixture: database unavailable")

// Clouds on each fake: the seven seeded clouds unless a test installs its own
// registry double.
func (f *fakeDashboardRepo) Clouds(ctx context.Context) ([]domain.CloudDefinition, error) {
	if f.registry != nil {
		return f.registry.Clouds(ctx)
	}
	return seededDefinitions(), nil
}

func (f *replayDashboardRepo) Clouds(context.Context) ([]domain.CloudDefinition, error) {
	return seededDefinitions(), nil
}

func (f *fakeCloudStatusRepo) Clouds(ctx context.Context) ([]domain.CloudDefinition, error) {
	if f.registry != nil {
		return f.registry.Clouds(ctx)
	}
	return seededDefinitions(), nil
}
