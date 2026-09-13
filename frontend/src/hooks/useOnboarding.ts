import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  OnboardingTurnRequest,
  OnboardingTurnResponse,
} from "../lib/types";

/**
 * Starts a new interview by asking the agent's first question, automatically,
 * once per real mount of the onboarding screen.
 *
 * This is a useQuery, not a useMutation, even though it POSTs and has a real
 * side effect (a new session_id on the server) — because it has to survive
 * StrictMode's dev-mode double-invoke of effects, and useQuery is the
 * primitive built for that; useMutation is not. A useMutation fired
 * imperatively from a useEffect left the mutation's pending state stuck
 * forever under StrictMode: the request completed, but the component never
 * saw it resolve. useQuery's own dedup logic is what "run this once on
 * mount" actually needs.
 *
 * instanceKey scopes the query to one real mount: state (and therefore this
 * key) survives StrictMode's double-invoke but not a genuine unmount, so
 * revisiting the screen starts a genuinely new interview rather than
 * resuming a cached one. staleTime: Infinity + retry: false +
 * refetchOnWindowFocus: false all exist for the same reason — this call
 * creates new server state, so it must never silently re-fire on its own.
 */
export function useStartOnboarding(instanceKey: string) {
  return useQuery({
    queryKey: ["onboarding", "start", instanceKey],
    queryFn: () =>
      apiFetch<OnboardingTurnResponse>("/onboarding/turns", {
        method: "POST",
        body: JSON.stringify({}),
      }),
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

/**
 * One turn of the onboarding interview after the first — triggered by an
 * explicit Send click, which is what useMutation is for. Unlike
 * useStartOnboarding, this one is never invoked from an effect, so it isn't
 * exposed to the StrictMode issue above at all.
 */
export function useSendOnboardingTurn() {
  return useMutation({
    mutationFn: (turn: OnboardingTurnRequest) =>
      apiFetch<OnboardingTurnResponse>("/onboarding/turns", {
        method: "POST",
        body: JSON.stringify(turn),
      }),
  });
}
