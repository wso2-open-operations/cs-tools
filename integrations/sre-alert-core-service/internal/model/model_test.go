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

package model

import (
	"strings"
	"testing"
)

func TestBuildCreationNote_RendersAlertAsHTMLTable(t *testing.T) {
	a := Alert{
		Service:          "checkout-svc",
		MetricName:       "HighCPU",
		Severity:         "Critical",
		Category:         "service_interruption",
		Environment:      "Production",
		Source:           "Grafana",
		UniqueIdentifier: "my21312",
		Description:      "HighCPU threshold breached on checkout-svc",
	}
	got := BuildCreationNote("ALT000000020", a)

	if !strings.HasPrefix(got, "<p>Incident auto-created from Alert: ALT000000020</p>") {
		t.Fatalf("expected an HTML intro paragraph naming the alert id, got: %s", got)
	}
	if !strings.Contains(got, "<table") || !strings.Contains(got, "</table>") {
		t.Fatalf("expected an HTML table, got: %s", got)
	}
	// Field order must match the Alert struct's declared/JSON order, not map iteration order.
	order := []string{"service", "metric_name", "severity", "category", "environment", "source", "unique_identifier", "description"}
	last := -1
	for _, field := range order {
		idx := strings.Index(got, ">"+field+"<")
		if idx == -1 {
			t.Fatalf("expected field %q to appear as a table row, got: %s", field, got)
		}
		if idx <= last {
			t.Fatalf("expected field %q to appear after the previous field, got: %s", field, got)
		}
		last = idx
	}
	for _, want := range []string{"checkout-svc", "HighCPU", "Critical", "service_interruption", "Production", "Grafana", "my21312"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected value %q to appear, got: %s", want, got)
		}
	}
}

func TestBuildCreationNote_EscapesHTMLInAlertFields(t *testing.T) {
	a := Alert{Service: "<script>alert(1)</script>", MetricName: "cpu"}
	got := BuildCreationNote("ALT1", a)

	if strings.Contains(got, "<script>") {
		t.Fatalf("expected alert field content to be HTML-escaped, got: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag, got: %s", got)
	}
}

func TestMoreSevere(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"critical", "warning", true},
		{"Major", "minor", true},
		{"warning", "critical", false},
		{"warning", "warning", false},
		{"ok", "warning", false},
		{"bogus", "warning", false},
		{"warning", "", true},
		{"critical", "bogus", false},
	} {
		if got := MoreSevere(c.a, c.b); got != c.want {
			t.Errorf("MoreSevere(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
