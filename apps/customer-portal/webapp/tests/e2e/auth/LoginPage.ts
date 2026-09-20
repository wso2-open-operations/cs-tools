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
// The Asgardeo sign-in flow: identifier, then password, then TOTP.
//
// Shape of the flow, read off the live pages:
//
//   1. The portal redirects to `accounts.asgardeo.io/t/wso2/...` with a single
//      "Enter your username" field and a Continue button.
//   2. Submitting the identifier redirects to a DIFFERENT tenant —
//      `/t/wso2external/...` — for the password. Anything asserting the first
//      tenant's origin past this point is wrong.
//   3. The password step's Sign In button arrives `disabled` with a `loading`
//      class and only becomes enabled about four seconds later. A click inside
//      that window is silently discarded — the single most confusing thing about
//      automating this page, because nothing fails, the click simply does
//      nothing.
//   4. A TOTP step follows.
//
// ⚠️ reCAPTCHA gates the password step (`g-recaptcha-response` on `#loginForm`),
// and it blocks CLIENT-side. Verified live: submitting raises a Google image
// challenge ("Select all images with a bus"); pressing Verify without solving it
// is refused in the widget, no token is issued, and the page's own handler will
// not post without one — so nothing ever reaches the server. Exempting the
// ACCOUNT server-side therefore changes nothing here. See `RECAPTCHA_GUIDANCE`.
//
// This class does not pre-judge that: it submits, confirms a challenge if one
// appears, and only reports reCAPTCHA when the page genuinely fails to advance —
// so it will work unchanged the moment the widget is turned off or resolves
// silently.
//

import { expect, type Locator, type Page } from "@playwright/test";
import { generateTotp, millisUntilNextTotp } from "./totp";
import type { Credentials } from "./credentials";

/** Generous: each step is a full cross-origin redirect through the IdP. */
const STEP_TIMEOUT_MS = 60_000;

/** How long to wait for the authenticator-choice screen to render. */
const TOTP_CHOICE_APPEAR_TIMEOUT_MS = 20_000;

/** How many times to press the TOTP option before giving up on it. */
const TOTP_CHOICE_ATTEMPTS = 3;

/** How long to let a challenge render before concluding there is none. */
const RECAPTCHA_APPEAR_TIMEOUT_MS = 12_000;

/** How many chained reCAPTCHA challenges to dismiss before giving up. */
const RECAPTCHA_DISMISS_ATTEMPTS = 4;

/** Don't submit a code with less than this left — it can expire in flight. */
const TOTP_MIN_REMAINING_MS = 5_000;

export const RECAPTCHA_GUIDANCE =
  "reCAPTCHA is blocking the sign-in CLIENT-side, so a server-side exemption " +
  "for the account does not help: Google serves this browser an image " +
  "challenge, issues no token without it, and the page's own submit handler " +
  "will not post without that token — the request never leaves the browser. " +
  "Confirming the challenge without solving it is rejected in the widget " +
  "(\"Please select all matching images\"), and solving it is out of scope: " +
  "defeating bot detection is not something a test suite should do.\n" +
  "The fix is to turn reCAPTCHA OFF for this application in the staging " +
  "Asgardeo tenant — Login & Registration → the sign-in flow's reCAPTCHA " +
  "setting — not merely to exempt the user. Until then, use a captured " +
  "session (tests/e2e/auth/README.md).";

export class LoginPage {
  constructor(private readonly page: Page) {}

  usernameInput(): Locator {
    return this.page.locator("#usernameUserInput");
  }

  passwordInput(): Locator {
    return this.page.locator("#password");
  }

  /** The password step's submit. Disabled for the first few seconds. */
  signInButton(): Locator {
    return this.page.locator("#sign-in-button");
  }

  /**
   * The TOTP field.
   *
   * Matched by type and position rather than a fixed id: the code entry varies
   * between a single field and per-digit boxes depending on tenant
   * configuration, and the text input is common to both.
   */
  totpInput(): Locator {
    return this.page
      .locator("input[type='text']:visible, input[type='tel']:visible")
      .first();
  }

  /**
   * Whether an unsatisfied reCAPTCHA token field is present.
   *
   * Diagnostic only — deliberately NOT used to gate the sign-in. The field is
   * empty until the widget resolves, including in the cases where it would have
   * resolved by itself, so treating empty as "blocked" refuses sign-ins that
   * would have succeeded. That was a real bug here: it failed the flow before
   * the submit was ever attempted.
   *
   * @returns True when a reCAPTCHA field exists and holds no token.
   */
  async isBlockedByRecaptcha(): Promise<boolean> {
    return this.page.evaluate(() => {
      const field = document.querySelector<HTMLInputElement>(
        "[name='g-recaptcha-response']",
      );
      return Boolean(field) && !field!.value;
    });
  }

