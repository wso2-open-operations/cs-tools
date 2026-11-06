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

// LOCAL DEV ONLY. A fake of the Salesforce REST API, just enough for
// sales-entity-service's membership write and the reads around it:
//
//	POST /services/oauth2/token                          any grant → a token
//	GET  /services/data/v{ver}/query?q=SOQL              Contact, Project_Contact__c, Account
//	POST /services/data/v{ver}/sobjects/{object}         create → 201 {id, success}
//	PATCH /services/data/v{ver}/sobjects/{object}/{id}   update → 204
//	POST /__mock/outage {"down":true|false}              every /services/* answers 503 while down
//	GET  /__mock/state                                   dump the store
//
// The SOQL evaluator understands the predicates the service emits
// (Field = 'v', Field IN ('a','b'), relationship fields such as
// Contact__r.Email and Account.Id); anything else is treated as true. Records
// are returned with the API field names the service's sf* structs decode.
// Ids are unique across restarts (Salesforce ids are), so a restarted fake
// never hands out an id an earlier run's CSM row already carries.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type record map[string]any

type store struct {
	mu         sync.Mutex
	down       atomic.Bool
	seq        int
	epoch      int64
	objects    map[string]map[string]record // object → Id → fields
	domainList string
}

const sfTime = "2006-01-02T15:04:05.000+0000"

func now() string { return time.Now().UTC().Format(sfTime) }

func (s *store) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%sM%08d%06d", prefix, s.epoch, s.seq)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// account is the synthetic Account every contact and project hangs off; any
// id the caller names exists.
func (s *store) account(id string) record {
	return record{"attributes": record{"type": "Account"}, "Id": id, "Name": "Mock Account " + id, "Website": nil, "IsPartner": false,
		"Domain_list__c": s.domainList, "Account_Classification__c": "Customer", "Type": "Customer", "Owner": nil,
		"Account_Status__c": "Active", "LastModifiedDate": now()}
}

func (s *store) project(id, accountID string) record {
	return record{"Id": id, "Name": "Mock Project " + id, "Project_Key__c": "MOCKPROJ", "Account__c": accountID}
}

// view returns the record as the service's SELECT expects it, with the
// relationship fields resolved.
func (s *store) view(object string, r record) record {
	out := record{"attributes": record{"type": object}}
	for k, v := range r {
		out[k] = v
	}
	switch object {
	case "Contact":
		out["Account"] = s.account(str(r["AccountId"]))
		out["Project_Contacts__r"] = nil
		out["AccountContactRelations"] = nil
		out["Owner"] = nil
	case "Project_Contact__c":
		var contact record
		if c, ok := s.objects["Contact"][str(r["Contact__c"])]; ok {
			contact = record{"Id": c["Id"], "Email": c["Email"], "Name": c["Name"], "AccountId": c["AccountId"]}
		}
		out["Contact__r"] = contact
		accountID := ""
		if contact != nil {
			accountID = str(contact["AccountId"])
		}
		out["Project__r"] = s.project(str(r["Project__c"]), accountID)
	}
	return out
}

// field resolves a (possibly dotted) SOQL field against a record view.
func field(v record, name string) string {
	parts := strings.Split(name, ".")
	var cur any = v
	for _, p := range parts {
		m, ok := cur.(record)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	return str(cur)
}

var (
	reQuery = regexp.MustCompile(`(?is)\bFROM\s+(\w+)\s*(?:\bWHERE\s+(.*?))?\s*(?:\bORDER\s+BY\b.*|\bLIMIT\b.*|\bOFFSET\b.*)?$`)
	reEq    = regexp.MustCompile(`^([\w.]+)\s*=\s*'((?:[^'\\]|\\.)*)'$`)
	reIn    = regexp.MustCompile(`^([\w.]+)\s+IN\s*\((.*)\)$`)
	reLit   = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)
)

func unescape(s string) string { return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(s) }

// splitAnd splits a WHERE clause on top-level AND.
func splitAnd(where string) []string {
	var out []string
	depth, start, inQuote := 0, 0, false
	up := strings.ToUpper(where)
	for i := 0; i < len(where); i++ {
		switch {
		case where[i] == '\\' && inQuote:
			i++
		case where[i] == '\'':
			inQuote = !inQuote
		case inQuote:
		case where[i] == '(':
			depth++
		case where[i] == ')':
			depth--
		case depth == 0 && i+5 <= len(where) && up[i:i+5] == " AND ":
			out = append(out, strings.TrimSpace(where[start:i]))
			start = i + 5
			i += 4
		}
	}
	out = append(out, strings.TrimSpace(where[start:]))
	return out
}

func matches(v record, pred string) bool {
	pred = strings.TrimSpace(pred)
	if m := reEq.FindStringSubmatch(pred); m != nil {
		return strings.EqualFold(field(v, m[1]), unescape(m[2]))
	}
	if m := reIn.FindStringSubmatch(pred); m != nil {
		got := field(v, m[1])
		for _, lit := range reLit.FindAllStringSubmatch(m[2], -1) {
			if strings.EqualFold(got, unescape(lit[1])) {
				return true
			}
		}
		return false
	}
	// Keyset predicates, subselects and anything else: not constraints the
	// write depends on.
	return true
}

