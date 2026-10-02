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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skip reasons returned with result "skipped".
const (
	AllocationSkipNoEngagementDetails = "no engagement details"
	AllocationSkipNoEngagementID      = "no engagement id"
	AllocationSkipNoLineItem          = "no engagement for line item"
	AllocationSkipAccountNotFound     = "account not found"
	AllocationSkipAmbiguousAccount    = "ambiguous account name"
	AllocationSkipUserNotFound        = "user not found"
)

// firefightingAllocationTypes are the allocation types ServiceNow's processAllocationEvent
// treats as firefighting (Support/Consulting Related Customer Firefighting).
var firefightingAllocationTypes = map[int]bool{76: true, 83: true}

// allocationClearanceStates maps the Finance Entity's clearance status labels to
// engagement_allocation_state_enum (same labels as csm-sync's mapping).
var allocationClearanceStates = map[string]string{
	"Tentative":                "TENTATIVE",
	"Ready for Clearance":      "READY_FOR_CLEARANCE",
	"Confirmed":                "CONFIRMED",
	"Clearance In Progress":    "CLEARANCE_IN_PROGRESS",
	"Allocation Cancelled":     "ALLOCATION_CANCELLED",
	"Accepted by Consultant":   "ACCEPTED_BY_CONSULTANT",
	"Rejected - Other":         "REJECTED_OTHER",
	"Rejected by Consultant":   "REJECTED_BY_CONSULTANT",
	"Confirmed - Visa Pending": "CONFIRMED_VISA_PENDING",
	"Rejected - Visa Issues":   "REJECTED_VISA_ISSUES",
}

// CustomerEngagementAllocationService applies Allocation-app events to customer engagements.
type CustomerEngagementAllocationService interface {
	// ProcessAllocationEvent ports ServiceNow's processAllocationEvent in one transaction.
	ProcessAllocationEvent(ctx context.Context, ev domain.AllocationEvent) (domain.AllocationEventResult, error)
}

type customerEngagementAllocationService struct {
	repo               repository.CustomerEngagementAllocationRepository
	firefightingTypeID string
}

// NewCustomerEngagementAllocationService constructs the service; firefightingTypeID is the
// engagement_type_id (ServiceNow sys_id) written on created firefighting engagements.
func NewCustomerEngagementAllocationService(repo repository.CustomerEngagementAllocationRepository, firefightingTypeID string) CustomerEngagementAllocationService {
	return &customerEngagementAllocationService{repo: repo, firefightingTypeID: firefightingTypeID}
}

// ProcessAllocationEvent implements CustomerEngagementAllocationService.
func (s *customerEngagementAllocationService) ProcessAllocationEvent(ctx context.Context, ev domain.AllocationEvent) (domain.AllocationEventResult, error) {
	in, err := normalizeAllocationEvent(ev)
	if err != nil {
		return domain.AllocationEventResult{}, err
	}
	var res domain.AllocationEventResult
	err = s.repo.InTx(ctx, func(store repository.AllocationEventStore) error {
		var txErr error
		res, txErr = s.process(ctx, store, in)
		return txErr
	})
	if err != nil {
		return domain.AllocationEventResult{}, err
	}
	return res, nil
}

func allocSkipped(reason string) domain.AllocationEventResult {
	return domain.AllocationEventResult{Result: domain.AllocationEventSkipped, Reason: reason}
}

