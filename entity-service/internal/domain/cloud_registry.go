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

package domain

import (
	"sort"
	"strings"
)

// The cloud registry.
//
// A cloud product on the public status dashboard is DATA, not code: a row in
// cloud_status_cloud, its regions in cloud_status_cloud_region and its
// per-offering availability weights in cloud_status_offering_weight (migration
// 0282). Onboarding another cloud is a set of INSERTs plus a webhook URL and
// secret in the notification service's environment -- no Go change.
//
// The weights are a product decision, not a derivation: how much each
// component counts toward a region's published uptime. They are stored as
// percentages and every region's weights sum to exactly 100. A cloud that is
// not weighted (Weighted = false) takes a plain mean instead.

// AvailabilityRegion is one region of a cloud, as the dashboard keys it, and
// the parent service whose offerings make it up.
type AvailabilityRegion struct {
	// Key is the literal, lower-case, oddly-spaced key the frontend reads --
	// "us - dp", "eu - dp - eu". Not derived; stored as given.
	Key string
	// ParentID is the service (not offering) whose children are weighted.
	ParentID string
}

// CloudAvailabilityPlan is everything needed to compute one cloud's uptime.
type CloudAvailabilityPlan struct {
	// Regions in display order.
	Regions []AvailabilityRegion
	// Weights maps a service offering to its share of its region, as a
	// percentage. NIL means the cloud is UNWEIGHTED and takes a plain mean.
	Weights map[string]float64
}

// Weighted reports whether this cloud uses a weighted average.
func (p CloudAvailabilityPlan) Weighted() bool { return p.Weights != nil }

// CloudDefinition is one registered, active cloud.
type CloudDefinition struct {
	// Slug is the dashboard's address for the cloud ("choreo-eu") and the
	// value the public endpoints take.
	Slug        string
	DisplayName string
	// OfferingKey is the value stored in cloud_monitor.cloud_offering for this
	// cloud ("CHOREO_EU"). It is a plain string, not a database enum, so a new
	// cloud needs no schema change; the monitor sync writes this exact value.
	OfferingKey string
	// IncidentPrefix is matched, case-insensitively, against the start of the
	// parent service's name to decide which incidents belong to this cloud.
	IncidentPrefix string
	Plan           CloudAvailabilityPlan
}

// CloudCatalog is an immutable snapshot of the active clouds.
type CloudCatalog struct {
	bySlug     map[string]CloudDefinition
	byOffering map[string]string
	slugs      []string
}

// NewCloudCatalog indexes the given active definitions. The caller supplies
// them in display order; Slugs preserves it.
func NewCloudCatalog(defs []CloudDefinition) CloudCatalog {
	c := CloudCatalog{
		bySlug:     make(map[string]CloudDefinition, len(defs)),
		byOffering: make(map[string]string, len(defs)),
		slugs:      make([]string, 0, len(defs)),
	}
	for _, d := range defs {
		c.bySlug[d.Slug] = d
		c.byOffering[d.OfferingKey] = d.Slug
		c.slugs = append(c.slugs, d.Slug)
	}
	return c
}

// Lookup resolves a dashboard slug, ignoring case and surrounding space. ok is
// false for an unknown or inactive cloud.
func (c CloudCatalog) Lookup(slug string) (CloudDefinition, bool) {
	d, ok := c.bySlug[strings.ToLower(strings.TrimSpace(slug))]
	return d, ok
}

// Slugs lists the valid slugs in display order.
func (c CloudCatalog) Slugs() []string { return append([]string(nil), c.slugs...) }

// SortedSlugs lists the valid slugs alphabetically, for stable messages.
func (c CloudCatalog) SortedSlugs() []string {
	out := c.Slugs()
	sort.Strings(out)
	return out
}

// SlugForOffering converts a stored cloud_offering value to the dashboard's
// slug. It returns "" for a value with no active cloud, which callers must
// treat as unroutable rather than posting an empty cloud.
//
// The dashboard routes on the slug, and CHOREO_EU -> "choreo-eu" is exactly
// the kind of transformation that silently produces "choreo_eu" and a webhook
// nobody receives, which is why the pairing is stored rather than derived.
func (c CloudCatalog) SlugForOffering(offeringKey string) string {
	return c.byOffering[offeringKey]
}
