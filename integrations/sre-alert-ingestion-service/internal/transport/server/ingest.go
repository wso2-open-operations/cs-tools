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

package server

import (
	"context"
	"net/http"
	"time"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/sources"
)

// Transforms looks up a source's transform; *sources.Registry implements it.
type Transforms interface {
	Lookup(source string) (sources.Transform, bool)
}

// Submitter stores alerts and returns their ids; *allocator.Allocator implements it.
type Submitter interface {
	Submit(ctx context.Context, source, requestID string, alerts []model.Alert) ([]string, error)
}

// SNSConfirmer handles AWS SNS subscription confirmations; *snsconfirm.Handler implements it.
type SNSConfirmer interface {
	HandleIfConfirmation(raw []byte) bool
}

// Ingestor is the Pipeline: transform, then store; a transform error is a 400 and never reaches the Submitter.
type Ingestor struct {
	transforms Transforms
	submitter  Submitter
	// waitTimeout must stay under the server's write timeout so the client gets a 503 instead of a dropped connection.
	waitTimeout time.Duration
	sns         SNSConfirmer
}

// NewIngestor returns a Pipeline over transforms and submitter.
func NewIngestor(transforms Transforms, submitter Submitter, waitTimeout time.Duration) *Ingestor {
	return &Ingestor{transforms: transforms, submitter: submitter, waitTimeout: waitTimeout}
}

// WithSNSConfirmer handles AWS subscription confirmations before the transform, so they answer 200 and never claim an id.
func (in *Ingestor) WithSNSConfirmer(c SNSConfirmer) *Ingestor {
	in.sns = c
	return in
}

// Ingest implements Pipeline.
func (in *Ingestor) Ingest(ctx context.Context, req Request) Result {
	transform, ok := in.transforms.Lookup(req.Source)
	if !ok {
		// The router only routes registered sources; this guards a mismatch between the two.
		return Result{Status: http.StatusBadRequest, Error: "unknown source"}
	}
	if req.Source == "aws" && in.sns != nil && in.sns.HandleIfConfirmation(req.Body) {
		return Result{Status: http.StatusOK}
	}
	alerts, err := transform(req.Body)
	req.Body = nil // not needed while Submit waits
	if err != nil {
		return Result{Status: http.StatusBadRequest, Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, in.waitTimeout)
	defer cancel()
	ids, err := in.submitter.Submit(ctx, req.Source, req.RequestID, alerts)
	if err != nil {
		return Result{Status: http.StatusServiceUnavailable, Error: err.Error()}
	}
	return Result{Status: http.StatusCreated, AltIDs: ids, Alerts: alerts}
}
