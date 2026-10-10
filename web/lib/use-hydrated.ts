import { useSyncExternalStore } from "react";

const subscribe = () => () => {};

// False during SSR and the hydration pass, true afterwards — gates browser-only reads like localStorage.
export function useHydrated() {
  return useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
}
