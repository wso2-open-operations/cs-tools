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

package incident

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

// foldNoteKinds are the kinds of work note an alert can add to an incident it folds into.
var foldNoteKinds = map[strategy.Outcome]string{strategy.Duplicate: "Duplicate", strategy.Resolve: "OK"}

// foldNotePrefix is how the alert's fold note begins; it holds the alert id, so a note starting with it belongs to that alert and no other.
func foldNotePrefix(kind, alertID string) string {
	return fmt.Sprintf("%s alert received.<br>Alert: %s<br>", kind, html.EscapeString(alertID))
}

// foldNotePrefixes lists every way the alert's fold note can begin, so a retry finds it whichever kind it was.
func foldNotePrefixes(alertID string) []string {
	return []string{foldNotePrefix("Duplicate", alertID), foldNotePrefix("OK", alertID)}
}

// foldNote is the work note for an alert added to an open incident.
func foldNote(outcome strategy.Outcome, e event.Event) string {
	kind, ok := foldNoteKinds[outcome]
	if !ok {
		kind = "Duplicate"
	}
	return foldNotePrefix(kind, e.AlertID) + fmt.Sprintf("Metric: %s<br>Source: %s",
		html.EscapeString(orNA(e.Alert.MetricName)), html.EscapeString(orNA(e.Alert.Source)))
}

func orNA(v string) string {
	if strings.TrimSpace(v) == "" {
		return "N/A"
	}
	return v
}