func (s *store) query(w http.ResponseWriter, r *http.Request) {
	soql := strings.Join(strings.Fields(r.URL.Query().Get("q")), " ")
	m := reQuery.FindStringSubmatch(soql)
	if m == nil {
		writeJSON(w, http.StatusBadRequest, []record{{"message": "cannot parse SOQL", "errorCode": "MALFORMED_QUERY"}})
		return
	}
	object, where := m[1], strings.TrimSpace(m[2])
	s.mu.Lock()
	defer s.mu.Unlock()
	var rows []record
	switch object {
	case "Account":
		// Any account the caller names exists.
		for _, pred := range splitAnd(where) {
			if mm := reEq.FindStringSubmatch(strings.TrimSpace(pred)); mm != nil && strings.EqualFold(mm[1], "Id") {
				rows = append(rows, s.account(unescape(mm[2])))
			} else if mm := reIn.FindStringSubmatch(strings.TrimSpace(pred)); mm != nil && strings.EqualFold(mm[1], "Id") {
				for _, lit := range reLit.FindAllStringSubmatch(mm[2], -1) {
					rows = append(rows, s.account(unescape(lit[1])))
				}
			}
		}
	default:
		for _, rec := range s.objects[object] {
			v := s.view(object, rec)
			ok := true
			if where != "" {
				for _, pred := range splitAnd(where) {
					if !matches(v, pred) {
						ok = false
						break
					}
				}
			}
			if ok {
				rows = append(rows, v)
			}
		}
	}
	if rows == nil {
		rows = []record{}
	}
	log.Printf("QUERY %s WHERE %s -> %d", object, where, len(rows))
	writeJSON(w, http.StatusOK, record{"totalSize": len(rows), "done": true, "records": rows})
}

func (s *store) create(w http.ResponseWriter, r *http.Request) {
	object := r.PathValue("object")
	var fields record
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeJSON(w, http.StatusBadRequest, []record{{"message": err.Error(), "errorCode": "JSON_PARSER_ERROR"}})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := map[string]string{"Contact": "003", "Project_Contact__c": "a0e"}[object]
	if prefix == "" {
		prefix = "a00"
	}
	if object == "Contact" && str(fields["LastName"]) == "" {
		writeJSON(w, http.StatusBadRequest, []record{{"message": "Required fields are missing: [LastName]", "errorCode": "REQUIRED_FIELD_MISSING", "fields": []string{"LastName"}}})
		return
	}
	id := s.nextID(prefix)
	rec := record{"Id": id, "CreatedDate": now(), "LastModifiedDate": now()}
	for k, v := range fields {
		rec[k] = v
	}
	if object == "Contact" {
		rec["Name"] = strings.TrimSpace(str(fields["FirstName"]) + " " + str(fields["LastName"]))
		if _, ok := rec["State__c"]; !ok {
			rec["State__c"] = true // locked out until registration
		}
	}
	if s.objects[object] == nil {
		s.objects[object] = map[string]record{}
	}
	s.objects[object][id] = rec
	log.Printf("CREATE %s %s %v", object, id, fields)
	writeJSON(w, http.StatusCreated, record{"id": id, "success": true, "errors": []string{}})
}

func (s *store) update(w http.ResponseWriter, r *http.Request) {
	object, id := r.PathValue("object"), r.PathValue("id")
	var fields record
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeJSON(w, http.StatusBadRequest, []record{{"message": err.Error(), "errorCode": "JSON_PARSER_ERROR"}})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.objects[object][id]
	if !ok {
		writeJSON(w, http.StatusNotFound, []record{{"message": "Provided external ID field does not exist or is not accessible", "errorCode": "NOT_FOUND"}})
		return
	}
	for k, v := range fields {
		rec[k] = v
	}
	rec["LastModifiedDate"] = now()
	log.Printf("UPDATE %s %s %v", object, id, fields)
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9300"
	}
	s := &store{epoch: time.Now().Unix() % 100000000, objects: map[string]map[string]record{"Contact": {}, "Project_Contact__c": {}}, domainList: os.Getenv("MOCK_SF_DOMAIN_LIST")}
	if s.domainList == "" {
		s.domainList = "example.com"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /__mock/outage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Down bool `json:"down"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, record{"error": err.Error()})
			return
		}
		s.down.Store(body.Down)
		log.Printf("outage set to %v", body.Down)
		writeJSON(w, http.StatusOK, record{"down": body.Down})
	})
	mux.HandleFunc("GET /__mock/state", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, record{"down": s.down.Load(), "contacts": s.objects["Contact"], "memberships": s.objects["Project_Contact__c"]})
	})
	mux.HandleFunc("POST /services/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, record{"access_token": "mock-salesforce-token", "instance_url": "http://" + r.Host, "token_type": "Bearer"})
	})
	mux.HandleFunc("GET /services/data/{ver}/query", s.query)
	mux.HandleFunc("GET /services/data/{ver}/queryAll", s.query)
	mux.HandleFunc("POST /services/data/{ver}/sobjects/{object}", s.create)
	mux.HandleFunc("PATCH /services/data/{ver}/sobjects/{object}/{id}", s.update)

	gate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/services/") && s.down.Load() {
			log.Printf("OUTAGE %s %s -> 503", r.Method, r.URL.Path)
			writeJSON(w, http.StatusServiceUnavailable, []record{{"message": "simulated Salesforce outage", "errorCode": "SERVER_UNAVAILABLE"}})
			return
		}
		mux.ServeHTTP(w, r)
	})
	log.Printf("fake Salesforce listening on :%s (domainList=%s)", port, s.domainList)
	log.Fatal(http.ListenAndServe(":"+port, gate))
}
