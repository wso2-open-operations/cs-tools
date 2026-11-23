# Case alert settings and public holidays — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 7.

**Audience:** product owners, CS leads, QA, operators, and engineers new to the service.

---

## 1. Purpose

Two small operational features that other teams depend on:

- **Case alert settings** let rota admins and heads keep the chat rooms, routing and timing
  of the paging ladders in the Team Schedule's Alert Settings tab, instead of in a file only
  engineers can change.
- **Public holidays** keep a local copy of each country's public holidays so the Team Schedule
  can show them against the people who observe them.

The detail of the Team Schedule itself is in [../specs/team-schedule.md](../specs/team-schedule.md). The
case alert settings are described there in full ("Case Alerts settings"); this document states
the requirements.

## 2. Who is involved

| Who | What they do |
|---|---|
| CRE rota admin, CRE head | Manage the CRE alert settings. |
| SRE rota admin | Manages the SRE alert settings. |
| SME rota admin | Manages the SME alert settings. |
| CS head | Manages the SRE and SME settings, and the CRE settings as head. |
| CSM admin or portal admin | Manages all three. |
| Team lead | Sees who gets alerts, not the settings. |
| The notification service | Reads the saved settings when it starts, and reports what it loaded. |

## 3. Case alert settings

The paging ladders belong to three families: **CRE**, **SRE** and **SME**. Each family has its
own settings, and one family's settings are never shown to or changed by someone who only
manages another.

**CUS-401** The service shall let an allowed person see the saved settings: **chat rooms**,
**group routes** (which assignment group goes to which team or rota), and **ladder values**
(the timing, teams, heads and triggers of each ladder). A person who may see nothing gets an
empty answer rather than a refusal, so the page can decide whether to show the section.

**CUS-402** Seeing a family's settings and changing them shall go together, and a person shall
only be sent the families they may change. Who may change which family is the table in
section 2.

**CUS-403** Only internal staff, signed in as a person, may use the settings. A service
credential is refused.

**CUS-404** A change by someone who may not change that family shall be refused with a message
naming who may.

**CUS-405** Every room, route and ladder-value change shall be recorded with the person who
made it.

### Chat rooms

**CUS-410** The service shall let an allowed person add or change a chat room. A room has a
label of 1 to 100 characters, belongs to a family, and may be switched on or off. If it is
linked to an assignment group, the label may default to that group's name.

**CUS-411** SRE and SME teams may have a room of their own; CRE has one shared room. An SME
room must name its team and has only its web address; the "teamless" and "channel" options are
for SRE team rooms. The named team must be one of that family's teams in the Team Schedule.

**CUS-412** A team's room may say how that team's alerts go out: chat, call, both, or log only.
It can never do more than the ladder allows: the ladder's own setting is the ceiling.

**CUS-413** A room's **webhook address** is write-only. The service shall accept only an address
for the chat service's own space webhook (secure, the chat service's host, a space path, with its
key and token), refuse anything else with a message saying how to copy it again, and show people
only a hint (the space and the last characters of the token). The address never appears in a
response, a log or the history of changes.

**CUS-414** The service shall let an allowed person delete a room.

**CUS-415** The service shall let an allowed person send a **test card** to a room's saved
address and record whether it was delivered or failed. A room with no saved address cannot be
tested. A room can be tested **once every two minutes**; sooner is refused as "too many
requests", with the seconds to wait.

### Group routes

**CUS-420** The service shall let an allowed person route an assignment group to a team (or, for
SRE, a default group to a rota). A route names its group (up to 255 characters, and the name of
a team the CSM portal knows for that family) and **exactly one** of a team or a rota.

**CUS-421** An SRE route may not take a CRE team's incidents. Only SRE groups can go to a
default rota, and the rota must be an active SRE rota. The team must be one of that family's
teams in the Team Schedule.

**CUS-422** The service shall let an allowed person delete a route.

### Ladder values

**CUS-430** The service shall let an allowed person save ladder values: each is checked against
its own limits (for example, a time interval within a set range), and the set is saved
**together or not at all**, so a refusal changes nothing.

**CUS-431** The service shall let an allowed person delete a saved value, which makes that value
fall back to the one in the notification service's own file.

### What the notification service uses

**CUS-440** The notification service shall be able to read the enabled rooms (with their
addresses opened), the enabled routes and the saved values. Only an internal **service
credential** may read this. A person's token is refused. If a list of allowed clients is
configured, only those clients may read it.