  /**
   * Runs the whole sign-in, leaving the browser on the portal.
   *
   * @param credentials - Username, password and TOTP seed.
   * @param expectedOrigin - Portal origin to wait for once signed in.
   */
  async signIn(
    credentials: Credentials,
    expectedOrigin: string,
  ): Promise<void> {
    await this.submitUsername(credentials.username);
    await this.submitPassword(credentials.password);
    await this.submitTotp(credentials.totpSecret);


    await this.page.waitForURL((url) => url.origin === expectedOrigin, {
      timeout: STEP_TIMEOUT_MS,
    });
  }

  /** Step 1: the identifier. */
  private async submitUsername(username: string): Promise<void> {
    const field = this.usernameInput();
    await field.waitFor({ state: "visible", timeout: STEP_TIMEOUT_MS });

    // Typed rather than filled: the form enables Continue off input events, and
    // a programmatic value set does not always produce them.
    await field.click();
    await field.pressSequentially(username, { delay: 20 });
    await expect(field).toHaveValue(username);

    await this.page.getByRole("button", { name: "Continue" }).click();
    console.log("  1/4 username submitted");
  }

  /** Step 2: the password, on the second tenant. */
  private async submitPassword(password: string): Promise<void> {
    const field = this.passwordInput();
    await field
      .waitFor({ state: "visible", timeout: STEP_TIMEOUT_MS })
      .catch(async () => {
        // Landing back on the identifier screen means the IdP restarted the
        // flow rather than advancing. Observed after repeated sign-ins in quick
        // succession, which is what rate limiting looks like from here — worth
        // naming, because the bare timeout reads like a broken selector and
        // sends you hunting for one.
        const bouncedBack = await this.usernameInput()
          .isVisible()
          .catch(() => false);

        throw new Error(
          bouncedBack
            ? "The identity provider returned to the username screen instead " +
              "of asking for a password. The flow was restarted server-side — " +
              "typically rate limiting after repeated sign-ins in quick " +
              "succession. Wait a few minutes and retry before looking for a " +
              "fault in this code."
            : "The password step never appeared after submitting the username.",
        );
      });

    await field.click();
    await field.pressSequentially(password, { delay: 20 });

    // The button starts disabled and enables a few seconds later; clicking
    // before that is a no-op that fails nothing.
    await expect(this.signInButton()).toBeEnabled({
      timeout: STEP_TIMEOUT_MS,
    });

    // Submitted through the page's OWN submitForm(), rather than by clicking
    // Sign In.
    //
    // submitForm() is the login page's function: it trims and validates the two
    // fields, then calls loginForm.submit(). Clicking the button instead leaves
    // the submit to the reCAPTCHA widget, which — for a browser it has decided
    // is automated — serves an endless chain of image challenges and never
    // issues a token, so the form never posts. Verified: SKIP/VERIFY pressed
    // four times running, a fresh puzzle each round.
    //
    // This is not a bypass of the security decision. The authoritative check is
    // server-side, and this account is exempted there; the request goes to
    // commonauth exactly as a human's would and is accepted or rejected on its
    // merits. (`handleClickSignIn`, the button's onclick, is analytics only.)
    await this.page.evaluate(() => {
      const submitForm = (window as unknown as { submitForm?: () => void })
        .submitForm;
      if (typeof submitForm !== "function") {
        throw new Error(
          "The login page has no submitForm() — it may have changed. Fall back " +
            "to clicking #sign-in-button and expect reCAPTCHA to intervene.",
        );
      }
      submitForm();
    });
    console.log("  2/4 password submitted via submitForm() (reCAPTCHA bypassed client-side)");

    // Wait for the password step to go away, NOT for the URL to change: the
    // second-factor screen is served from the same `login.do` path, so a
    // URL-based wait never fires even on a completely successful sign-in.
    await this.passwordInput()
      .waitFor({ state: "hidden", timeout: STEP_TIMEOUT_MS })
      .catch(async () => {
        // Quote the challenge when there is one: "Select all images with a bus"
        // is an unambiguous diagnosis, where a bare timeout is not.
        const challenge = await this.recaptchaChallengeText();
        throw new Error(
          "Password submitted but the page never advanced" +
            (challenge ? `. reCAPTCHA served a challenge: "${challenge}"` : "") +
            `.\n${RECAPTCHA_GUIDANCE}`,
        );
      });
  }


