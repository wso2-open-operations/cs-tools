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

// Package alertsource maps each gateway route to its transform, whose output matches alert-ingestion's so fingerprints agree.
package alertsource

import (
	"sort"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/aws"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/azure"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/datadog"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/elasticsearch"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/gcp"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/icinga"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/openobserve"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/opensearch"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/prometheus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/servicenow"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/site24x7"
)

// Transform turns one webhook body into canonical alerts; an error means the payload can never be stored.
type Transform func(raw []byte) ([]model.Alert, error)

// Registry holds every source's transform.
type Registry struct {
	transforms map[string]Transform
}

// New builds every source's transform with its built-in defaults.
func New() *Registry {
	return &Registry{transforms: map[string]Transform{
		"aws": func(raw []byte) ([]model.Alert, error) {
			a, err := aws.Transform(raw)
			return one(err, a)
		},
		"azure": func(raw []byte) ([]model.Alert, error) {
			a, err := azure.Transform(raw)
			return one(err, a)
		},
		"datadog": func(raw []byte) ([]model.Alert, error) {
			a, err := datadog.Transform(raw)
			return one(err, a)
		},
		"elasticsearch": func(raw []byte) ([]model.Alert, error) {
			a, err := elasticsearch.Transform(raw)
			return one(err, a)
		},
		"gcp": func(raw []byte) ([]model.Alert, error) {
			a, err := gcp.Transform(raw)
			return one(err, a)
		},
		"icinga": func(raw []byte) ([]model.Alert, error) {
			a, err := icinga.Transform(raw)
			return one(err, a)
		},
		"openobserve": func(raw []byte) ([]model.Alert, error) {
			a, err := openobserve.Transform(raw)
			return one(err, fromOpenObserve(a))
		},
		"opensearch": func(raw []byte) ([]model.Alert, error) {
			a, err := opensearch.Transform(raw)
			return one(err, a)
		},
		"prometheus": prometheus.Transform,
		"servicenow": servicenow.Transform,
		"site24x7": func(raw []byte) ([]model.Alert, error) {
			a, err := site24x7.Transform(raw)
			return one(err, a)
		},
	}}
}

// Lookup returns the transform for a route name.
func (r *Registry) Lookup(source string) (Transform, bool) {
	t, ok := r.transforms[source]
	return t, ok
}

// Names returns every registered route name, sorted.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.transforms))
	for n := range r.transforms {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func one(err error, a model.Alert) ([]model.Alert, error) {
	if err != nil {
		return nil, err
	}
	return []model.Alert{a}, nil
}

// fromOpenObserve keeps the eight canonical fields and the defaulted list, and drops the ServiceNow Incident fields OpenObserve's transform also produces.
func fromOpenObserve(a openobserve.Alert) model.Alert {
	return model.Alert{
		Service:          a.Service,
		MetricName:       a.MetricName,
		Severity:         a.Severity,
		Category:         a.Category,
		Environment:      a.Environment,
		Source:           a.Source,
		UniqueIdentifier: a.UniqueIdentifier,
		Description:      a.Description,
		Defaulted:        a.Defaulted,
	}
}
