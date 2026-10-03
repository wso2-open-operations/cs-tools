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

// Package sources maps each route name to its transform, following the ServiceNow Edge API mappings.
package sources

import (
	"fmt"
	"sort"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/sources/aws"
	"sre-alert-ingestion-service/internal/sources/azure"
	"sre-alert-ingestion-service/internal/sources/datadog"
	"sre-alert-ingestion-service/internal/sources/elasticsearch"
	"sre-alert-ingestion-service/internal/sources/gcp"
	"sre-alert-ingestion-service/internal/sources/icinga"
	"sre-alert-ingestion-service/internal/sources/openobserve"
	"sre-alert-ingestion-service/internal/sources/opensearch"
	"sre-alert-ingestion-service/internal/sources/prometheus"
	"sre-alert-ingestion-service/internal/sources/servicenow"
	"sre-alert-ingestion-service/internal/sources/site24x7"
)

// Transform turns one webhook body into canonical alerts; an error means the payload is rejected with 400.
type Transform func(raw []byte) ([]model.Alert, error)

// Registry holds every source's transform, bound to the config it loaded at startup.
type Registry struct {
	transforms map[string]Transform
}

// New loads every source's <SOURCE>_ALERT_CONFIG, failing startup on a malformed config instead of silently falling back to defaults.
func New() (*Registry, error) {
	awsCfg, err := aws.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("aws: %w", err)
	}
	azureCfg, err := azure.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("azure: %w", err)
	}
	datadogCfg, err := datadog.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("datadog: %w", err)
	}
	esCfg, err := elasticsearch.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: %w", err)
	}
	gcpCfg, err := gcp.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("gcp: %w", err)
	}
	icingaCfg, err := icinga.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("icinga: %w", err)
	}
	ooCfg, err := openobserve.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("openobserve: %w", err)
	}
	osCfg, err := opensearch.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("opensearch: %w", err)
	}
	promCfg, err := prometheus.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("prometheus: %w", err)
	}
	s247Cfg, err := site24x7.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("site24x7: %w", err)
	}

	return &Registry{transforms: map[string]Transform{
		"aws": func(raw []byte) ([]model.Alert, error) {
			a, err := aws.Transform(raw, awsCfg)
			return one(err, model.Alert(a))
		},
		"azure": func(raw []byte) ([]model.Alert, error) {
			a, err := azure.Transform(raw, azureCfg)
			return one(err, model.Alert(a))
		},
		"datadog": func(raw []byte) ([]model.Alert, error) {
			a, err := datadog.Transform(raw, datadogCfg)
			return one(err, model.Alert(a))
		},
		"elasticsearch": func(raw []byte) ([]model.Alert, error) {
			a, err := elasticsearch.Transform(raw, esCfg)
			return one(err, model.Alert(a))
		},
		"gcp": func(raw []byte) ([]model.Alert, error) {
			a, err := gcp.Transform(raw, gcpCfg)
			return one(err, model.Alert(a))
		},
		"icinga": func(raw []byte) ([]model.Alert, error) {
			a, err := icinga.Transform(raw, icingaCfg)
			return one(err, model.Alert(a))
		},
		"openobserve": func(raw []byte) ([]model.Alert, error) {
			a, err := openobserve.Transform(raw, ooCfg)
			return one(err, fromOpenObserve(a))
		},
		"opensearch": func(raw []byte) ([]model.Alert, error) {
			a, err := opensearch.Transform(raw, osCfg)
			return one(err, model.Alert(a))
		},
		"prometheus": func(raw []byte) ([]model.Alert, error) {
			batch, err := prometheus.Transform(raw, promCfg)
			if err != nil {
				return nil, err
			}
			out := make([]model.Alert, len(batch))
			for i, a := range batch {
				out[i] = model.Alert(a)
			}
			return out, nil
		},
		// Temporary: alerts ServiceNow forwards already in canonical form, during the parallel run.
		"servicenow": servicenow.Transform,
		"site24x7": func(raw []byte) ([]model.Alert, error) {
			a, err := site24x7.Transform(raw, s247Cfg)
			return one(err, model.Alert(a))
		},
	}}, nil
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

// fromOpenObserve keeps the eight canonical fields and drops the ServiceNow Incident fields OpenObserve's transform also produces.
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
	}
}
