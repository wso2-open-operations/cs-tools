# Portal banners — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

A portal banner is a short notice shown at the top of the customer portal, the CSM portal,
or both: a planned maintenance window, a known issue, a new feature. CSM admins write
them in the CSM portal's Settings page and decide where and when each one shows.

The service stores the banners, decides which ones each portal may be given, and keeps their
order. The portals draw them.

## 2. Who is involved

| Who | What they do |
|---|---|
| CSM admin | Writes, orders, publishes and unpublishes banners, through the CSM portal. |
| CS engineer | Sees the banners meant for the CSM portal. |
| Customer user | Sees the banners meant for the customer portal. |
| The CSM portal backend | The only caller allowed to change banners (section 5). |

## 3. Banner states and audiences

A banner is in exactly one of three states:

- **Draft** — being written; shown nowhere.
- **Published** — eligible to be shown.
- **Unpublished** — taken down; shown nowhere, kept for later.

Each banner also has two independent switches: **visible in the customer portal** and
**visible in the CSM portal**. A banner may be shown in one portal, both, or neither.

**CUS-101** The service shall treat a banner as showable in a portal only when it is
**published** and the switch for that portal is on.
*Check:* a published banner with only the customer switch on appears for customers and not in
the CSM portal; a draft with both switches on appears in neither.

## 4. Showing banners

**CUS-110** The service shall let the customer portal read the banners meant for customers: those
that are published and switched on for the customer portal. This read is not limited to
internal callers.
*Check:* publish a banner for customers and read the customer list; it is there.

**CUS-111** The service shall let internal callers read the banners meant for the CSM portal:
those that are published and switched on for it. A caller who is not internal is refused.
*Check:* the same read from a customer token is refused.

**CUS-112** Each banner returned to a portal shall carry only what the portal needs to draw
it: its id, its position, its HTML, and its start and end times. The state and the audience
switches are not returned, because the route has already applied them.

**CUS-113** The service shall return banners in order, lowest position first.

**CUS-114** The service returns a banner's start and end times but does **not** hide a banner
that has not started or has expired. The portal decides whether the current time is inside
the window. (**Confirm**, see 8.1)

**CUS-115** Banner responses shall not be cached by browsers or gateways, so a banner that is
taken down disappears at once.

## 5. Managing banners

**CUS-120** The service shall let an admin list **every** banner, including drafts and
unpublished ones, with their state, audience switches, position, times, and who created and
last changed each.

**CUS-121** The service shall let an admin create a banner and shall record that person as its
creator and last editor.

**CUS-122** The service shall let an admin replace a banner's content, state, audience, times
and position, and shall record that person as the last editor. Changing a banner that does not
exist is refused as "not found".

**CUS-123** Listing, creating and changing banners shall work **only when called through the
CSM portal backend** on behalf of a signed-in person. Whether that person is a CSM admin is
decided once, by the portal backend, before it calls the service. The service does not check
admin roles a second time. Any other caller is refused, including one whose token carries an
admin role, and so is a call with no signed-in person.
*Check:* call the create route directly with an admin token that did not come through the
portal; refused.

**CUS-124** There is no delete. A banner is taken down by unpublishing it.

## 6. Rules for a banner

**CUS-130** The status shall be draft, published or unpublished. Anything else is refused.

**CUS-131** The position shall be zero or more.

**CUS-132** The HTML is required. Surrounding blanks are removed, and what is left must not be
empty and must be at most 100,000 characters.
*Check:* a banner with only spaces as HTML is refused; one longer than the limit is refused.

**CUS-133** When both a start and an end are given, the end shall be later than the start.
Either may be left empty.

**CUS-134** A banner shall be saved in one step with its position, so a refused save leaves
the existing banners and their order untouched.

## 7. Order

Banners that are not unpublished form one stack, numbered 0, 1, 2 and so on without gaps.
Unpublished banners sit after the stack.

**CUS-140** Saving a banner that is not unpublished shall place it at the requested position,
and every banner at that position or lower shall move down by one.
*Check:* with banners at 0, 1, 2, saving a new one at 1 makes the old 1 and 2 become 2 and 3.

**CUS-141** A requested position below zero is refused, as CUS-131 requires. A requested
position beyond the end is treated as the last place of the stack. The position actually
assigned is returned and may differ from the one asked for.

**CUS-142** Saving a banner as unpublished shall move it behind the whole stack, and the
banners it leaves behind shall close up so there is no gap.

**CUS-143** Two admins saving at the same time shall be handled one after the other, so
positions never collide or skip.

**CUS-144** When the HTML marks its outermost element as a banner (it carries `data-banner="1"`),
the service shall write the banner's own id into that element on every save, replacing any id
already there, so closing or editing a banner always refers to the stored id. HTML without
that marker is stored as sent.

The close button is part of the HTML the admin writes; the service does not add one.

## 8. Open questions

1. **Start and end times are not enforced here.** A published banner whose window has not
   opened, or has closed, is still returned. Both portals must apply the window themselves.
   Is that the intended split?
2. **The HTML is stored and served exactly as written.** The service does not clean it. Safety
   rests entirely on the admin gate. Is that acceptable, or should content be sanitised?
3. **The customer list needs no sign-in at this service.** It answers any caller that reaches
   it, and relies on the gateway in front. Confirm this is intended.
4. **No limit on how many banners exist** or how many may be showing at once.

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Rules, access, validation | `internal/service/portal_banner_service.go` |
| Storage, listing, ordering | `internal/repository/portal_banner_repo.go`, `internal/repository/portal_banner_order.go` |
| Banner types, id stamping | `internal/domain/portal_banner.go` |
| Routes | `GET /banners`, `GET /portal-banners`, `GET /portal-banners/manage`, `POST /portal-banners`, `PUT /portal-banners/{id}` |
| Table | migration `0245` |
| Admin gate | CSM portal backend (checks the admin role before calling) |
| Tests | `portal_banner_service_test.go` |