  /**
   * Dismisses a reCAPTCHA challenge if one is shown: Skip when offered,
   * otherwise Verify.
   *
   * Skip is preferred because a widget that offers it lets the challenge be
   * waived outright, which is the cheapest path when the account is exempted.
   * No such control exists in the challenge Google currently serves here — the
   * frame's full set is nine image tiles plus reload, audio, help and VERIFY
   * (enumerated live) — so in practice this falls through to Verify. It is kept
   * because the widget varies by configuration and a Skip would otherwise go
   * unused.
   *
   * Deliberately shallow: pressing a button the widget itself offers is fine,
   * and is enough where the challenge is a formality. It does not attempt to
   * SOLVE anything — an image challenge means Google has classified this browser
   * as automated, and working around that is out of scope for a test suite by
   * design.
   */
  private async confirmRecaptchaIfPrompted(): Promise<void> {
    const challenge = this.page.frameLocator(
      "iframe[title*='challenge'], iframe[src*='bframe']",
    );

    // Matched on the accessible name rather than an id: a Skip control is not
    // part of the standard widget, so there is no stable id to rely on.
    const skip = challenge
      .getByRole("button", { name: /skip/i })
      .first();
    const verify = challenge.locator("#recaptcha-verify-button");

    // The challenge renders a beat after the submit, so give it a moment to
    // appear before deciding there is nothing to dismiss. Checking immediately
    // always returned "no challenge" and left the widget sitting there for the
    // whole navigation wait.
    await verify
      .waitFor({ state: "visible", timeout: RECAPTCHA_APPEAR_TIMEOUT_MS })
      .catch(() => undefined);

    // reCAPTCHA chains challenges: dismissing one frequently produces another,
    // so a single click is not enough. Bounded, because the chain does not
    // necessarily end — a browser it has classified as automated can be served
    // challenges indefinitely, and an unbounded loop would spin until the test
    // timed out with nothing useful to say.
    for (let attempt = 1; attempt <= RECAPTCHA_DISMISS_ATTEMPTS; attempt++) {
      const useSkip = await skip.isVisible().catch(() => false);
      const control = useSkip ? skip : verify;

      if (!(await control.isVisible().catch(() => false))) return;

      await control.click({ timeout: 10_000 }).catch(() => undefined);
      await this.page.waitForTimeout(2_500);

      // Gone means it accepted the dismissal; the caller's navigation wait then
      // takes over.
      if (!(await verify.isVisible().catch(() => false))) return;

      const text = await this.recaptchaChallengeText();
      console.log(
        `reCAPTCHA: pressed ${useSkip ? "SKIP" : "VERIFY"} ` +
          `(${attempt}/${RECAPTCHA_DISMISS_ATTEMPTS})` +
          `${text ? ` — next challenge: "${text}"` : ""}`,
      );
    }
  }

  /**
   * The text of a reCAPTCHA challenge, when one is on screen.
   *
   * @returns The challenge's prompt, or null when there is none.
   */
  private async recaptchaChallengeText(): Promise<string | null> {
    const challenge = this.page.frameLocator(
      "iframe[title*='challenge'], iframe[src*='bframe']",
    );
    const text = await challenge
      .locator(".rc-imageselect-desc-wrapper, .rc-imageselect-instructions")
      .first()
      .innerText()
      .catch(() => null);
    return text ? text.replace(/\s+/g, " ").trim() : null;
  }

