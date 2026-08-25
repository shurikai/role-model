import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  ApproveDraftResponse,
  ContributionDraft,
  CreateImportBatchResponse,
  DraftEditableField,
  ImportBatch,
} from "../lib/types";

export function useCreateImportBatch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rawText: string) =>
      apiFetch<CreateImportBatchResponse>("/import", {
        method: "POST",
        body: JSON.stringify({ raw_text: rawText }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["import"] });
    },
  });
}

/**
 * Poll only while the batch is still working.
 *
 * Today `POST /import` runs extraction and enrichment synchronously and does
 * not answer until they finish, so a batch is almost always `ready` or
 * `failed` on first read and this never fires. It is here because the states
 * exist in the schema and the handler is the sort of thing that moves to a
 * queue — when it does, the screen already waits correctly instead of showing
 * an empty draft list.
 */
const WORKING_STATUSES = new Set(["pending", "extracting", "enriching"]);

export function useImportBatch(batchID: string | undefined) {
  return useQuery({
    queryKey: ["import", batchID],
    queryFn: () => apiFetch<ImportBatch>(`/import/${batchID}`),
    enabled: !!batchID,
    refetchInterval: (query) =>
      query.state.data && WORKING_STATUSES.has(query.state.data.status)
        ? 2000
        : false,
  });
}

export function useImportDrafts(batchID: string | undefined) {
  return useQuery({
    queryKey: ["import", batchID, "drafts"],
    queryFn: () => apiFetch<ContributionDraft[]>(`/import/${batchID}/drafts`),
    enabled: !!batchID,
  });
}

/**
 * Only the fields that actually changed are sent. The handler rejects an
 * unknown or non-editable key with a 400 that fails the whole request, so a
 * caller must never include `employer_name` or `position_title`; the
 * `DraftEditableField` key type is what keeps that honest at compile time.
 */
export function useUpdateDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      draftID,
      changes,
    }: {
      draftID: string;
      changes: Partial<Record<DraftEditableField, string | null>>;
    }) =>
      apiFetch<ContributionDraft>(`/import/drafts/${draftID}`, {
        method: "PUT",
        body: JSON.stringify(changes),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["import", batchID, "drafts"],
      });
    },
  });
}

export function useApproveDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      draftID,
      positionID,
    }: {
      draftID: string;
      positionID: string;
    }) =>
      apiFetch<ApproveDraftResponse>(`/import/drafts/${draftID}/approve`, {
        method: "POST",
        body: JSON.stringify({ position_id: positionID }),
      }),
    onSuccess: () => {
      // Both, and on purpose: approving changes a draft's status AND moves a
      // number in the batch header's counts.
      queryClient.invalidateQueries({ queryKey: ["import", batchID] });
    },
  });
}

export function useRejectDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (draftID: string) =>
      apiFetch<{ id: string; status: string }>(
        `/import/drafts/${draftID}/reject`,
        { method: "POST" },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["import", batchID] });
    },
  });
}
