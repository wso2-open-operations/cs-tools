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

package dashboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ErrGeneratedInvalid marks a generated dashboard definition that cannot be
// shown: not one JSON object, a field this schema does not have, a field a
// generated dashboard may not set, or a definition the loader's own
// validation rejects. The error's text says which, readably enough to hand
// back to the model.
var ErrGeneratedInvalid = errors.New("invalid generated dashboard")

// GeneratedPlaceholderID is the id a parsed generated dashboard carries. The
// model is told to leave the id out, and whatever it writes is replaced: the
// generate endpoint gives the dashboard its real id (a UUID). The loader's
// validation needs a non-empty one meanwhile.
const GeneratedPlaceholderID = "generated"

// generatedSource names a generated definition in the loader's error text.
const generatedSource = "generated dashboard"

// fieldsNotForGenerated are top-level fields of the file format a generated
// dashboard may not set: they choose who sees a dashboard and when it opens
// by itself (isDefault is checked separately: false is allowed), or pull in
// shared configuration the generate path does not load.
var fieldsNotForGenerated = []string{"restricted", "defaultForTeamKeys", "includeSections", "filterPresets"}

// ParseGenerated turns a dashboard definition the model wrote (the file
// format, as in DASHBOARDS_DIR) into a Dashboard that is checked exactly as a
// definition file is: every field must be one this schema has (unlike the
// file loaders, which ignore an unknown field, so a misspelt one is silently
// dropped), then the loader's own pipeline runs -- the implied "type" filters
// are injected and every structural rule (shapes, slices/groupBy, grid width,
// type and isTeamBased, ...) is validated. On top of that a generated
// dashboard holds only case-family widgets and those of
// generatedResourceRules (the catalogue's scope; their queries are checked
// key by key, see checkGeneratedResourceWidget), is never isDefault, and sets
// none of fieldsNotForGenerated.
//
// The result's ID is GeneratedPlaceholderID. Free-form parts (each "query",
// "sortBy") are not checked field by field here; which filters Postgres
// accepts is a later check.
func ParseGenerated(raw []byte) (Dashboard, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	var value any
	if err := dec.Decode(&value); err != nil {
		return Dashboard{}, fmt.Errorf("%w: not valid JSON: %v", ErrGeneratedInvalid, err)
	}
	if dec.More() {
		return Dashboard{}, fmt.Errorf("%w: data after the dashboard object", ErrGeneratedInvalid)
	}
	top, ok := value.(map[string]any)
	if !ok {
		return Dashboard{}, fmt.Errorf("%w: the dashboard must be a JSON object", ErrGeneratedInvalid)
	}
	if err := unknownField(top, reflect.TypeOf(Dashboard{}), ""); err != nil {
		return Dashboard{}, err
	}
	for _, name := range fieldsNotForGenerated {
		if _, set := top[name]; set {
			return Dashboard{}, fmt.Errorf("%w: field %q cannot be set on a generated dashboard", ErrGeneratedInvalid, name)
		}
	}

	var d Dashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return Dashboard{}, fmt.Errorf("%w: %v", ErrGeneratedInvalid, err)
	}
	if d.IsDefault {
		return Dashboard{}, fmt.Errorf("%w: \"isDefault\" must be false on a generated dashboard", ErrGeneratedInvalid)
	}
	for i, w := range d.Widgets {
		if rules, ok := generatedResourceRules[w.ResourceType]; ok {
			if err := checkGeneratedResourceWidget(i, w, rules); err != nil {
				return Dashboard{}, err
			}
			continue
		}
		if !caseTableResourceTypes[w.ResourceType] {
			return Dashboard{}, fmt.Errorf("%w: widgets[%d] (%q): resourceType %q is not one of case, service_request, engagement, security_report_analysis, announcement, time_card, incident, change_request, problem, account, project, case_feedback",
				ErrGeneratedInvalid, i, w.ID, w.ResourceType)
		}
	}
	d.ID = GeneratedPlaceholderID

	out, err := finalize([]sourced{{dashboard: d, source: generatedSource}}, true, nil, nil)
	if err != nil {
		msg := strings.TrimPrefix(err.Error(), fmt.Sprintf("dashboard definitions: %s (id %q): ", generatedSource, GeneratedPlaceholderID))
		return Dashboard{}, fmt.Errorf("%w: %s", ErrGeneratedInvalid, msg)
	}
	return out[0], nil
}

// unknownField reports the first key of value (decoded JSON) that the struct
// type t, or a struct nested in it, has no json field for. Struct fields are
// matched by their json tag names exactly; maps, interfaces and scalars are
// free-form and not looked into; a value of the wrong JSON kind is left for
// json.Unmarshal to reject. Keys are visited in sorted order so the error is
// the same every time.
func unknownField(value any, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFields(t)
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			at := k
			if path != "" {
				at = path + "." + k
			}
			ft, known := fields[k]
			if !known {
				return fmt.Errorf("%w: unknown field %q", ErrGeneratedInvalid, at)
			}
			if err := unknownField(obj[k], ft, at); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		arr, ok := value.([]any)
		if !ok {
			return nil
		}
		for i, elem := range arr {
			if err := unknownField(elem, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// jsonFields maps each json key of struct type t to its field's type.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = f.Type
	}
	return fields
}
