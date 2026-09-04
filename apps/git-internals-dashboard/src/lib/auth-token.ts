// Module-level bridge so the plain-fetch API client doesn't need React context.
type TokenGetter = () => Promise<string>;
let getter: TokenGetter | null = null;

export function setAccessTokenGetter(fn: TokenGetter | null) {
  getter = fn;
}

export async function getAccessToken(): Promise<string | null> {
  if (!getter) return null;
  try {
    return await getter();
  } catch {
    return null;
  }
}
