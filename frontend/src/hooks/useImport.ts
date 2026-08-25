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
 * The states a batch is still being worked in. Everything else — `review`,
 * `complete`, and any status added later that this UI has not heard of — is
 * something a person can look at.
 *
 * Defined as the working set rather than as a single finished state on
 * purpose. The first version of this screen tested `status === "ready"`, a
 * value the database's CHECK constraint does not contain and nothing ever
 * writes, so every batch read as still-extracting and no draft ever rendered.
 * A closed list of finished states fails that way; a closed list of working
 * states degrades to showing the drafts.
 */
const WORKING_STATUSES = new Set(["pending", "extracting", "enriching"]);

export function isBatchWorking(status: string): boolean {
  return WORKING_STATUSES.has(status);
}

export function useImportBatch(batchID: string | undefined) {
  return useQuery({
    queryKey: ["import", batchID],
    queryFn: () => apiFetch<ImportBatch>(`/import/${batchID}`),
    enabled: !!batchID,
    refetchInterval: (query) =>
      query.state.data && isBatchWorking(query.state.data.status)
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
