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
package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// CodeDashboardGenerator turns one chat message into dashboard source code
// for POST /dashboards/generate-code (the code approach, phase 2).
//
// prompt is already trimmed and length-checked by the handler. current is
// the source being edited, as the webapp sent it, or "" for the first
// message. Implementations: sampleCodeDashboardGenerator (no AI integration
// service configured) and serviceCodeDashboardGenerator.
type CodeDashboardGenerator interface {
	Generate(ctx context.Context, prompt, current string) (GeneratedCodeDashboard, error)
}

// GeneratedCodeDashboard is one generator turn: the source to run, the chat
// reply to show and the name the model suggests saving it under ("" for
// none).
type GeneratedCodeDashboard struct {
	Source string
	Reply  string
	Name   string
}

// generatedCodeMaxLength caps a draft source, in characters (runes): one over
// it is refused by the handler before any model call, since it is sent back
// to the model on every follow-up. The AI integration service holds a reply
// to the same cap.
const generatedCodeMaxLength = 60000

// sampleCodeDashboardName is the sample generator's suggested name: its title.
const sampleCodeDashboardName = "My Cases and Escalations"

// sampleCodeDashboardReply is the sample generator's chat message.
const sampleCodeDashboardReply = "Here is a sample code dashboard: your open cases by severity and the latest escalations. " +
	"This is a fixed sample: dashboard generation is not configured."

// sampleCodeDashboardSource is the dashboard the sample generator returns
// for every message (no AI integration service configured). It shows the
// shape generated code takes (the AI integration service's
// code_catalogue.md): a `render(portal, root)` function that asks for its
// data through the fixed portal APIs (`portal.currentUser`,
// `portal.countCases`, `portal.searchCases`) and draws into `root`. It holds
// no data of its own, so it shows live numbers every time it runs.
const sampleCodeDashboardSource = `async function render(portal, root) {
  const me = await portal.currentUser();

  const bySeverity = await portal.countCases({
    filters: [
      { field: "assignedUserId", op: "in", values: [me.id] },
      { field: "state", op: "notIn", values: ["closed"] },
    ],
    groupBy: "severity",
  });

  const escalations = await portal.searchCases({
    filters: [{ field: "escalation", op: "isNotEmpty" }],
    sortBy: { field: "createdOn", order: "desc" },
    limit: 10,
  });

  root.innerHTML = "";

  const title = document.createElement("h2");
  title.textContent = "My Cases and Escalations";
  root.appendChild(title);

  const counts = document.createElement("ul");
  for (const group of bySeverity.groups) {
    const item = document.createElement("li");
    item.textContent = group.label + ": " + group.count;
    counts.appendChild(item);
  }
  root.appendChild(counts);

  const list = document.createElement("ol");
  for (const c of escalations.items) {
    const item = document.createElement("li");
    item.textContent = c.number + " - " + c.subject;
    list.appendChild(item);
  }
  root.appendChild(list);
}
`

// sampleCodeDashboardGenerator answers every message with
// sampleCodeDashboardSource, ignoring the prompt and the draft. It is the
// fallback when no AI integration service is configured, like
// sampleDashboardGenerator.
type sampleCodeDashboardGenerator struct{}

// NewSampleCodeDashboardGenerator returns the sample generator.
func NewSampleCodeDashboardGenerator() CodeDashboardGenerator {
	return sampleCodeDashboardGenerator{}
}

// Generate returns the sample.
func (sampleCodeDashboardGenerator) Generate(context.Context, string, string) (GeneratedCodeDashboard, error) {
	return GeneratedCodeDashboard{Source: sampleCodeDashboardSource, Reply: sampleCodeDashboardReply, Name: sampleCodeDashboardName}, nil
}

// aiCodeDashboardClient is csmai.Client's code approach call.
type aiCodeDashboardClient interface {
	GenerateCode(ctx context.Context, req csmai.CodeRequest) (csmai.CodeResult, error)
}

// serviceCodeDashboardGenerator is the code approach's generator when the
// model call is the AI integration service's. The service builds the prompt
// (the code catalogue, the prompt and the source being edited) and checks the
// reply holds a render function; the service's error codes become this
// package's generate errors as for the dashboard generator
// (serviceGenerateError).
type serviceCodeDashboardGenerator struct {
	client    aiCodeDashboardClient
	teams     []csmai.Team
	playbooks func(context.Context) ([]plgdomain.Playbook, error)
}

// WithCodePlaybooks makes each Generate read the PLG playbooks with list and
// send the model their answer fields, as WithPlaybooks does for the
// dashboard generator; a code dashboard's codes are checked when it runs,
// not here. g is a generator from NewServiceCodeDashboardGenerator; any
// other is returned as it is.
func WithCodePlaybooks(g CodeDashboardGenerator, list func(context.Context) ([]plgdomain.Playbook, error)) CodeDashboardGenerator {
	if s, ok := g.(*serviceCodeDashboardGenerator); ok {
		s.playbooks = list
	}
	return g
}

// NewServiceCodeDashboardGenerator returns the code generator calling the AI
// integration service. teams is the registry; the model gets the teams a
// dashboard can filter by, as the dashboard generator does (promptTeams).
func NewServiceCodeDashboardGenerator(client aiCodeDashboardClient, teams []directory.TeamResult) CodeDashboardGenerator {
	return &serviceCodeDashboardGenerator{client: client, teams: promptTeams(teams)}
}

// Generate makes one service call for one chat message.
func (g *serviceCodeDashboardGenerator) Generate(ctx context.Context, prompt, current string) (GeneratedCodeDashboard, error) {
	// Read on every message, as the dashboard generator does; when they
	// cannot be, the model is sent none.
	var answers []csmai.PlaybookAnswerField
	if g.playbooks != nil {
		playbooks, err := g.playbooks(ctx)
		if err != nil {
			slog.WarnContext(ctx, "dashboard generate-code: PLG playbooks unavailable; sending no playbook answers", "err", err)
		}
		answers = playbookAnswerFields(playbooks)
	}
	res, err := g.client.GenerateCode(ctx, csmai.CodeRequest{Prompt: prompt, Current: current, Teams: g.teams, PlaybookAnswers: answers})
	if err != nil {
		return GeneratedCodeDashboard{}, serviceGenerateError(err)
	}
	slog.InfoContext(ctx, "dashboard generate-code: model replied",
		"inputTokens", res.Usage.InputTokens,
		"cacheCreationInputTokens", res.Usage.CacheCreationInputTokens,
		"cacheReadInputTokens", res.Usage.CacheReadInputTokens,
		"outputTokens", res.Usage.OutputTokens)
	if res.Declined != "" {
		return GeneratedCodeDashboard{}, &generateDeclinedError{Reason: declineMessage(res.Declined)}
	}
	if res.Source == "" {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: no source in the answer", errGenerateUnusable)
	}
	return GeneratedCodeDashboard{Source: res.Source, Reply: res.Reply, Name: res.Name}, nil
}