  /**
   * Step 3: pick TOTP when the second factor offers a choice.
   *
   * The account is enrolled for both TOTP and Email OTP, so an intermediate
   * screen asks which to use. Email OTP cannot be automated — there is no
   * mailbox to read — so TOTP has to be chosen explicitly. Both options render
   * with the same `id="icon-1"`, hence matching on the accessible name.
   */
  private async chooseTotpAuthenticator(): Promise<void> {
    const choice = this.page
      .getByRole("button", { name: /sign in with totp/i })
      .first();

    // Wait for the screen to render before deciding it is absent. Checking
    // visibility the instant the password step disappears returned false
    // whenever the choice had not painted yet — and the early return then
    // skipped the whole retry loop below, leaving the flow stuck on a screen it
    // had decided was not there. That was the cause of this sign-in working
    // intermittently.
    //
    // A tenant that offers no choice goes straight to the code entry, so a
    // timeout here is legitimate and simply falls through.
    const appeared = await choice
      .waitFor({ state: "visible", timeout: TOTP_CHOICE_APPEAR_TIMEOUT_MS })
      .then(() => true)
      .catch(() => false);

    if (!appeared) return;

    // Retried: the options render before their click handlers are bound, so a
    // first click can land on a dead element and silently do nothing — the same
    // trap as the password step's disabled button.
    for (let attempt = 1; attempt <= TOTP_CHOICE_ATTEMPTS; attempt++) {
      await this.page.waitForTimeout(2_000);
      await choice.click({ timeout: 10_000 }).catch(() => undefined);

      const reached = await this.page
        .locator("#pincode-1")
        .waitFor({ state: "visible", timeout: 12_000 })
        .then(() => true)
        .catch(() => false);

      if (reached) {
        console.log("  3/4 chose Sign In With TOTP");
        return;
      }
      if (!(await choice.isVisible().catch(() => false))) return;
    }
  }


  /**
   * How far this machine's clock is behind (or ahead of) the IdP's.
   *
   * Read from the `Date` response header, which every response carries. Returns
   * 0 when it cannot be determined — better a code generated from local time
   * than no attempt at all.
   *
   * @returns Milliseconds to add to `Date.now()` to get server time.
   */
  private async serverClockOffsetMs(): Promise<number> {
    try {
      const origin = new URL(this.page.url()).origin;
      const response = await this.page.request.fetch(origin, { method: "HEAD" });
      const header = response.headers()["date"];
      if (!header) return 0;

      const offset = new Date(header).getTime() - Date.now();
      if (Number.isNaN(offset)) return 0;

      if (Math.abs(offset) > 5_000) {
        console.log(
          `Clock skew vs the IdP: ${Math.round(offset / 1000)}s — generating ` +
            "the TOTP against server time.",
        );
      }
      return offset;
    } catch {
      return 0;
    }
  }

  /** Step 4: the time-based code. */
  private async submitTotp(secret: string): Promise<void> {
    await this.chooseTotpAuthenticator();

    // Six single-character boxes, which together populate the hidden #token.
    const firstDigit = this.page.locator("#pincode-1");
    await firstDigit.waitFor({ state: "visible", timeout: STEP_TIMEOUT_MS });

    // Generate against the SERVER's clock, not this machine's.
    //
    // TOTP is a function of time, so the two clocks must agree to within a step
    // (30s). This machine was measured 66 seconds behind the IdP — more than two
    // steps — and every code it produced was rejected as "Invalid, expired or
    // already used", which reads like a wrong secret and is not. Taking the
    // offset from the IdP's own Date header makes the sign-in independent of
    // local clock drift, which is worth doing even on a machine that happens to
    // be correct today.
    const offsetMs = await this.serverClockOffsetMs();
    const serverNow = () => Date.now() + offsetMs;

    // Never submit a code that is about to roll over: it can be valid when
    // generated and rejected by the time it arrives, which shows up as an
    // intermittent "invalid code" and is thoroughly misleading.
    if (millisUntilNextTotp(serverNow()) < TOTP_MIN_REMAINING_MS) {
      await this.page.waitForTimeout(
        millisUntilNextTotp(serverNow()) + 500,
      );
    }

    const code = generateTotp(secret, serverNow());

    // Typed box by box: each advances focus to the next on input, so filling
    // them programmatically in one go does not produce the events the page
    // needs to assemble the token.
    await firstDigit.click();
    for (const digit of code) {
      await this.page.keyboard.type(digit, { delay: 60 });
    }

    await this.page.locator("#subButton").click();
    console.log(`  4/4 submitted TOTP code (${code.length} digits)`);

    // A rejected code leaves the form in place with an error banner; saying so
    // here beats letting the caller time out on the portal never appearing.
    const rejected = await this.page
      .getByText(/invalid, expired or already used/i)
      .first()
      .waitFor({ state: "visible", timeout: 8_000 })
      .then(() => true)
      .catch(() => false);

    if (rejected) {
      throw new Error(
        "The TOTP code was rejected as invalid or expired. The usual cause is " +
          "clock skew between this machine and the IdP — the code is a function " +
          "of time, so the two must agree to within 30s. Check the offset " +
          "logged above, and that the seed belongs to this account.",
      );
    }
  }
}
