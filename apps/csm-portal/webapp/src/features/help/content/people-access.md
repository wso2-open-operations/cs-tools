# People & project access

Almost every name in the portal (a case's creator, its assignee, a watcher, a comment
author, an attachment uploader) is a link to that person's profile. This section covers
what the profile page shows, and how to read its project-access table when you're chasing a
"why can't this customer see their case" question.

Looking someone up this way isn't restricted to admins: any signed-in CS engineer can open
any user's profile.

## Looking up a person

Click the name wherever it appears: on a case, a request, a comment, an attachment, a
dashboard widget row. The link only appears once the portal has resolved that name to a user
id, which is usually instant but occasionally lags a beat behind the rest of the row loading.

The profile page itself shows:

- **Header**: name, email, and whether the account is Internal (WSO2 staff) or Customer,
  plus a headline status chip. A locked-out account shows **Locked out** here instead of
  **Active**/**Inactive**: locked-out takes priority in the headline because the account is
  unusable either way, but both attributes are always shown separately below.
- **Overview**: username, email, timezone, phone (internal users), team (internal users),
  account status, locked-out state, and created/updated timestamps. For an external contact
  who isn't a wso2.com address, there's also an **External account** field showing whether a
  matching account exists and whether it's locked; this is a separate lock concept from the
  "Locked out" chip above, and it's a best-effort lookup: if it couldn't be checked, this
  field reads "Unavailable" rather than a false "Not found".
- **Permissions & assignments**: platform roles, and (internal users only) group
  memberships.
- **Accessible projects**: external contacts only; see below.

## Reading project access status

For an external (customer) contact, the profile adds an **Accessible projects** table: one
row per project this person has any contact record on, with the project name, its short key,
their roles on it, and an **Access** column that's the actual verdict.

Two banners can appear above the table before you even get to individual rows:

- If the account itself is **inactive**, an error banner says so up front: an inactive
  account can't access any project's cases no matter what the per-project rows say below it.
- If the person's external account is **locked**, a separate error banner flags that they
  can't sign in at all until it's unlocked.

Each row's **Access** chip is one of:

- **No access** (red): this project does not grant this person case access. The reason line
  underneath tells you which of two distinct problems it is, because the fix is different for
  each:
  - *No contact record is linked to this project for this user*: the invite to this project
    never completed; there's nothing to fix on the person's own record, the project side needs
    a contact added.
  - *Invited as `<address A>` but linked to a contact whose own address is `<address B>`.
    This project is invisible to both.* The project has a contact row, but it was invited
    under one email address while the linked contact's actual account uses a different
    address. Neither address sees this project: not the one the invite named, and not the one
    the person actually logs in with. This is the case worth specifically checking for when a
    customer insists "I should have access": the fix is usually correcting the invited
    address to match the account they actually use.
- **Invited** (amber): the project does grant case access, but this person hasn't completed
  registration yet, so they can't see it in practice until they do.
- **Has access** (green): the project grants case access and registration is complete. If
  the customer still can't see a case here, look at the account-level banners above (inactive,
  locked) rather than this row.

Above the table, a warning banner summarizes **"Blocked on N of M projects"** whenever at
least one row isn't a clean **Has access**: N is every row that's either **No access** or
**Invited**, so it's a quick read on how much of this person's project access is incomplete
before you scan the individual reasons.

A couple of caveats worth knowing:

- This table (and the External account field) is only populated for users sourced from the
  data source that carries group and project-contact records; a person profile from a data
  source that doesn't track project contacts will simply show no Accessible projects section
  worth of data, not an error.
- All three blocks (memberships, project access, external account) are best-effort: if the
  underlying lookup fails, the section renders empty or absent rather than failing the whole
  profile page.
