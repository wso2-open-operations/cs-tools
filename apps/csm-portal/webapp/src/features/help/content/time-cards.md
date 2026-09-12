# Time cards

Time cards track the time engineers spend working a case. Every card is
logged against a specific case, is submitted immediately, and then goes
through a lead's approval before it counts as final.

## Logging time

Time is always logged from a case, not from the Time cards page:

1. Open the case and go to its **Time tracking** tab.
2. Select **Log time**.
3. Fill in the entry:
   - **Date**: the day the work was actually done (it can be backdated).
   - **Time breakdown**: minutes split across five fixed activities:
     Analysis and debugging, Reproduce, Setting up, Providing solution, and
     Answering. The total across all five is the card's logged time.
   - **Issue complexity**: N/A, Low, Medium, or High.
   - **Billable**: whether the time is billable to the customer. This is
     locked to non-billable for the most severe cases and can't be changed
     on those.
   - **Work log comment**: a short note on what was worked on. Required.
   - **Approver**: the team lead who will review the card. Search by name
     or email; you can't pick yourself.
4. Select **Submit for review**.

A card is created already in the **Submitted** state; there's no draft or
save-for-later step.

While a card is still **Submitted**, its own submitter can edit or delete it
from the case's Time tracking tab or from the **My time sheets** tab. Once a
lead has made a decision, the card is locked and can no longer be edited or
deleted.

## Approval

A submitted card is reviewed by the approver chosen when it was logged. A
lead can never approve or reject their own time card, even if they are also
an approver.

To decide a card, the lead opens **Review** (from the case's Time tracking
tab, or from the **Approvals** tab on the Time cards page) and selects
**Accept** or **Reject**. A comment is optional when accepting but required
when rejecting: it's the only record of why a card was rejected.

Cards move through these states:

- **Submitted**: logged and awaiting a decision.
- **Approved**: accepted by the lead.
- **Rejected**: declined by the lead, with a comment explaining why.

On the **Approvals** tab, a lead can also select several submitted cards at
once and approve them together with **Approve**.

## Finding time cards

The Time cards page has three tabs:

- **My time sheets**: only your own cards.
- **All**: everyone's cards, for visibility only. No approve, reject, edit,
  or delete actions are available here, even on your own cards.
- **Approvals**: visible only to approvers and admins. Shows cards awaiting
  a decision from you.

Each tab can be filtered by project, work item (case number), state, and
work date range; the **All** and **Approvals** tabs can also be filtered by
engineer. Use **Export CSV** to download whatever cards are currently
loaded on a tab.
