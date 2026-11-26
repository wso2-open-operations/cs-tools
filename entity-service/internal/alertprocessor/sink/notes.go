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

package sink

import (
	"fmt"
	"html"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
)

// Work notes are HTML, the format incident work notes are rendered in, and every value is escaped.

// creationNote is the first work note on an incident an alert opened: the alert's fields as a table.
func creationNote(e event.Event) string {
	a := e.Alert
	rows := [][2]string{
		{"service", a.Service},
		{"metric_name", a.MetricName},
		{"severity", a.Severity},
		{"category", a.Category},
		{"environment", a.Environment},
		{"source", a.Source},
		{"unique_identifier", a.UniqueIdentifier},
		{"description", a.Description},
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<p>Incident auto-created from Alert: %s</p><table>", html.EscapeString(e.AlertID))
	for _, r := range rows {
		fmt.Fprintf(&b, "<tr><td><b>%s</b></td><td>%s</td></tr>", r[0], html.EscapeString(orNA(r[1])))
	}
	b.WriteString("</table>")
	return b.String()
}

// noteMarker is the text every work note about an alert carries, by which a retry finds it.
func noteMarker(alertID string) string { return "Alert: " + alertID }

// foldNote is the work note for an alert added to an open incident.
func foldNote(outcome strategy.Outcome, e event.Event) string {
	kind := "Duplicate"
	if outcome == strategy.Resolve {
		kind = "OK"
	}
	return fmt.Sprintf("%s alert received.<br>%s<br>Metric: %s<br>Source: %s",
		kind, html.EscapeString(noteMarker(e.AlertID)), html.EscapeString(orNA(e.Alert.MetricName)), html.EscapeString(orNA(e.Alert.Source)))
}

func orNA(v string) string {
	if strings.TrimSpace(v) == "" {
		return "N/A"
	}
	return v
}
