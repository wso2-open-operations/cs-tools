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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func stdp(s string) *string { return &s }

func wantStdValidation(t *testing.T, err error, contains string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a ValidationError containing %q", err, contains)
	}
	if !strings.Contains(ve.Msg, contains) {
		t.Fatalf("message %q does not contain %q", ve.Msg, contains)
	}
}

func TestNormalizeStandardChangeParameters(t *testing.T) {
	ok, err := normalizeStandardChangeParameters([]domain.StandardChangeParameter{
		{Key: " cluster_name ", Label: " Cluster ", Type: domain.StandardChangeParameterTypeText, Required: true, HelpText: stdp("  ")},
		{Key: "env", Label: "Environment", Type: domain.StandardChangeParameterTypeSelect, Options: []string{" dev ", "prod"}},
	})
	if err != nil {
		t.Fatalf("valid declarations refused: %v", err)
	}
	if ok[0].Key != "cluster_name" || ok[0].Label != "Cluster" || ok[0].HelpText != nil {
		t.Fatalf("not trimmed: %+v", ok[0])
	}
	if !reflect.DeepEqual(ok[1].Options, []string{"dev", "prod"}) {
		t.Fatalf("options not trimmed: %v", ok[1].Options)
	}

	cases := []struct {
		name   string
		params []domain.StandardChangeParameter
		want   string
	}{
		{"bad key", []domain.StandardChangeParameter{{Key: "Cluster", Label: "x", Type: "text"}}, "must start with a lower-case letter"},
		{"key with dash", []domain.StandardChangeParameter{{Key: "a-b", Label: "x", Type: "text"}}, "must start with a lower-case letter"},
		{"duplicate key", []domain.StandardChangeParameter{{Key: "a", Label: "x", Type: "text"}, {Key: "a", Label: "y", Type: "text"}}, "declared twice"},
		{"no label", []domain.StandardChangeParameter{{Key: "a", Label: " ", Type: "text"}}, "needs a label"},
		{"unknown type", []domain.StandardChangeParameter{{Key: "a", Label: "x", Type: "html"}}, "type must be one of"},
		{"select without options", []domain.StandardChangeParameter{{Key: "a", Label: "x", Type: "select"}}, "needs 1 to"},
		{"select duplicate option", []domain.StandardChangeParameter{{Key: "a", Label: "x", Type: "select", Options: []string{"p", " p"}}}, "listed twice"},
		{"options on text", []domain.StandardChangeParameter{{Key: "a", Label: "x", Type: "text", Options: []string{"p"}}}, "only a select takes options"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := normalizeStandardChangeParameters(c.params)
			wantStdValidation(t, err, c.want)
		})
	}

	many := make([]domain.StandardChangeParameter, maxStandardChangeParameters+1)
	for i := range many {
		many[i] = domain.StandardChangeParameter{Key: "k" + strings.Repeat("a", i%5) + string(rune('a'+i%26)), Label: "x", Type: "text"}
	}
	_, err = normalizeStandardChangeParameters(many)
	wantStdValidation(t, err, "at most 30 parameters")
}

func completeProposal() domain.StandardChangeProposal {
	low := domain.ChangeRequestRiskLow
	impact := domain.ChangeRequestImpactLow
	return domain.StandardChangeProposal{
		Kind:      domain.StandardChangeProposalKindNew,
		Name:      "Update AKS cluster tags",
		Rationale: stdp("<p>Done 12 times without incident.</p>"),
		StandardChangeContent: domain.StandardChangeContent{
			Subject:            stdp("Update tags on {{cluster}}"),
			ImplementationPlan: stdp("<p>az aks update -n {{cluster}} --tags {{tags}}</p>"),
			BackoutPlan:        stdp("<p>Re-apply the previous tags of {{cluster}}.</p>"),
			TestPlan:           stdp("<p>az aks show -n {{ cluster }}</p>"),
			Risk:               &low,
			Impact:             &impact,
			Parameters: []domain.StandardChangeParameter{
				{Key: "cluster", Label: "Cluster", Type: "text", Required: true},
				{Key: "tags", Label: "Tags", Type: "textarea", Required: true},
			},
		},
	}
}

func TestCheckStandardChangeProposalComplete(t *testing.T) {
	if err := checkStandardChangeProposalComplete(completeProposal()); err != nil {
		t.Fatalf("complete proposal refused: %v", err)
	}

	p := completeProposal()
	p.Rationale, p.TestPlan, p.Risk, p.Subject = stdp("<p><br></p>"), stdp("<p>&nbsp;</p>"), nil, stdp("  ")
	wantStdValidation(t, checkStandardChangeProposalComplete(p), "still needs: rationale, subject, testPlan, risk")

	p = completeProposal()
	p.RollbackDurationText = stdp(strings.Repeat("x", 256))
	wantStdValidation(t, checkStandardChangeProposalComplete(p), "rollbackDurationText must be at most 255")

	p = completeProposal()
	high := domain.ChangeRequestRiskHigh
	p.Risk = &high
	wantStdValidation(t, checkStandardChangeProposalComplete(p), "cannot be high risk")

	p = completeProposal()
	p.Description = stdp("<p>Owner {{owner}} and {{cluster}}</p>")
	wantStdValidation(t, checkStandardChangeProposalComplete(p), "does not declare: {{owner}}")

	p = completeProposal()
	p.Parameters = append(p.Parameters, domain.StandardChangeParameter{Key: "cluster", Label: "Again", Type: "text"})
	wantStdValidation(t, checkStandardChangeProposalComplete(p), "declared twice")

	// A declared parameter the text does not use is allowed: it records a
	// specific of the change without appearing in the plan.
	p = completeProposal()
	p.Parameters = append(p.Parameters, domain.StandardChangeParameter{Key: "ticket", Label: "Ticket", Type: "text"})
	if err := checkStandardChangeProposalComplete(p); err != nil {
		t.Fatalf("unused parameter refused: %v", err)
	}

	// A retirement needs its rationale and nothing of the content.
	r := domain.StandardChangeProposal{Kind: domain.StandardChangeProposalKindRetire, Name: "x", Rationale: stdp("no longer used")}
	if err := checkStandardChangeProposalComplete(r); err != nil {
		t.Fatalf("retirement refused: %v", err)
	}
	r.Rationale = nil
	wantStdValidation(t, checkStandardChangeProposalComplete(r), "still needs: rationale")
}