**CUS-441** A room whose saved address cannot be opened shall be left out of that read and
logged, so the ladder keeps using the file's room for it.

**CUS-442** The notification service shall report what it loaded (its mode: off, shadow or
database; the revision; and a summary of at most 32 KB), and the settings page shall show it
as "running" next to what is "saved". A report with an unknown mode or a negative revision is
refused.

## 4. Public holidays

**CUS-450** The service shall keep a local copy of public holidays by country and date, and the
Team Schedule shall read only that copy.

**CUS-451** A person's holidays shall be those of their own **country**: the one recorded for them
by the employee service, otherwise their team's country (India for the India team, Sri Lanka for
every other team). Each answer shows which country was used and where it came from.

**CUS-452** The service shall let an internal person ask for the holidays of a list of people over
a window. The list is required, has at most **500** people, and every id must be a valid user id.
The answer lists each person's holidays by date, then person.

**CUS-453** The service shall refresh the copy: it looks up each person's country, then reads the
current and next year's holidays for every country in use (Sri Lanka always, India when an India
team exists, and any country a person has recorded).

**CUS-454** A refresh shall be safe to repeat. If the calendar fails or returns nothing for a
country and year, the days already stored are kept. If a person's country cannot be looked up,
their stored country, or their team's, is kept.

**CUS-455** A refresh requested by route shall start in the background and be acknowledged at
once, saying it started and what the last finished run was. Only one refresh runs at a time;
asking for another while one is running is refused as a conflict. A background run is cut off after
ten minutes.

**CUS-456** A refresh may be requested only by an internal **service credential**, never by a
person.

**CUS-457** When the calendar service or the employee service is configured, the service shall
refresh daily on its own, starting two minutes after it starts. With neither configured, no
refresh runs and the Team Schedule shows only the holidays already stored.

**CUS-458** After a successful refresh, CRE members shall have the coming weeks' **Public
holiday** tag placed on each holiday. The tag takes the day off the timesheet and the member off
the rota until a lead removes it. (A holiday is not leave and moves no shift by itself.)

## 5. Health checks (moved)

The liveness and database health checks are an operator requirement and are described once, in
[platform-and-operations.md](platform-and-operations.md) (section 8, PLT-070 to PLT-074).

## 6. What is not covered

There is no feature in this service for muting or suppressing alerts. Alert **mute rules** are
not part of what the service does today.

## 7. Open questions

1. **Case alert settings check roles inside this service.** A CSM admin is recognised by a role
   in the person's token, matched against this service's own list of admin roles, and the other
   managers from stored roles. If that list is unset or differs from the portal's, a genuine
   admin is refused. The portal banners and the de-escalator list instead trust the CSM portal
   backend and the person it forwards. Should the alert settings follow the same rule?
2. **A saved webhook address is stored as plain text** in the database. The notes accept that
   anyone who can read the database can read the addresses. Is that acceptable?
3. **Holidays have no manual edit.** A wrong or missing holiday can only be fixed at the
   calendar source or by the refresh. Is that intended?
4. **Only two countries have a fallback.** A person with no recorded country and not on the
   India team is treated as Sri Lanka. Confirm that is right for people in other countries.

## 8. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Alert settings rules, access, validation | `internal/service/case_alert_settings_service.go`, `internal/service/case_alert_ladder_values.go`, `internal/service/case_alert_webhook.go` |
| Alert settings storage | `internal/repository/case_alert_settings_repo.go` |
| Alert settings routes | `GET/PUT/DELETE /team-schedule/case-alerts/settings/...` (rooms, webhook, test, routes, values, runtime, applied) |
| Alert settings tables | migrations `0233`, `0237`, `0238`, `0243`, `0244` |
| Public holidays | `internal/service/public_holiday_service.go`, `internal/holidays/`, `internal/service/holiday_availability_service.go` |
| Holiday routes | `POST /team-schedule/public-holidays/search`, `POST /team-schedule/public-holidays/refresh` |
| Holiday settings | `HOLIDAY_CALENDAR_*`, `HR_ENTITY_*` (set together or not at all) |
| Existing spec | [../specs/team-schedule.md](../specs/team-schedule.md) |
| Tests | `case_alert_*_test.go`, `public_holiday_service_test.go` |
