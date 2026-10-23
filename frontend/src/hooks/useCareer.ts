import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  CareerTree,
  Contribution,
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

export function useCareerTree() {
  return useQuery({
    queryKey: ["career"],
    queryFn: () => apiFetch<CareerTree>("/career"),
  });
}

export function usePositionContributions(positionID: string | undefined) {
  return useQuery({
    queryKey: ["positions", positionID, "contributions"],
    queryFn: () =>
      apiFetch<Contribution[]>(`/positions/${positionID}/contributions`),
    enabled: !!positionID,
  });
}

/**
 * Whether the account has at least one position, and so whether there is
 * anything to add contributions to. An employer can exist with no positions
 * (a career import that approved the employer and rejected its roles), so the
 * employer count alone cannot answer this.
 */
export function useHasPositions(): {
  hasPositions: boolean;
  isLoading: boolean;
} {
  const tree = useCareerTree();
  return {
    hasPositions: (tree.data?.employers ?? []).some(
      (e) => e.positions.length > 0,
    ),
    isLoading: tree.isLoading,
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
      queryClient.invalidateQueries({ queryKey: ["career"] });
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
      queryClient.invalidateQueries({ queryKey: ["career"] });
    },
  });
}
