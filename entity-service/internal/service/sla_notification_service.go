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
	"net"
	"net/netip"
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SLANotificationRuleService is the admin-facing surface of
// sla_notification_rule (R46-R51).
type SLANotificationRuleService interface {
	List(ctx context.Context) ([]domain.SLANotificationRule, error)
	Get(ctx context.Context, id string) (domain.SLANotificationRule, error)
	Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error)
	Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error)
}

type slaNotificationRuleService struct {
	repo       repository.SLANotificationRuleRepository
	access     AccessService
	adminRoles []string
}

// NewSLANotificationRuleService constructs an SLANotificationRuleService.
// adminRoles is AUTH_ADMIN_ROLES.
func NewSLANotificationRuleService(repo repository.SLANotificationRuleRepository, access AccessService, adminRoles []string) SLANotificationRuleService {
	return &slaNotificationRuleService{repo: repo, access: access, adminRoles: adminRoles}
}

func (s *slaNotificationRuleService) List(ctx context.Context) ([]domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *slaNotificationRuleService) Get(ctx context.Context, id string) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if id == "" {
		return domain.SLANotificationRule{}, &apierror.ValidationError{Msg: "notification rule id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLANotificationRule{}, err
	}
	return s.repo.Get(ctx, id)
}

// validateWebhookURL requires an absolute https URL with a host -- not just
// a non-empty string. csm-notification-service will later call this URL
// from inside the network on a threshold-crossing event; a value like
// "http://169.254.169.254/..." (a cloud metadata endpoint) or "file:///..."
// would otherwise be stored and dispatched to unchecked. This route is
// admin-only, so an attacker would need to already be an admin to exploit
// it -- a hardening gap, not an open one -- but it costs nothing to close.
// Mirrors apps/csm-portal/backend's own validateSecureURL (a different Go
// module, so not importable here), minus its "no path" rule: a webhook
// URL's whole point is a path.
// disallowedWebhookHostnames are hostnames that resolve (or self-identify)
// as this machine regardless of DNS -- net.ParseIP never matches a plain
// name like "localhost", so it needs its own check alongside the IP-literal
// one below.
var disallowedWebhookHostnames = map[string]bool{
	"localhost":             true,
	"localhost.localdomain": true,
}

// isDisallowedWebhookIP reports whether ip is loopback, private, link-local
// (unicast or multicast), unspecified ("any address") or multicast -- every
// category of address that has no business being dialed as a public
// chat-provider webhook. net.IP's own IsLoopback/IsPrivate/etc. already
// normalize an IPv4 address embedded in an IPv6 one (e.g.
// "::ffff:169.254.169.254", a cloud metadata endpoint written in
// IPv6-mapped form) internally, so no separate To4() handling is needed
// here.
func isDisallowedWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}

