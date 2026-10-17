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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// InstanceIngestionService defines the service operations for instance ingestion.
type InstanceIngestionService interface {
	UpsertInstanceMeta(ctx context.Context, req domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error)
	InsertInstanceInfo(ctx context.Context, req domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error)
	InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error)
	ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error)
	GetProjectSecretKey(ctx context.Context, projectKey string) (domain.ProjectSecretKeyResponse, error)
	GetDeploymentConsumptionSecret(ctx context.Context, deploymentNumber string) (domain.DeploymentConsumptionSecretResponse, error)
}

// Column widths of the usage tables (migration 0054), checked here so an
// oversized value is a 400 naming the field rather than a database error.
const (
	maxNodeIDLen           = 36
	maxProjectKeyLen       = 128
	maxDeploymentNumberLen = 50
	maxHourlyCountTypeLen  = 50
	maxJDKVersionLen       = 64
	maxInfoHashLen         = 128
	maxDailyUsageBatchSize = 5000
)

// ingestionTimeLayout is the zoneless layout product-consumption-tracking's
// payloads use; it is read as UTC, never in the database session's time zone.
const ingestionTimeLayout = "2006-01-02 15:04:05"

type instanceIngestionService struct {
	repo repository.InstanceIngestionRepository
	now  func() time.Time
}

// NewInstanceIngestionService constructs an InstanceIngestionService.
func NewInstanceIngestionService(repo repository.InstanceIngestionRepository) InstanceIngestionService {
	return &instanceIngestionService{repo: repo, now: time.Now}
}

func checkLen(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s must be at most %d characters", field, max)}
	}
	return nil
}

// checkCount keeps a count inside the INTEGER columns it is stored in.
func checkCount(field string, v int) error {
	if v < 0 || v > math.MaxInt32 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s must be between 0 and %d", field, math.MaxInt32)}
	}
	return nil
}

// normalizeNodeID trims and checks a node id.
func normalizeNodeID(field, nodeID string) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return "", &apierror.ValidationError{Msg: field + " is required"}
	}
	return nodeID, checkLen(field, nodeID, maxNodeIDLen)
}

// normalizeNodeMeta trims the meta fields and checks them against their columns.
func normalizeNodeMeta(meta domain.InstanceNodeMeta) (domain.InstanceNodeMeta, error) {
	meta.ProjectKey = strings.TrimSpace(meta.ProjectKey)
	meta.DeploymentNumber = strings.TrimSpace(meta.DeploymentNumber)
	meta.Product = strings.TrimSpace(meta.Product)
	meta.Version = strings.TrimSpace(meta.Version)
	if err := checkLen("projectKey", meta.ProjectKey, maxProjectKeyLen); err != nil {
		return meta, err
	}
	if err := checkLen("deploymentNumber", meta.DeploymentNumber, maxDeploymentNumberLen); err != nil {
		return meta, err
	}
	return meta, nil
}

// parseIngestionTime reads a payload timestamp: RFC 3339 (any zone), or
// YYYY-MM-DD HH:MM:SS read as UTC. Empty means now.
func (s *instanceIngestionService) parseIngestionTime(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return s.now().UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation(ingestionTimeLayout, value, time.UTC); err == nil {
		return t, nil
	}
	return time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("%s must be RFC 3339 or YYYY-MM-DD HH:MM:SS (UTC)", field)}
}

// UpsertInstanceMeta validates and persists deployment node metadata.
func (s *instanceIngestionService) UpsertInstanceMeta(ctx context.Context, req domain.InstanceMetaRequest) (domain.InstanceMetaResponse, error) {
	nodeID, err := normalizeNodeID("nodeId", req.NodeID)
	if err != nil {
		return domain.InstanceMetaResponse{}, err
	}
	meta, err := normalizeNodeMeta(req.InstanceNodeMeta)
	if err != nil {
		return domain.InstanceMetaResponse{}, err
	}

	id, err := s.repo.UpsertDeploymentNode(ctx, nodeID, meta)
	if err != nil {
		return domain.InstanceMetaResponse{}, err
	}
	return domain.InstanceMetaResponse{ID: id}, nil
}

