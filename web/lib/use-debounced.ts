import { useEffect, useState } from "react";

/** Returns `value` after it has been stable for `ms`; `onCommit` runs right before the new value is returned (reset list state there). */
export function useDebounced(value: string, onCommit: () => void, ms = 300) {
  const [committed, setCommitted] = useState(value);
  useEffect(() => {
    if (value === committed) return;
    const timer = setTimeout(() => {
      onCommit();
      setCommitted(value);
    }, ms);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, committed, ms]);
  return committed;
}
