# Project memberships and onboarding — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 15.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related:** [users.md](users.md), [access-control.md](access-control.md). Memberships that
originate in Salesforce and arrive as events are covered by the Salesforce ingest
requirements, not here.

---

## 1. Purpose

A customer or partner gets access to a project by being a **member** of it (a project
contact). This document covers how a person is invited, how they become registered when
they first sign in, how their roles are changed or removed, and how each step of that
journey is tracked. Memberships are kept in two places, the customer database and
Salesforce, and every write here updates both, with one exception: when Salesforce is
unavailable and the safeguard in section 12 is on, a first invitation is saved in the customer
database first and handed to Salesforce afterwards, and a removal may then be made in the customer
database only (USR-280, USR-284).

## 2. Who is involved

| Who | What they do |
|---|---|
| Customer account admin | Invites, re-roles, removes and re-invites people on their own projects (through the customer portal). |
| CS engineer (through the CSM portal) | Does the same on any project. |
| The invited person | Is registered automatically on their first sign-in. |
| Internal services | Record and read onboarding steps. |

## 3. Terms

A membership is in one of four **states**:

| State | Meaning |
|---|---|
| Invited | The person has been invited and has not signed in yet. |
| Re-invited | A removed person has been invited again. |
| Registered | The person has signed in. Only this state gives access to the project. |
| Deactivated | The person has been removed from the project. |

A membership carries one or more **roles**, in the form Salesforce uses: *Portal user*,
*Security Contact*, *Lead* and *Admin*. A portal request may use only these four.

The roles decide the person's **project access group**: Portal user alone gives General
Access; Security Contact alone gives Security Only; both give Full Access; Lead also adds
the Lead User Group; Admin also adds the Admin group. (A Business Contact group exists but
can only be set in Salesforce.)

## 4. Becoming registered on sign-in

**USR-200** When a person's profile is loaded, the service shall register any of their
memberships that are still invited or re-invited. The call needs no input; it answers
"no content". The person is found by the email in their token.

**USR-201** Without a token the answer is "unauthorized"; an unreadable token is a bad
request.

**USR-202** A person with nothing to register is the common case. The service shall then
do nothing, call nobody, and log nothing.

**USR-203** Only memberships that exist in both Salesforce and the customer database (each
has its Salesforce id, and so does the linked contact) shall be registered. One missing an
id cannot be changed in Salesforce, so it is left out.

**USR-204** For each membership the service shall, in this order: clear the contact's
"locked out" marker in Salesforce; set the membership's state to registered in Salesforce;
then bring the customer database in line before answering. The order matters, because
Salesforce recomputes the state from the marker on every save and would otherwise turn the
change back to invited.

**USR-205** The service shall only change memberships that already exist. It shall never
create anything in Salesforce on this path.

**USR-206** Each attempt shall record a **registration** onboarding step as succeeded or
failed (with a shortened reason). A failure to record the step is logged and does not fail
the request.

**USR-207** If some memberships register and others fail, the call succeeds; the failed
ones are still invited and are retried at the next profile load. Only when every
membership fails is an error returned, with the first failure's own status.