// InsertInstanceInfo validates and persists a host environment snapshot. With
// no hash, the snapshot's own JSON is hashed (SHA-256) so identical snapshots
// still collapse into one row.
func (s *instanceIngestionService) InsertInstanceInfo(ctx context.Context, req domain.InstanceInfoRequest) (domain.InstanceInfoResponse, error) {
	nodeID, err := normalizeNodeID("nodeId", req.NodeID)
	if err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	req.NodeID = nodeID
	if req.InstanceNodeMeta, err = normalizeNodeMeta(req.InstanceNodeMeta); err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	req.JDKVersion = strings.TrimSpace(req.JDKVersion)
	if err := checkLen("jdkVersion", req.JDKVersion, maxJDKVersionLen); err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	if err := checkCount("coreCount", req.CoreCount); err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	req.Hash = strings.TrimSpace(req.Hash)
	if req.Hash == "" {
		b, err := json.Marshal(req.DeploymentInfo)
		if err != nil {
			return domain.InstanceInfoResponse{}, &apierror.ValidationError{Msg: "deploymentInfo must be valid JSON"}
		}
		sum := sha256.Sum256(b)
		req.Hash = hex.EncodeToString(sum[:])
	}
	if err := checkLen("hash", req.Hash, maxInfoHashLen); err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	reportedOn, err := s.parseIngestionTime("createdTime", req.CreatedTime)
	if err != nil {
		return domain.InstanceInfoResponse{}, err
	}

	id, err := s.repo.InsertDeploymentInformation(ctx, req, reportedOn)
	if err != nil {
		return domain.InstanceInfoResponse{}, err
	}
	return domain.InstanceInfoResponse{ID: id}, nil
}

// InsertUsageStats validates and persists hourly & daily usage telemetry.
func (s *instanceIngestionService) InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) (domain.InstanceUsageIngestResponse, error) {
	nodeID, err := normalizeNodeID("nodeId", req.NodeID)
	if err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	req.NodeID = nodeID
	if req.InstanceNodeMeta, err = normalizeNodeMeta(req.InstanceNodeMeta); err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	req.CountType = strings.TrimSpace(req.CountType)
	if req.CountType == "" {
		return domain.InstanceUsageIngestResponse{}, &apierror.ValidationError{Msg: "countType is required"}
	}
	if err := checkLen("countType", req.CountType, maxHourlyCountTypeLen); err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	if err := checkCount("count", req.Count); err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	countedOn, err := s.parseIngestionTime("createdTime", req.CreatedTime)
	if err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}

	if err := s.repo.InsertUsageStats(ctx, req, countedOn); err != nil {
		return domain.InstanceUsageIngestResponse{}, err
	}
	return domain.InstanceUsageIngestResponse{Status: "SUCCESS"}, nil
}

