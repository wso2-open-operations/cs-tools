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

package sweep

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/apierror"
)

// CSMWriter writes a CSM project. Satisfied by *entity.Client.
type CSMWriter interface {
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

// SalesforceStatusWriter sets a Salesforce project's State__c. Satisfied by
// *salesentity.Client.
type SalesforceStatusWriter interface {
	UpdateProjectStatus(ctx context.Context, id, status string) error
}

// Writer is the sweep's real projectUpdater: CSM writes go to csm, and a
// suspension's State__c goes to Salesforce through sf.
type Writer struct {
	csm CSMWriter
	sf  SalesforceStatusWriter
}

// NewWriter builds a Writer.
func NewWriter(csm CSMWriter, sf SalesforceStatusWriter) *Writer {
	return &Writer{csm: csm, sf: sf}
}

// UpdateProject writes the CSM project id. A failure that every other
// project's write would hit too is a *writesFailingError, which stops the
// run (see evaluate).
func (w *Writer) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	resp, err := w.csm.UpdateProject(ctx, id, body)
	if err != nil && failsEveryWrite(err) {
		return nil, &writesFailingError{err: err}
	}
	return resp, err
}

// writesFailingError is a CSM write failure that is not about the project:
// the write was refused (401/403: this component's client id missing from
// entity-service's M2M_CLIENT_IDS), throttled (429), or CSM is down (5xx, or
// no response). Notices go out before they are recorded, so carrying on
// would send every remaining due notice without recording it, and the next
// run would send them all again.
type writesFailingError struct{ err error }

func (e *writesFailingError) Error() string {
	return "CSM is not taking writes: " + e.err.Error()
}

func (e *writesFailingError) Unwrap() error { return e.err }

// failsEveryWrite reports whether err, from a CSM write, would fail any
// project's write: anything but a 4xx about this request (a 400, 404, 409
// or 422 is the project's own).
func failsEveryWrite(err error) bool {
	var ae *apierror.Error
	if !errors.As(err, &ae) {
		return true
	}
	switch {
	case ae.StatusCode == http.StatusUnauthorized, ae.StatusCode == http.StatusForbidden,
		ae.StatusCode == http.StatusTooManyRequests, ae.StatusCode >= 500:
		return true
	}
	return false
}

// SuspendInSalesforce sets State__c to Suspended on the Salesforce project
// sfID. A blank id is an error: the write has nowhere to go.
func (w *Writer) SuspendInSalesforce(ctx context.Context, sfID string) error {
	sfID = strings.TrimSpace(sfID)
	if sfID == "" {
		return errors.New("project has no Salesforce id")
	}
	return w.sf.UpdateProjectStatus(ctx, sfID, "Suspended")
}
