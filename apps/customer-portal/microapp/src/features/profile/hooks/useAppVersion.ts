import { useEffect, useState } from "react";

import { getVersion } from "@src/bridge";

export function useAppVersion() {
  const [version, setVersion] = useState<string | undefined>(undefined);
  useEffect(() => {
    getVersion((v) => setVersion(v));
  }, []);
  return version;
}
