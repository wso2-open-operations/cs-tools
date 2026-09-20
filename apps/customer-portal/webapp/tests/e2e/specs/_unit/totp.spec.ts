import { test, expect } from "@playwright/test";
import { generateTotp, millisUntilNextTotp } from "../../auth/totp";

test("rfc6238 vectors", () => {
  const seed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ";
  expect(generateTotp(seed, 59_000)).toBe("287082");
  expect(generateTotp(seed, 1111111109_000)).toBe("081804");
  expect(generateTotp(seed, 1234567890_000)).toBe("005924");
  expect(generateTotp(seed, 2000000000_000)).toBe("279037");
  expect(millisUntilNextTotp(59_000)).toBe(1000);
  // Spaces and case, as authenticator enrolment screens display a seed.
  // Deliberately an arbitrary value — never a real account's seed, which would
  // put a live second factor in a tracked file.
  expect(generateTotp("jbsw y3dp ehpk 3pxp", 59_000)).toBe(
    generateTotp("JBSWY3DPEHPK3PXP", 59_000),
  );
  expect(() => generateTotp("not-base32-1!", 0)).toThrow(/not valid base32/);
});
