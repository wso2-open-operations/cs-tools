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

package domain

import "time"

// Public holiday countries, spelt as the leave app's employee locations and
// the calendar-events service spell them.
const (
	PublicHolidayCountrySriLanka = "Sri Lanka"
	PublicHolidayCountryIndia    = "India"
)

// Where a person's holiday country came from.
const (
	// PublicHolidayCountryFromEmployee: the employee service's location.
	PublicHolidayCountryFromEmployee = "EMPLOYEE"
	// PublicHolidayCountryFromTeam: no location known, so the team's
	// country (India for the India team, Sri Lanka otherwise).
	PublicHolidayCountryFromTeam = "TEAM"
)

// SearchSchedulePublicHolidaysRequest is the body of
// POST /team-schedule/public-holidays/search.
type SearchSchedulePublicHolidaysRequest struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	UserIDs []string `json:"userIds"`
}

// SchedulePublicHoliday is one person's public holiday on one day. It is not
// leave and moves no shift: the roster only shades the day.
type SchedulePublicHoliday struct {
	UserID        string `json:"userId"`
	Date          string `json:"date"`
	Title         string `json:"title"`
	Country       string `json:"country"`
	CountrySource string `json:"countrySource"`
}

// SchedulePublicHolidaysResponse answers the search.
type SchedulePublicHolidaysResponse struct {
	Holidays []SchedulePublicHoliday `json:"holidays"`
	Count    int                     `json:"count"`
}

// SchedulePerson is a Team Schedule member as the holiday lookup needs them.
type SchedulePerson struct {
	UserID   string
	Email    string
	TeamKeys []string
	// Location is the stored employee-service location, "" when unknown.
	Location string
}

// PublicHolidayDay is one stored holiday row.
type PublicHolidayDay struct {
	Country string
	Date    string
	Title   string
}

// PublicHolidayRefreshResult reports one refresh run.
type PublicHolidayRefreshResult struct {
	CalendarConfigured  bool      `json:"calendarConfigured"`
	EmployeesConfigured bool      `json:"employeesConfigured"`
	People              int       `json:"people"`
	LocationsUpdated    int       `json:"locationsUpdated"`
	LocationsFailed     int       `json:"locationsFailed"`
	Countries           []string  `json:"countries"`
	Years               []int     `json:"years"`
	HolidaysStored      int       `json:"holidaysStored"`
	CountryYearsFailed  int       `json:"countryYearsFailed"`
	CountryYearsEmpty   int       `json:"countryYearsEmpty"`
	RanAt               time.Time `json:"ranAt"`
	// Tags is what placing the Public holiday tags did after the refresh,
	// nil when it was not run or failed (logged).
	Tags *HolidayTagPlacementResult `json:"tags,omitempty"`
}
