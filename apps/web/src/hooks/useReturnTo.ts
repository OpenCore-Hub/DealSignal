import { useLocation } from "react-router";

export interface ReturnToState {
  returnTo?: string;
  returnLabel?: string;
}

interface UseReturnToResult {
  to: string;
  label: string;
}

export function useReturnTo(defaultTo: string, defaultLabel: string): UseReturnToResult {
  const location = useLocation();
  const state = (location.state as ReturnToState | undefined) ?? {};
  return {
    to: state.returnTo && state.returnTo.trim() !== "" ? state.returnTo : defaultTo,
    label: state.returnLabel && state.returnLabel.trim() !== "" ? state.returnLabel : defaultLabel,
  };
}
