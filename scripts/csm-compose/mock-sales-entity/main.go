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
//

// LOCAL DEV ONLY. A stand-in for REST sales/sales-entity-service with the
// handful of endpoints entity-service's portal membership writes use, plus a
// switch that makes it answer 503 to everything -- which is how the deferred
// Salesforce write (entity-service SALESFORCE_DEFERRED_WRITES_ENABLED) is
// exercised end to end without Salesforce:
//
//	POST /__mock/outage {"down":true|false}   flip the outage
//	GET  /__mock/state                        dump contacts and memberships
//
// Records live in memory and vanish on restart. Ids are invented
// ("003MOCK…", "a0eMOCK…") and never collide with real Salesforce ids.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type contact struct {
	ID                  string          `json:"id"`
	Email               *string         `json:"email"`
	Name                *string         `json:"name"`
	FirstName           *string         `json:"firstName"`
	LastName            *string         `json:"lastName"`
	IsCsAdmin           *bool           `json:"isCsAdmin"`
	IsCsIntegrationUser *bool           `json:"isCsIntegrationUser"`
	LockoutStatus       *bool           `json:"lockoutStatus"`
	Account             *contactAccount `json:"account"`
	LastModifiedDate    *string         `json:"lastModifiedDate"`
}

type contactAccount struct {
	ID             *string `json:"id"`
	Classification *string `json:"classification"`
	IsPartner      *bool   `json:"isPartner"`
}

type projectContact struct {
	ID               string        `json:"id"`
	Email            string        `json:"email"`
	State            *string       `json:"state"`
	Roles            []string      `json:"roles"`
	Type             *string       `json:"type"`
	Contact          *pcContact    `json:"contact"`
	Subscription     *subscription `json:"subscription"`
	CreatedDate      *string       `json:"createdDate"`
	LastModifiedDate *string       `json:"lastModifiedDate"`
}

type pcContact struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Email      *string `json:"email"`
	CustomerID *string `json:"customerId"`
}

type subscription struct {
	ID         *string `json:"id"`
	Name       *string `json:"name"`
	Key        *string `json:"key"`
	CustomerID *string `json:"customerId"`
}

type customer struct {
	ID                    string  `json:"id"`
	Name                  *string `json:"name"`
	AccountClassification *string `json:"accountClassification"`
	DomainList            *string `json:"domainList"`
	IsPartner             *bool   `json:"isPartner"`
	LastModifiedDate      *string `json:"lastModifiedDate"`
}

type store struct {
	mu          sync.Mutex
	down        atomic.Bool
	seq         int
	contacts    map[string]*contact
	memberships map[string]*projectContact
	// domainList is what every customer-search answers with, so the
	// invitation validator accepts the test address (MOCK_SF_DOMAIN_LIST).
	domainList string
	calls      atomic.Int64
}

func str(s string) *string { return &s }
func boolp(b bool) *bool   { return &b }
func now() *string         { return str(time.Now().UTC().Format(time.RFC3339Nano)) }

