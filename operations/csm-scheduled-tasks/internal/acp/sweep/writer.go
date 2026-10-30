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
	"strings"
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

// UpdateProject writes the CSM project id.
func (w *Writer) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return w.csm.UpdateProject(ctx, id, body)
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
