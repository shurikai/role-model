import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  CreateEmployerRequest,
  CreatePositionRequest,
  Employer,
  Position,
} from "../lib/types";

export function useEmployers() {
  return useQuery({
    queryKey: ["employers"],
    queryFn: () => apiFetch<Employer[]>("/employers"),
  });
}

export function useEmployerPositions(employerID: string | undefined) {
  return useQuery({
    queryKey: ["employers", employerID, "positions"],
    queryFn: () => apiFetch<Position[]>(`/employers/${employerID}/positions`),
    enabled: !!employerID,
  });
}

/**
 * Whether the account has at least one position, and so whether there is
 * anything to add contributions to. An employer can exist with no positions
 * (a career import that approved the employer and rejected its roles), so the
 * employer list alone cannot answer this.
 *
 * One request per employer, reusing the same keys useEmployerPositions caches
 * under. The career tree endpoint (#118) will answer this in one.
 */
export function useHasPositions(): {
  hasPositions: boolean;
  isLoading: boolean;
} {
  const employers = useEmployers();
  const positions = useQueries({
    queries: (employers.data ?? []).map((e) => ({
      queryKey: ["employers", e.id, "positions"],
      queryFn: () => apiFetch<Position[]>(`/employers/${e.id}/positions`),
    })),
  });
  return {
    hasPositions: positions.some((q) => (q.data?.length ?? 0) > 0),
    isLoading: employers.isLoading || positions.some((q) => q.isLoading),
  };
}

export function useCreateEmployer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateEmployerRequest) =>
      apiFetch<Employer>("/employers", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["employers"] });
    },
  });
}

export function useCreatePosition() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreatePositionRequest) =>
      apiFetch<Position>("/positions", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: (position) => {
      queryClient.invalidateQueries({
        queryKey: ["employers", position.employer_id, "positions"],
      });
    },
  });
}