**USR-208** This feature is off unless explicitly enabled. When off, or when Salesforce or
the membership ingest is not configured, the route does not exist (it answers "not
found") and nothing can write to Salesforce through it.

## 5. Who may manage a project's contacts

**USR-210** Inviting, checking an invitation, changing roles, removing and re-sending
shall be allowed for: internal callers on any project; and a customer or partner only on a
project they are **registered** on **and** who holds an account admin role (customer admin
or partner admin).

**USR-211** A person who can see the project but is not an account admin shall be refused
with "only an account admin can manage this project's contacts". A person with no access to
the project shall be told it was not found.

**USR-212** When a customer invites someone, the inviter shall always be the signed-in
customer, taken from their own token. A customer cannot name someone else as inviter.

**USR-213** These routes exist only when the feature is enabled, Postgres is the main data
source and the Salesforce connection is complete. Otherwise they answer "not found", so a
portal does not write one system without the other. The exception is the deferred-write safeguard
in section 12: when Salesforce is unavailable and that safeguard is on, a first invitation is
written to the customer database first (USR-280), and a removal may be local-only if its deferred
write failed (USR-284).

**USR-214** Writes for the same project and email shall be handled one at a time.

## 6. Inviting someone

**USR-220** The service shall invite a person to a project given their email, optional
first and last name, and one or more roles.

**USR-221** The email shall be trimmed, lower-cased, and look like an address (something
before and after one "@", no spaces). Roles shall be one of the four allowed roles,
ignoring case; duplicates are dropped; an unknown role is refused by name. At least one
role is required unless the invited contact is an **integration user** (a service account
that never signs in).

**USR-222** A new member starts as invited. Someone who was previously removed
(deactivated) is re-invited. Someone who is already a live member shall be refused as a
conflict ("already a contact on the project; change their roles instead").

**USR-223** If Salesforce itself records the new membership as registered straight away (the
contact has already signed in, so it is not locked out), the membership shall be stored as
registered rather than invited, and the invitation email still goes out.

**USR-224** The service shall find the person in Salesforce by their email (or by the
linked contact when re-inviting) and create the Salesforce contact only if none exists.
When no name is given it uses the part of the email before the "@" so the contact can be
created.

**USR-225** The invitation shall be refused before anything is written if the project,
its account, or an existing membership is missing its Salesforce link. The person is told
generically to contact WSO2 support; the cause is logged with ids, never with the email.
This is a conflict, not a bad request, because retrying will not help until the data is
fixed.

**USR-226** When both systems have been written, the service shall publish an
"invited" event so the notification service sends the invitation email. A failure to
publish is logged and recorded, and does not fail the invitation.

**USR-227** The answer shall give the project, the membership, the person's user record,
their email, the state and the roles.

**USR-228** Writes shall happen in this order: Salesforce first, then the customer
database, in one transaction. If Salesforce succeeds and the database then fails to save,
the caller is told "the change could not be saved; please try again", and the half-done
Salesforce write is recorded so someone can repair it. A retry is safe, because the service
always searches Salesforce before creating anything.

**USR-229** The person's cached profile shall be cleared after any membership write, so the
next read shows the new access.

## 7. Checking an invitation first (dry run)

**USR-230** The service shall let a caller check an invitation without sending it. The
answer is "valid" or a reason: *conflict* (already a member), *forbidden*, or *invalid*,
with the message a person would see. Refusals are answered normally, not as errors.

**USR-231** A valid answer shall include, when the person already has a Salesforce
contact, who they are: id, email, name, whether they are an admin or an integration user,
and their account's classification and whether it is a partner.

**USR-232** A person who was previously removed shall be reported as valid, with their
earlier state shown.

**USR-233** The same rules (section 8) are applied by the dry run as by the real invitation.

## 8. Who may be invited

These rules apply to every invitation, in this order, stopping at the first that fails.

**USR-240** The email shall not be on a public provider (five common free-mail
domains are refused outright, whatever the account allows).

**USR-241** Salesforce shall hold at most one contact for the address; more than one is
refused as a duplicate.

**USR-242** When a **customer** invites (an inviter is present): the inviter must have a
Salesforce contact with an account, and that account must own the project or be one of its
partners. Otherwise they are refused.

**USR-243** Also for a customer inviter: an existing contact from a *different*
organisation who still has a live membership on this project shall not be taken over.

**USR-244** The account whose allowed email domains apply is the inviter's account (when
there is an inviter), else the project's account. That account must have a list of allowed
domains; an empty list is refused with advice to contact the account manager.

**USR-245** The invited address's domain must be on that list, or on the list of any partner
of the project's account. Otherwise refused.

**USR-246** If an existing Salesforce contact belongs to an account that is neither the
project's nor one of its partners, the invitation shall be refused with a message that does
not confirm the person exists, so the check cannot be used to find out who is a contact.

**USR-247** When staff invite through the CSM portal (no inviter), the checks on the
person's address apply, and the allowed domains start from the project's account. The
staff person's authority is the CSM portal's to decide.

## 9. Changing roles

**USR-250** The service shall change the roles of an existing member. A person who is not a
member of the project is "not found".

**USR-251** Roles the portal manages (the four allowed) shall be replaced by the new list;
any other roles on the Salesforce record are kept. If the existing roles cannot be read,
the change is refused rather than overwriting them.

**USR-252** The membership keeps its current state. The change is written to Salesforce and
then to the database, like an invitation (USR-228).

**USR-253** A change is refused while the membership's Salesforce write is still pending
or has failed (see section 12).

## 10. Removing a member

**USR-260** The service shall remove a member by setting the membership to deactivated.
The roles and access groups are kept as they were; only the state changes. A person who is
not a member is "not found". The answer is "no content".

**USR-261** Someone removed can later be invited again, which makes the membership
re-invited (USR-222).

**USR-262** A removal is refused while the membership's Salesforce write is still pending
(see section 12).

## 11. Re-sending an invitation

