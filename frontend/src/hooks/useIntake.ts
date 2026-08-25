import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  ApproveEntityDraftResponse,
  EntityDraft,
  ResolveBatchResponse,
  StartCareerImportResponse,
} from "../lib/types";

/**
 * The wide import path — a whole career, including the employers and positions
 * it hangs off. Separate from `useImport.ts`, which drives the narrow path
 * (contribution drafts against positions that already exist). The two stage
 * different things and the backend keeps them apart; so does this.
 */

const entitiesKey = (batchID: string | undefined) =>
  ["import", batchID, "entities"] as const;

export function useStartCareerImport() {
  return useMutation({
    mutationFn: (rawText: string) =>
      apiFetch<StartCareerImportResponse>("/import/career", {
        method: "POST",
        body: JSON.stringify({ raw_text: rawText }),
      }),
  });
}

export function useEntityDrafts(batchID: string | undefined) {
  return useQuery({
    queryKey: entitiesKey(batchID),
    queryFn: () => apiFetch<EntityDraft[]>(`/import/${batchID}/entities`),
    enabled: !!batchID,
  });
}

export function useApproveEntityDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (draftID: string) =>
      apiFetch<ApproveEntityDraftResponse>(
        `/import/entities/${draftID}/approve`,
        { method: "POST" },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entitiesKey(batchID) });
    },
  });
}

export function useRejectEntityDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (draftID: string) =>
      apiFetch<{ id: string; status: string }>(
        `/import/entities/${draftID}/reject`,
        { method: "POST" },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entitiesKey(batchID) });
    },
  });
}

export function useUpdateEntityDraft(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      draftID,
      payload,
    }: {
      draftID: string;
      payload: Record<string, unknown>;
    }) =>
      // Whole payload, every time. The backend replaces rather than merges,
      // and rejects a field it does not recognise for that kind — so a
      // partial object here would silently drop what it left out.
      apiFetch<EntityDraft>(`/import/entities/${draftID}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entitiesKey(batchID) });
    },
  });
}

export function useResolveBatch(batchID: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<ResolveBatchResponse>(`/import/${batchID}/resolve`, {
        method: "POST",
      }),
    onSuccess: () => {
      // The batch's own status moves too, so invalidate the whole subtree
      // rather than just the entity list.
      queryClient.invalidateQueries({ queryKey: ["import", batchID] });
    },
  });
}