// validateWebhookURL requires an absolute https URL with a public host --
// not just a non-empty string, and not just "https with some host".
// csm-notification-service will later dial this URL from inside the
// network on a threshold-crossing event; a value such as
// "https://169.254.169.254/..." (a cloud metadata endpoint) or
// "https://localhost/..." would otherwise be stored and later dispatched
// to unchecked. This route is admin-only, so an attacker would need to
// already be an admin to exploit it -- a hardening gap, not an open one --
// but it costs little to close. Mirrors apps/csm-portal/backend's own
// validateSecureURL (a different Go module, so not importable here), minus
// its "no path" rule (a webhook URL's whole point is a path), plus the
// IP-literal/loopback/private/link-local/localhost checks that function
// doesn't need for its own callers (fixed, operator-configured base URLs,
// never a value an admin types in through a browser form).
//
// What this does NOT close: a hostname that is public today (passes every
// check here) but is later re-pointed by its own DNS at a private address
// (DNS rebinding), or one whose A/AAAA record already resolves to a private
// address at request time despite looking like an ordinary public name --
// this function only ever sees the literal host string, never a resolved
// IP. Closing that needs the *dispatching* HTTP client (in
// csm-notification-service, which does not exist yet -- see this change's
// own plan file) to validate the IP it actually connects to, e.g. via a
// custom net.Dialer.Control or a DialContext that re-checks
// isDisallowedWebhookIP against the resolved address before completing the
// connection. Flagged here rather than silently left for whoever builds
// that piece to discover on their own.
func validateWebhookURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &apierror.ValidationError{Msg: "webhookUrl is not a valid URL"}
	}
	if parsed.Scheme != "https" {
		return &apierror.ValidationError{Msg: "webhookUrl must use the https scheme"}
	}
	host := parsed.Hostname()
	if host == "" {
		return &apierror.ValidationError{Msg: "webhookUrl must include a host"}
	}
	if parsed.User != nil {
		return &apierror.ValidationError{Msg: "webhookUrl must not contain embedded userinfo"}
	}
	// Normalized before the literal-hostname lookup: a trailing dot
	// ("localhost.") is a legal absolute-hostname spelling url.Parse leaves
	// alone, and "foo.localhost" is a real subdomain many resolvers still
	// point at loopback -- neither would otherwise match the plain
	// "localhost" entry.
	normalizedHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if disallowedWebhookHostnames[normalizedHost] || strings.HasSuffix(normalizedHost, ".localhost") {
		return &apierror.ValidationError{Msg: "webhookUrl must not point at this machine"}
	}
	// netip.ParseAddr, not net.ParseIP: an IPv6 literal with a zone (e.g.
	// "fe80::1%eth0", written "fe80::1%25eth0" in a URL) makes
	// parsed.Hostname() return the zoned form, which net.ParseIP flatly
	// rejects (returns nil) -- silently skipping the IP-range check below
	// for exactly the address most likely to be link-local. netip handles
	// the zone, and Unmap() normalizes an IPv4-mapped IPv6 address before
	// the range checks run.
	if addr, err := netip.ParseAddr(host); err == nil {
		if isDisallowedWebhookIP(net.IP(addr.WithZone("").Unmap().AsSlice())) {
			return &apierror.ValidationError{Msg: "webhookUrl must not point at a private, loopback or link-local address"}
		}
	}
	return nil
}

func validateSLANotificationRuleRequest(req domain.UpsertSLANotificationRuleRequest) error {
	switch req.Channel {
	case "email":
	case "webhook":
		if req.WebhookURL == nil || *req.WebhookURL == "" {
			return &apierror.ValidationError{Msg: `webhookUrl is required when channel is "webhook"`}
		}
		if err := validateWebhookURL(*req.WebhookURL); err != nil {
			return err
		}
	default:
		return &apierror.ValidationError{Msg: `channel must be "email" or "webhook"`}
	}
	if req.DefinitionID != nil {
		if err := validateUUIDs("definitionId", []string{*req.DefinitionID}); err != nil {
			return err
		}
	}
	if req.ThresholdPercentage != nil && *req.ThresholdPercentage <= 0 {
		return &apierror.ValidationError{Msg: "thresholdPercentage must be greater than 0"}
	}
	if !req.Recipients.Assignee && !req.Recipients.Team && len(req.Recipients.Users) == 0 && len(req.Recipients.Groups) == 0 {
		return &apierror.ValidationError{Msg: "recipients must name at least one of assignee, team, users or groups"}
	}
	return nil
}

func (s *slaNotificationRuleService) Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if err := validateSLANotificationRuleRequest(req); err != nil {
		return domain.SLANotificationRule{}, err
	}
	return s.repo.Create(ctx, req, slaActorEmail(ctx))
}

func (s *slaNotificationRuleService) Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if id == "" {
		return domain.SLANotificationRule{}, &apierror.ValidationError{Msg: "notification rule id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if err := validateSLANotificationRuleRequest(req); err != nil {
		return domain.SLANotificationRule{}, err
	}
	return s.repo.Update(ctx, id, req, slaActorEmail(ctx))
}