**USR-270** The service shall re-send the invitation email for a member whose state is
invited or re-invited. Any other state is refused ("only a contact in state INVITED or
RE-INVITED can have their invitation re-sent").

**USR-271** An invitation sent in the last 5 minutes shall not be sent again. The caller is
told how long to wait. This stops a double-clicked button filling someone's mailbox.

**USR-272** The re-send is an event for the notification service; nothing is written to
Salesforce. If events are not configured the answer is "unavailable" with a message that
invitation emails are not configured. If the event cannot be queued the caller is told to
try again.

## 12. When Salesforce is unavailable

This is an optional safeguard, available only when its switch and event publishing are on.

**USR-280** If Salesforce is down or times out while a **first** invitation is being
written, the service shall save the invitation in the customer database, mark it "sync
pending", and queue the Salesforce half to be done later. The answer shows the membership
with that pending marker and no Salesforce ids.

**USR-281** A re-invitation, role change or removal is never deferred this way; it fails
as it would without the safeguard. A refusal from Salesforce (as opposed to an outage) is
never deferred either.

**USR-282** When Salesforce later completes the write, the service shall bring both systems
into line and send the invitation email, as a normal invitation would have.

**USR-283** While a membership's Salesforce write is pending, a role change, removal or
re-send shall be refused with "still being synced, please try again in a few minutes".

**USR-284** If the deferred write failed or was never handed on, role changes and re-sends
shall be refused with a message that nothing is coming, and removing the member is allowed:
it removes them in the customer database only and cancels the stuck request.

## 13. Effect on the person's account roles

**USR-290** Every invited contact (other than an integration user) shall hold the
**external** role, and one of **customer** or **partner** depending on whether their
Salesforce account is classified as a partner. If the classification is unknown the person
is treated as a customer and an existing partner role is not removed.

**USR-291** A person who is an Admin on any of their projects shall hold **customer admin**
(or **partner admin**); the role is removed when they are no longer an admin anywhere.
Exactly one of these two admin roles is held at a time.

**USR-292** An integration user shall hold none of these roles and shall have any admin role
removed.

**USR-293** Because a person's type comes from their roles (see users.md), these grants are
what make someone a customer.

## 14. Onboarding steps

An **onboarding step** is a small record of one stage of a membership's journey. There is
at most one record per membership and step.

| Step | Meaning |
|---|---|
| IDENTITY | The person's sign-in identity exists. |
| DATABASE | The membership is in the customer database. |
| EMAIL | The invitation email was sent. |
| REGISTRATION | The person's first sign-in registered the membership. |
| WELCOME_EMAIL | The welcome email was sent. |

A step has a status: succeeded, failed or skipped.

**USR-300** Onboarding steps shall be available to **internal callers only**; a customer is
refused, because the steps carry other people's emails and Salesforce ids.

**USR-301** An internal service shall record a step with: the membership's Salesforce id,
the step name, the status, an event type (up to 64 characters), the time of the event, and
the person's email. Step and status are accepted in any letter case; anything else is
refused with the list of valid values. The email is stored lower-cased.

**USR-302** Optionally the record carries the contact's Salesforce id, the project and the
project contact. The project and project contact must be valid identifiers.

**USR-303** Recording again for the same membership and step shall update the one record.
An **older** event shall never overwrite a newer one; the stored version only moves forward.
A retry that is stale is still noted, so it can be seen.

**USR-304** A failure message is stored shortened. When a step later succeeds or is skipped,
any earlier failure message is cleared.

**USR-305** The service shall return all steps of one membership, and search steps by
project, by a list of memberships and by status, with normal paging (default 20, up to 50).
Results are never null; no match is an empty list.

**USR-306** The person recorded as the author is the caller's label when given, else a
default label for the onboarding service.

## 15. Open questions for the product owner

1. **A customer admin can invite anyone on an allowed domain** and change roles with no
   second approval. Is a limit or approval wanted?
2. **Changing roles with an empty role list** is not refused by the service, whereas
   inviting without roles is. It is not clear that an update to no roles is meant to strip
   the person's portal roles. Should it be refused?
3. **Removing a member does not check that an admin remains.** A project can end with no
   admin contact.
4. **Half-written changes need a person to repair them.** If Salesforce is written but the
   database then fails, the case is only recorded (and the caller asked to retry). Who
   watches that record?
5. **Five public email domains are blocked by name**, a fixed list in the code. Should it
   be configurable or longer?
6. **Registration runs inside the profile load.** A person with an un-accepted invitation
   triggers live Salesforce calls and a database update before their profile loads, so a
   slow Salesforce slows that first sign-in. Is that acceptable?
7. **Deferred invitations (section 12) are optional and first-invitation-only**, so a
   re-invitation during a Salesforce outage simply fails.

## 16. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Registration on sign-in | `internal/service/membership_registration_service.go`, `internal/repository/membership_registration_repo.go`, `internal/handler/membership_registration_handler.go` |
| Invite, dry run, roles, remove, re-send | `internal/service/project_membership_write_service.go`, `internal/handler/project_membership_handler.go`, `internal/repository/project_membership_repo.go` |
| Who may be invited | `internal/service/invitation_validator.go` |
| Deferred Salesforce write | `internal/service/project_membership_deferred_write.go` |
| Roles, groups and states mapping | `internal/service/salesforce_membership_mapping.go` |
| Onboarding steps | `internal/service/onboarding_step_service.go`, `internal/repository/onboarding_step_repo.go`, `internal/handler/onboarding_step_handler.go` |
| Routes | `POST /users/me/memberships/register`; `POST /projects/{id}/contacts`, `.../contacts/validate`, `PATCH`/`DELETE .../contacts/{email}`, `POST .../contacts/{email}/resend-invitation`; `PUT /onboarding-steps/{membershipSfId}/{step}`, `GET /onboarding-steps/{membershipSfId}`, `POST /onboarding-steps/search` |
| Settings | `CSM_MIGRATION_MEMBERSHIP_REGISTRATION_ENABLED`, `CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED`, `CSM_MIGRATION_PORTAL_WRITES_ENABLED`, `SALESFORCE_DEFERRED_WRITES_ENABLED`, the Salesforce connection settings, the event hub settings |
| Tests | `membership_registration_*_test.go`, `project_membership_*_test.go`, `invitation_validator_test.go`, `onboarding_step_service_test.go` |