func (s *customerEngagementAllocationService) process(ctx context.Context, store repository.AllocationEventStore, in allocationInput) (domain.AllocationEventResult, error) {
	if in.engagement == nil {
		return allocSkipped(AllocationSkipNoEngagementDetails), nil
	}
	var engagementID string
	created := false
	if firefightingAllocationTypes[in.allocationType] {
		if in.engagementID == "" {
			return allocSkipped(AllocationSkipNoEngagementID), nil
		}
		existing, err := store.FindEngagementByEngagementID(ctx, in.engagementID)
		if err != nil {
			return domain.AllocationEventResult{}, err
		}
		if existing != nil {
			engagementID = *existing
		} else {
			accountID, reason, err := resolveAllocationAccount(ctx, store, in)
			if err != nil {
				return domain.AllocationEventResult{}, err
			}
			if accountID == "" {
				return allocSkipped(reason), nil
			}
			engagementID, created, err = store.InsertEngagement(ctx, s.newFirefightingEngagement(in, accountID))
			if err != nil {
				return domain.AllocationEventResult{}, err
			}
		}
	} else {
		if in.productID == "" {
			return allocSkipped(AllocationSkipNoLineItem), nil
		}
		found, err := store.FindEngagementByLineItemSfID(ctx, in.productID)
		if err != nil {
			return domain.AllocationEventResult{}, err
		}
		if found == nil {
			return allocSkipped(AllocationSkipNoLineItem), nil
		}
		engagementID = *found
	}

	res := domain.AllocationEventResult{EngagementID: &engagementID, EngagementCreated: created}
	fields := in.resourceFields(engagementID)
	updated, err := store.UpdateAllocationResource(ctx, fields)
	if err != nil {
		return domain.AllocationEventResult{}, err
	}
	if updated != nil {
		res.Result, res.AllocationResourceID = domain.AllocationEventUpdated, updated
		return res, nil
	}
	userID, err := store.FindUserByEmailOrUserName(ctx, in.email)
	if err != nil {
		return domain.AllocationEventResult{}, err
	}
	if userID == nil {
		// Like ServiceNow, an engagement created above is kept.
		res.Result, res.Reason = domain.AllocationEventSkipped, AllocationSkipUserNotFound
		return res, nil
	}
	rid, inserted, err := store.UpsertAllocationResource(ctx, fields, *userID)
	if err != nil {
		return domain.AllocationEventResult{}, err
	}
	res.AllocationResourceID = &rid
	res.Result = domain.AllocationEventUpdated
	if inserted {
		res.Result = domain.AllocationEventCreated
	}
	return res, nil
}

// resolveAllocationAccount matches customerCode to account.sf_id, then falls back to a
// unique account name; an empty id comes with the skip reason.
func resolveAllocationAccount(ctx context.Context, store repository.AllocationEventStore, in allocationInput) (string, string, error) {
	if in.customerCode != "" {
		id, err := store.FindAccountBySfID(ctx, in.customerCode)
		if err != nil {
			return "", "", err
		}
		if id != nil {
			return *id, "", nil
		}
	}
	if in.customerName == "" {
		return "", AllocationSkipAccountNotFound, nil
	}
	all, err := store.FindAccountsByName(ctx, in.customerName)
	if err != nil {
		return "", "", err
	}
	var live []domain.AccountCandidate
	for _, c := range all {
		if c.Live {
			live = append(live, c)
		}
	}
	pick := all
	if len(live) > 0 {
		pick = live
	}
	switch len(pick) {
	case 0:
		return "", AllocationSkipAccountNotFound, nil
	case 1:
		return pick[0].ID, "", nil
	default:
		return "", AllocationSkipAmbiguousAccount, nil
	}
}

func (s *customerEngagementAllocationService) newFirefightingEngagement(in allocationInput, accountID string) domain.NewCustomerEngagement {
	return domain.NewCustomerEngagement{
		EngagementID:     in.engagementID,
		EngagementCode:   in.engagementCode,
		Name:             allocTruncate(in.customerName+" - "+in.allocationTypeName, 200),
		AccountID:        accountID,
		IsPaid:           in.engagement.EngagementTypeName == "Paid",
		DeliveryMode:     deliveryModeFromNature(in.engagement.EngagementNature),
		EngagementTypeID: s.firefightingTypeID,
		PlannedStartDate: in.startDate,
		PlannedEndDate:   in.endDate,
	}
}

