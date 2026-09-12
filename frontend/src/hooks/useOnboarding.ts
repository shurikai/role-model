import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  OnboardingTurnRequest,
  OnboardingTurnResponse,
} from "../lib/types";

/**
 * One turn of the onboarding interview (#117). Unlike the import batch hooks
 * in useImport.ts / useIntake.ts, there is nothing to poll: a turn is a
 * direct request/response, just a slower one, so this is a plain mutation
 * with no accompanying query.
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
