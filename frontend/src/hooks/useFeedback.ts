import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { CreateFeedbackRequest } from "../lib/types";

/** Sends one product-feedback report. Nothing reads these back in the app. */
export function useSubmitFeedback() {
  return useMutation({
    mutationFn: (req: CreateFeedbackRequest) =>
      apiFetch<{ id: string }>("/feedback", {
        method: "POST",
        body: JSON.stringify(req),
      }),
  });
}
