import { useState } from "react";

export const usePopoverAnchor = () => {
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);

  const open = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setAnchor(event.currentTarget);
  };

  const close = () => setAnchor(null);

  return { anchor, isOpen: Boolean(anchor), open, close };
};
