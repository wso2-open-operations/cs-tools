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

// Package auth validates the Asgardeo-issued x-user-id-token entity-service
// receives, and carries the verified caller identity to the services that
// scope by it.
//
// Two tokens can arrive on a request:
//   - x-user-id-token: the end user's ID token (email claim, audience = one of
//     the accepted application client ids). Present when a portal backend acts
//     for a user. Fully verified: signature, issuer, expiry, audience.
//   - x-jwt-assertion: the calling application's own client-credentials
//     assertion (client_id/azp claim). Present on every service-to-service
//     call; it is the ONLY token a machine-to-machine caller (e.g.
//     csm-integration-service) sends. Only decoded, never signature-verified
//     -- see ExtractClientID's own doc comment for why.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Config holds token validation settings. See config.Config.Auth* for what
// each means.
type Config struct {
	Issuer             string
	JWKSURL            string
	UserTokenAudiences []string
	ClockSkew          time.Duration
}

// signingMethods restricts accepted algorithms to asymmetric ones, so a token
// can never be "verified" by treating a public key as an HMAC secret.
var signingMethods = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512"}

// UserClaims is what a validated x-user-id-token yields.
type UserClaims struct {
	Email   string
	Subject string
	// UserID is the token's "userid" claim -- Asgardeo's stable, per-account
	// user identifier. Unlike Subject ("sub"), which csm-portal-backend's own
	// frontend has separately documented as per-session rather than stable
	// (see that repo's IdTokenClaims doc comment), UserID is the same value
	// both csm-portal-backend and customer-portal backend-v2 already decode
	// from this identical token into their own UserInfo.UserID and log on
	// every request they handle -- see that value's own doc comment for why
	// this one, not Subject, is the field to correlate a request across
	// services by.
	UserID string
}

// ClientClaims is what a decoded client-credentials assertion yields. See
// ExtractClientID for why this is a decode, not a validation.
type ClientClaims struct {
	ClientID string
}

type tokenClaims struct {
	Email    string `json:"email"`
	UserID   string `json:"userid"`
	ClientID string `json:"client_id"`
	AZP      string `json:"azp"`
	jwt.RegisteredClaims
}

// Validator verifies signatures against the issuer's JWKS and checks issuer,
// expiry and (for user tokens) audience.
type Validator struct {
	cfg     Config
	keyFunc jwt.Keyfunc
}

// NewValidator fetches the issuer's JWKS and fails unless at least one key
// loaded, so a wrong or unreachable JWKS URL stops the process at startup.
// That check is deliberate: keyfunc itself only logs a failed initial fetch
// and keeps retrying in the background, which would otherwise leave a
// misconfigured deployment up while silently rejecting every token.
func NewValidator(ctx context.Context, cfg Config) (*Validator, error) {
	client := &http.Client{Transport: &x5cStrippingTransport{base: http.DefaultTransport}, Timeout: 15 * time.Second}
	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, keyfunc.Override{Client: client})
	if err != nil {
		return nil, fmt.Errorf("auth: initialise JWKS from %s: %w", cfg.JWKSURL, err)
	}
	keys, err := jwks.Storage().KeyReadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: read JWKS from %s: %w", cfg.JWKSURL, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("auth: no signing keys loaded from %s", cfg.JWKSURL)
	}
	return &Validator{cfg: cfg, keyFunc: jwks.Keyfunc}, nil
}

// NewValidatorWithKeyfunc builds a Validator around an existing key function
// (tests supply a static key rather than a JWKS endpoint).
func NewValidatorWithKeyfunc(cfg Config, kf jwt.Keyfunc) *Validator {
	return &Validator{cfg: cfg, keyFunc: kf}
}

func (v *Validator) parse(raw string) (*tokenClaims, error) {
	var c tokenClaims
	tok, err := jwt.ParseWithClaims(raw, &c, v.keyFunc,
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithLeeway(v.cfg.ClockSkew),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods(signingMethods),
	)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return &c, nil
}

// ValidateUserToken validates an end user's ID token: signature, issuer,
// expiry, an audience among the configured application client ids, and a
// non-empty email claim.
func (v *Validator) ValidateUserToken(raw string) (UserClaims, error) {
	c, err := v.parse(raw)
	if err != nil {
		return UserClaims{}, err
	}
	if !hasAnyAudience(c.Audience, v.cfg.UserTokenAudiences) {
		return UserClaims{}, errors.New("token audience not accepted")
	}
	if strings.TrimSpace(c.Email) == "" {
		return UserClaims{}, errors.New("token missing email claim")
	}
	return UserClaims{Email: strings.TrimSpace(c.Email), Subject: c.Subject, UserID: strings.TrimSpace(c.UserID)}, nil
}

// ExtractClientID reads the client id (client_id claim, falling back to azp)
// out of a client-credentials assertion -- x-jwt-assertion -- WITHOUT
// verifying its signature, issuer, or expiry. Unlike ValidateUserToken, this
// is a plain decode.
//
// This is deliberate, not a shortcut: x-jwt-assertion is minted by the
// gateway in front of this service after it has already authenticated the
// caller by whatever means that gateway uses, and delivered over a path this
// service already trusts (the same reason apps/csm-portal/backend's own
// x-jwt-assertion handling runs with signature verification off in every
// Choreo deployment, not just locally -- see that repo's own
// middleware.Auth). Re-verifying it here against Asgardeo's JWKS doesn't add
// security (the token isn't necessarily even Asgardeo-issued or signed with a
// key that JWKS publishes) and did cause real outages: a JWKS refresh
// rate-limit or transient lookup failure turned into every internal caller
// being rejected. The client id is only ever used to check membership in
// AUTH_INTERNAL_CLIENT_IDS -- a deployment-controlled allow-list, not a
// capability grant derived from unproven claims -- so trusting it at face
// value here carries no more risk than trusting any other value read off this
// same header elsewhere in the fleet.
func (v *Validator) ExtractClientID(raw string) (ClientClaims, error) {
	var c tokenClaims
	if _, _, err := new(jwt.Parser).ParseUnverified(raw, &c); err != nil {
		return ClientClaims{}, fmt.Errorf("decode token: %w", err)
	}
	id := strings.TrimSpace(c.ClientID)
	if id == "" {
		id = strings.TrimSpace(c.AZP)
	}
	if id == "" {
		return ClientClaims{}, errors.New("token has neither client_id nor azp claim")
	}
	return ClientClaims{ClientID: id}, nil
}

func hasAnyAudience(tokenAuds jwt.ClaimStrings, accepted []string) bool {
	for _, want := range accepted {
		for _, got := range tokenAuds {
			if got == want {
				return true
			}
		}
	}
	return false
}
