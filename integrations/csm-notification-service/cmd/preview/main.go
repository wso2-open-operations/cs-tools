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

// Command preview is a DEV-ONLY tool that renders a notification email to an
// HTML file instead of sending it, so the thing a recipient would actually
// receive can be opened in a browser and reviewed.
//
// It closes the last gap in local testing. csm-flow-service's cmd/dryrun runs
// a flow against the real database and writes the event it would publish;
// this reads that event and renders it. Between them, every hop from a row
// changing to an email body can be checked without a broker, a mail
// credential, or a consumer group.
//
//	# in csm-flow-service
//	DATABASE_URL=… go run ./cmd/dryrun -flow cr_approval_notice -out /tmp/notice.json
//
//	# here
//	go run ./cmd/preview /tmp/notice.json -out /tmp/notice.html && open /tmp/notice.html
//
// It sends nothing, reads no credential, and talks to no service. The envelope
// goes through events.Validate first, so a payload the deployed consumer would
// reject is reported here rather than rendering something that could never
// arrive.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

func main() {
	out := flag.String("out", "preview.html", "file to write the rendered email to")
	csmBase := flag.String("csm-base", envOr("CSM_PORTAL_WEB_BASE_URL", "https://csm.example"), "CSM portal base URL for links")
	customerBase := flag.String("customer-base", envOr("CUSTOMER_PORTAL_WEB_BASE_URL", "https://portal.example"), "customer portal base URL for links")
	flag.Parse()

	// Go's flag package stops at the first non-flag argument, so a flag
	// written after the file would be silently ignored — and the failure mode
	// is a file written somewhere the caller did not ask for. Take the
	// positional argument, then parse whatever follows it.
	args := flag.Args()
	var path string
	if len(args) > 0 {
		path = args[0]
		if err := flag.CommandLine.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
	}

	data, err := readInput(path)
	if err != nil {
		fail("read input: %v", err)
	}

	var env events.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		fail("input is not an event envelope: %v", err)
	}
	if !env.Type.IsKnown() {
		fail("unknown event type %q — the deployed consumer would dead-letter this", env.Type)
	}
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		fail("the deployed consumer would reject this event: %v", err)
	}

	links := recipientlinks.New(nil, recipientlinks.Config{
		CSMBaseURL:      strings.TrimRight(*csmBase, "/"),
		CustomerBaseURL: strings.TrimRight(*customerBase, "/"),
	})

	subject, to, body, err := render(env, links)
	if err != nil {
		fail("%v", err)
	}
	if err := os.WriteFile(*out, []byte(body), 0o600); err != nil {
		fail("write %s: %v", *out, err)
	}

	fmt.Printf("type    : %s\n", env.Type)
	fmt.Printf("subject : %s\n", subject)
	fmt.Printf("to      : %s (%d)\n", strings.Join(to, ", "), len(to))
	fmt.Printf("rendered: %s (%d bytes)\n", *out, len(body))
	fmt.Println("\nNothing was sent. Open the file in a browser to see what a recipient would get.")
}

// render dispatches on the envelope's type. Only the types whose rendering is
// worth eyeballing are here; add one as needed rather than mirroring every
// handler in internal/dispatch.
func render(env events.Envelope, links *recipientlinks.Resolver) (subject string, to []string, body string, err error) {
	switch env.Type {
	case events.TypeCRApprovalRequested:
		var p events.CRApprovalRequestedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", nil, "", fmt.Errorf("decode CR approval payload: %w", err)
		}
		return p.Subject, p.Recipients, notifications.RenderCRApprovalRequestedEmail(notifications.CRApprovalEmailData{
			Number:        p.Number,
			State:         p.State,
			Audience:      p.Audience,
			Team:          p.Team,
			GroupName:     p.GroupName,
			RequesterName: p.RequesterName,
			ProjectName:   p.ProjectName,
			Link:          links.ChangeRequestLink(p.Audience, p.ChangeRequestID, p.ProjectID),
		}), nil
	default:
		return "", nil, "", fmt.Errorf("no preview for %q yet — add a case in cmd/preview", env.Type)
	}
}

// readInput reads the envelope from path, or from stdin when path is empty
// or "-".
func readInput(path string) ([]byte, error) {
	if path != "" && path != "-" {
		return os.ReadFile(path)
	}
	return io.ReadAll(os.Stdin)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "preview: "+format+"\n", args...)
	os.Exit(1)
}