func (s *store) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%sMOCK%012d", prefix, s.seq)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// outageGate answers 503 to every business endpoint while the outage is on.
func (s *store) outageGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__mock/") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		s.calls.Add(1)
		if s.down.Load() {
			log.Printf("OUTAGE  %s %s -> 503", r.Method, r.URL.Path)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mock sales-entity: simulated outage"})
			return
		}
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9200"
	}
	s := &store{contacts: map[string]*contact{}, memberships: map[string]*projectContact{}, domainList: os.Getenv("MOCK_SF_DOMAIN_LIST")}
	if s.domainList == "" {
		s.domainList = "example.com"
	}
	if os.Getenv("MOCK_SF_START_DOWN") == "true" {
		s.down.Store(true)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /__mock/outage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Down bool `json:"down"`
		}
		if err := decode(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.down.Store(body.Down)
		log.Printf("outage set to %v", body.Down)
		writeJSON(w, http.StatusOK, map[string]any{"down": body.Down})
	})
	mux.HandleFunc("GET /__mock/state", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"down": s.down.Load(), "calls": s.calls.Load(), "contacts": s.contacts, "memberships": s.memberships})
	})

	// OAuth2 client credentials: any client is welcome.
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "mock-sales-entity-token", "token_type": "Bearer", "expires_in": 3600})
	})

	mux.HandleFunc("POST /customer-search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		_ = decode(r, &req)
		out := make([]customer, 0, len(req.IDs))
		for _, id := range req.IDs {
			out = append(out, customer{ID: id, Name: str("Mock Account " + id), AccountClassification: str("Customer"),
				DomainList: str(s.domainList), IsPartner: boolp(false), LastModifiedDate: now()})
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /contacts/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Limit int    `json:"limit"`
		}
		_ = decode(r, &req)
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []contact{}
		for _, c := range s.contacts {
			if (req.ID != "" && c.ID == req.ID) || (req.Email != "" && c.Email != nil && strings.EqualFold(*c.Email, req.Email)) {
				out = append(out, *c)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /contacts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			FirstName           string `json:"firstName"`
			LastName            string `json:"lastName"`
			Email               string `json:"email"`
			AccountID           string `json:"accountId"`
			IsCsIntegrationUser bool   `json:"isCsIntegrationUser"`
		}
		if err := decode(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		c := &contact{ID: s.nextID("003"), Email: str(strings.ToLower(in.Email)), Name: str(strings.TrimSpace(in.FirstName + " " + in.LastName)),
			FirstName: str(in.FirstName), LastName: str(in.LastName), IsCsAdmin: boolp(false), IsCsIntegrationUser: boolp(in.IsCsIntegrationUser),
			LockoutStatus: boolp(true), Account: &contactAccount{ID: str(in.AccountID), Classification: str("Customer"), IsPartner: boolp(false)},
			LastModifiedDate: now()}
		s.contacts[c.ID] = c
		// Like Salesforce: every POST creates, duplicates included.
		writeJSON(w, http.StatusCreated, c)
	})

	mux.HandleFunc("PATCH /contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			LockoutStatus bool `json:"lockoutStatus"`
		}
		_ = decode(r, &in)
		s.mu.Lock()
		defer s.mu.Unlock()
		c, ok := s.contacts[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such contact"})
			return
		}
		c.LockoutStatus = boolp(in.LockoutStatus)
		c.LastModifiedDate = now()
		writeJSON(w, http.StatusOK, c)
	})

	mux.HandleFunc("POST /project-contacts/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID        string `json:"id"`
			ProjectID string `json:"projectId"`
			ContactID string `json:"contactId"`
		}
		_ = decode(r, &req)
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []projectContact{}
		for _, m := range s.memberships {
			switch {
			case req.ID != "" && m.ID == req.ID:
				out = append(out, *m)
			case req.ID == "" && req.ProjectID != "" && req.ContactID != "" &&
				m.Subscription != nil && m.Subscription.ID != nil && *m.Subscription.ID == req.ProjectID &&
				m.Contact != nil && m.Contact.ID != nil && *m.Contact.ID == req.ContactID:
				out = append(out, *m)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /project-contacts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ProjectID string   `json:"projectId"`
			ContactID string   `json:"contactId"`
			State     string   `json:"state"`
			Role      []string `json:"role"`
		}
		if err := decode(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		c, ok := s.contacts[in.ContactID]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown contactId " + in.ContactID})
			return
		}
		email := ""
		if c.Email != nil {
			email = *c.Email
		}
		var accountID *string
		if c.Account != nil {
			accountID = c.Account.ID
		}
		m := &projectContact{ID: s.nextID("a0e"), Email: email, State: str(in.State), Roles: in.Role, Type: str("OWN CONTACT"),
			Contact:      &pcContact{ID: str(c.ID), Name: c.Name, Email: c.Email, CustomerID: accountID},
			Subscription: &subscription{ID: str(in.ProjectID), Name: str("Mock Project"), Key: str("MOCKPROJ"), CustomerID: accountID},
			CreatedDate:  now(), LastModifiedDate: now()}
		s.memberships[m.ID] = m
		writeJSON(w, http.StatusCreated, m)
	})

	mux.HandleFunc("PATCH /project-contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			State *string   `json:"state"`
			Role  *[]string `json:"role"`
		}
		_ = decode(r, &in)
		s.mu.Lock()
		defer s.mu.Unlock()
		m, ok := s.memberships[r.PathValue("id")]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such membership"})
			return
		}
		if in.State != nil {
			m.State = in.State
		}
		if in.Role != nil {
			m.Roles = *in.Role
		}
		m.LastModifiedDate = now()
		writeJSON(w, http.StatusOK, m)
	})

	log.Printf("mock sales-entity listening on :%s (down=%v, domainList=%s)", port, s.down.Load(), s.domainList)
	log.Fatal(http.ListenAndServe(":"+port, s.outageGate(mux)))
}