// ImportDailyUsagesBatch validates every entry before anything is written, so a
// malformed upload is refused whole with the index of the bad entry instead of
// being imported in part.
func (s *instanceIngestionService) ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (domain.DailyUsageBatchResponse, error) {
	nodeID, err := normalizeNodeID("nodeId", req.NodeID)
	if err != nil {
		return domain.DailyUsageBatchResponse{}, err
	}
	req.NodeID = nodeID
	if req.InstanceNodeMeta, err = normalizeNodeMeta(req.InstanceNodeMeta); err != nil {
		return domain.DailyUsageBatchResponse{}, err
	}
	req.UserEmail = strings.TrimSpace(req.UserEmail)
	if err := checkLen("userEmail", req.UserEmail, 255); err != nil {
		return domain.DailyUsageBatchResponse{}, err
	}
	if len(req.Usages) > maxDailyUsageBatchSize {
		return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("usages may hold at most %d entries", maxDailyUsageBatchSize)}
	}

	for i := range req.Usages {
		e := &req.Usages[i]
		field := func(name string) string { return fmt.Sprintf("usages[%d].%s", i, name) }
		e.Date = strings.TrimSpace(e.Date)
		if _, err := time.Parse("2006-01-02", e.Date); err != nil {
			return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: field("date") + " must be a date (YYYY-MM-DD)"}
		}
		e.Type = strings.TrimSpace(e.Type)
		if e.Type == "" {
			return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: field("type") + " is required"}
		}
		if err := checkLen(field("type"), e.Type, 100); err != nil {
			return domain.DailyUsageBatchResponse{}, err
		}
		if err := checkCount(field("value"), e.Value); err != nil {
			return domain.DailyUsageBatchResponse{}, err
		}
		e.NodeID = strings.TrimSpace(e.NodeID)
		if err := checkLen(field("nodeId"), e.NodeID, maxNodeIDLen); err != nil {
			return domain.DailyUsageBatchResponse{}, err
		}
		e.DeploymentNumber = strings.TrimSpace(e.DeploymentNumber)
		if err := checkLen(field("deploymentNumber"), e.DeploymentNumber, maxDeploymentNumberLen); err != nil {
			return domain.DailyUsageBatchResponse{}, err
		}
	}

	if len(req.Nodes) > maxDailyUsageBatchSize {
		return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("nodes may hold at most %d entries", maxDailyUsageBatchSize)}
	}
	for i := range req.Nodes {
		n := &req.Nodes[i]
		field := func(name string) string { return fmt.Sprintf("nodes[%d].%s", i, name) }
		n.NodeID = strings.TrimSpace(n.NodeID)
		if n.NodeID == "" {
			return domain.DailyUsageBatchResponse{}, &apierror.ValidationError{Msg: field("nodeId") + " is required"}
		}
		if err := checkLen(field("nodeId"), n.NodeID, maxNodeIDLen); err != nil {
			return domain.DailyUsageBatchResponse{}, err
		}
		n.Product = strings.TrimSpace(n.Product)
		n.Version = strings.TrimSpace(n.Version)
	}

	count, err := s.repo.ImportDailyUsagesBatch(ctx, req)
	if err != nil {
		return domain.DailyUsageBatchResponse{}, err
	}
	return domain.DailyUsageBatchResponse{ImportedCount: count, Status: "SUCCESS"}, nil
}

// GetProjectSecretKey retrieves the primary secret key for HMAC validation.
func (s *instanceIngestionService) GetProjectSecretKey(ctx context.Context, projectKey string) (domain.ProjectSecretKeyResponse, error) {
	trimmed := strings.TrimSpace(projectKey)
	if trimmed == "" {
		return domain.ProjectSecretKeyResponse{}, &apierror.ValidationError{Msg: "project key is required"}
	}

	secretKey, err := s.repo.GetProjectSecretKey(ctx, trimmed)
	if err != nil {
		return domain.ProjectSecretKeyResponse{}, err
	}
	return domain.ProjectSecretKeyResponse{SecretKey: secretKey}, nil
}

// GetDeploymentConsumptionSecret resolves a deployment number to its project and
// the project's primary secret key.
func (s *instanceIngestionService) GetDeploymentConsumptionSecret(ctx context.Context, deploymentNumber string) (domain.DeploymentConsumptionSecretResponse, error) {
	number := strings.TrimSpace(deploymentNumber)
	if number == "" {
		return domain.DeploymentConsumptionSecretResponse{}, &apierror.ValidationError{Msg: "deployment number is required"}
	}
	if err := checkLen("deployment number", number, maxDeploymentNumberLen); err != nil {
		return domain.DeploymentConsumptionSecretResponse{}, err
	}
	return s.repo.GetDeploymentConsumptionSecret(ctx, number)
}