func TestStandardChangePlaceholders(t *testing.T) {
	c := domain.StandardChangeContent{
		Subject:            stdp("{{b}} on {{ a }}"),
		ImplementationPlan: stdp("{{a}} {{Not_A_Key}} {{c}} {{ }} {c}"),
	}
	if got := standardChangePlaceholders(c); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("placeholders = %v", got)
	}
}

func version() domain.StandardChangeTemplateVersion {
	p := completeProposal()
	p.Description = stdp("<p>Requested: {{reason}} by {{owner}}</p>")
	p.Parameters = append(p.Parameters,
		domain.StandardChangeParameter{Key: "reason", Label: "Reason", Type: "textarea"},
		domain.StandardChangeParameter{Key: "owner", Label: "Owner", Type: "select", Options: []string{"team-a", "team-b"}},
		domain.StandardChangeParameter{Key: "count", Label: "Count", Type: "number"},
		domain.StandardChangeParameter{Key: "on", Label: "On", Type: "date"},
	)
	return domain.StandardChangeTemplateVersion{StandardChangeContent: p.StandardChangeContent}
}

func TestRenderStandardChange(t *testing.T) {
	out, stored, err := renderStandardChange(version(), map[string]string{
		"cluster": "  aks-<prod>  ",
		"tags":    "env=prod\nowner=sre",
		"reason":  "a & b\r\nc",
		"owner":   "team-b",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := *out.Subject; got != "Update tags on aks-<prod>" {
		t.Errorf("subject (plain text, raw) = %q", got)
	}
	if got := *out.ImplementationPlan; got != "<p>az aks update -n aks-&lt;prod&gt; --tags env=prod<br>owner=sre</p>" {
		t.Errorf("plan (escaped, textarea line breaks) = %q", got)
	}
	if got := *out.TestPlan; got != "<p>az aks show -n aks-&lt;prod&gt;</p>" {
		t.Errorf("spaces inside the braces = %q", got)
	}
	if got := *out.Description; got != "<p>Requested: a &amp; b<br>c by team-b</p>" {
		t.Errorf("description = %q", got)
	}
	want := map[string]string{"cluster": "aks-<prod>", "tags": "env=prod\nowner=sre", "reason": "a & b\r\nc", "owner": "team-b", "count": "", "on": ""}
	if !reflect.DeepEqual(stored, want) {
		t.Errorf("stored = %v", stored)
	}

	// The version itself is not changed by rendering.
	v := version()
	if _, _, err := renderStandardChange(v, map[string]string{"cluster": "x", "tags": "y"}); err != nil {
		t.Fatal(err)
	}
	if *v.Subject != "Update tags on {{cluster}}" {
		t.Errorf("version mutated: %q", *v.Subject)
	}

	refusals := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"required missing", map[string]string{"tags": "x"}, "Cluster (cluster) is required"},
		{"required blank", map[string]string{"cluster": "  ", "tags": "x"}, "is required"},
		{"undeclared value", map[string]string{"cluster": "x", "tags": "y", "zone": "z"}, "does not declare: zone"},
		{"text with newline", map[string]string{"cluster": "a\nb", "tags": "y"}, "single line"},
		{"select not an option", map[string]string{"cluster": "x", "tags": "y", "owner": "team-c"}, "must be one of: team-a, team-b"},
		{"number", map[string]string{"cluster": "x", "tags": "y", "count": "three"}, "must be a number"},
		{"number NaN", map[string]string{"cluster": "x", "tags": "y", "count": "NaN"}, "must be a number"},
		{"number Inf", map[string]string{"cluster": "x", "tags": "y", "count": "-Inf"}, "must be a number"},
		{"date", map[string]string{"cluster": "x", "tags": "y", "on": "28/10/2026"}, "must be a date"},
		{"text too long", map[string]string{"cluster": strings.Repeat("x", 501), "tags": "y"}, "at most 500"},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := renderStandardChange(version(), c.values)
			wantStdValidation(t, err, c.want)
		})
	}

	rb := version()
	rb.RollbackDurationText = stdp("Restore within {{reason}}")
	_, _, err = renderStandardChange(rb, map[string]string{"cluster": "x", "tags": "y", "reason": strings.Repeat("r", 300)})
	wantStdValidation(t, err, "rollback duration this template makes")

	long := version()
	long.Subject = stdp("{{reason}}")
	_, _, err = renderStandardChange(long, map[string]string{"cluster": "x", "tags": "y", "reason": strings.Repeat("r ", 300)})
	wantStdValidation(t, err, "longer than 512")
}