// deliveryModeFromNature maps engagementNature to customer_engagement_delivery_mode_enum
// by meaning (csm-sync: Offsite/"0" -> OFFSITE, Onsite/"1" -> ONSITE); others stay NULL.
func deliveryModeFromNature(nature string) *string {
	var mode string
	switch strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(nature)) {
	case "offsite":
		mode = "OFFSITE"
	case "onsite":
		mode = "ONSITE"
	default:
		return nil
	}
	return &mode
}

// allocationInput is a validated, trimmed AllocationEvent.
type allocationInput struct {
	id, email, allocationTypeName                       string
	allocationType                                      int
	startDate, endDate, startTime, endTime, timeZone    *string
	state                                               *string
	engagement                                          *domain.AllocationEventEngagement
	engagementID, productID, customerCode, customerName string
	engagementCode                                      *string
}

func (in allocationInput) resourceFields(engagementID string) domain.AllocationResourceFields {
	return domain.AllocationResourceFields{
		EngagementID: engagementID, AllocationID: in.id,
		StartDate: in.startDate, EndDate: in.endDate,
		StartTime: in.startTime, EndTime: in.endTime,
		TimeZone: in.timeZone, State: in.state,
	}
}

// normalizeAllocationEvent validates ev against the column sizes, so a bad event is a
// 400 rather than a retried 500.
func normalizeAllocationEvent(ev domain.AllocationEvent) (allocationInput, error) {
	in := allocationInput{
		id:                 strings.TrimSpace(ev.ID),
		email:              strings.TrimSpace(ev.Email),
		allocationType:     ev.AllocationType,
		allocationTypeName: strings.TrimSpace(ev.AllocationTypeName),
		engagement:         ev.Engagement,
	}
	if in.id == "" || in.email == "" {
		return allocationInput{}, &apierror.ValidationError{Msg: "id and email are required"}
	}
	var err error
	if in.startDate, err = allocDate("startDate", ev.StartDate); err != nil {
		return allocationInput{}, err
	}
	if in.endDate, err = allocDate("endDate", ev.EndDate); err != nil {
		return allocationInput{}, err
	}
	in.startTime, in.endTime, in.timeZone = allocTrimmed(ev.StartTime), allocTrimmed(ev.EndTime), allocTrimmed(ev.TimeZone)
	if ev.ClearanceStatus != nil {
		if st, ok := allocationClearanceStates[strings.TrimSpace(*ev.ClearanceStatus)]; ok {
			in.state = &st
		}
	}
	in.customerCode = allocDeref(allocTrimmed(ev.CustomerCode))
	if e := ev.Engagement; e != nil {
		in.engagementID = strings.TrimSpace(e.EngagementID)
		in.engagementCode = allocTrimmed(&e.EngagementCode)
		in.productID = allocDeref(allocTrimmed(e.ProductID))
		in.customerName = strings.TrimSpace(e.CustomerName)
		if in.customerCode == "" {
			in.customerCode = allocDeref(allocTrimmed(e.CustomerCode))
		}
	}
	limits := []struct {
		field string
		value *string
		max   int
	}{
		{"id", &in.id, 20}, {"startTime", in.startTime, 8}, {"endTime", in.endTime, 8},
		{"timeZone", in.timeZone, 20}, {"engagement.engagementId", &in.engagementID, 20},
		{"engagement.engagementCode", in.engagementCode, 20},
	}
	for _, l := range limits {
		if l.value != nil && utf8.RuneCountInString(*l.value) > l.max {
			return allocationInput{}, &apierror.ValidationError{Msg: l.field + " is too long"}
		}
	}
	return in, nil
}

// allocDate accepts YYYY-MM-DD, optionally followed by a time part.
func allocDate(field, v string) (*string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if len(v) > 10 {
		v = v[:10]
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return nil, &apierror.ValidationError{Msg: field + " must be a date (YYYY-MM-DD)"}
	}
	return &v, nil
}

func allocTrimmed(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

func allocDeref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func allocTruncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
