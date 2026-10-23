import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  CareerLevel,
  CareerTree,
  Contribution,
  CreateContributionRequest,
  CreateEmployerRequest,
  CreatePositionRequest,
  Employer,
  Position,
  UpdateContributionRequest,
  UpdateEmployerRequest,
  UpdatePositionRequest,
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

/**
 * The account's seniority ladder, for a position's industry_level. It is the
 * user's own rows, not a fixed enum, so the form offers whatever is there.
 */
export function useCareerLevels() {
  return useQuery({
    queryKey: ["vocabulary", "career-levels"],
    queryFn: () => apiFetch<CareerLevel[]>("/vocabulary/career-levels"),
    // Installed at signup and edited rarely; refetching it per mount is noise.
    staleTime: 5 * 60 * 1000,
  });
}

// Every career mutation invalidates ["career"]: the tree carries names,
// dates and contribution counts, so almost any edit moves something on it.

export function useUpdateEmployer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...req }: UpdateEmployerRequest & { id: string }) =>
      apiFetch<Employer>(`/employers/${id}`, {
        method: "PATCH",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["career"] });
      queryClient.invalidateQueries({ queryKey: ["employers"] });
    },
  });
}

/** 409 has_dependents while the employer has positions. */
export function useDeleteEmployer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/employers/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["career"] });
      queryClient.invalidateQueries({ queryKey: ["employers"] });
    },
  });
}

export function useUpdatePosition() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...req }: UpdatePositionRequest & { id: string }) =>
      apiFetch<Position>(`/positions/${id}`, {
        method: "PATCH",
        body: JSON.stringify(req),
      }),
    onSuccess: (position) => {
      queryClient.invalidateQueries({ queryKey: ["career"] });
      queryClient.invalidateQueries({
        queryKey: ["employers", position.employer_id, "positions"],
      });
    },
  });
}

/** 409 has_dependents while the position has contributions, hidden included. */
export function useDeletePosition() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (position: Pick<Position, "id" | "employer_id">) =>
      apiFetch<void>(`/positions/${position.id}`, { method: "DELETE" }),
    onSuccess: (_, position) => {
      queryClient.invalidateQueries({ queryKey: ["career"] });
      queryClient.invalidateQueries({
        queryKey: ["employers", position.employer_id, "positions"],
      });
    },
  });
}

function useInvalidateContributions(positionID: string) {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: ["career"] });
    queryClient.invalidateQueries({
      queryKey: ["positions", positionID, "contributions"],
    });
  };
}

export function useCreateContribution(positionID: string) {
  const invalidate = useInvalidateContributions(positionID);
  return useMutation({
    mutationFn: (req: Omit<CreateContributionRequest, "position_id">) =>
      apiFetch<Contribution>("/contributions", {
        method: "POST",
        body: JSON.stringify({ ...req, position_id: positionID }),
      }),
    onSuccess: invalidate,
  });
}

export function useUpdateContribution(positionID: string) {
  const invalidate = useInvalidateContributions(positionID);
  return useMutation({
    mutationFn: ({ id, ...req }: UpdateContributionRequest & { id: string }) =>
      apiFetch<Contribution>(`/contributions/${id}`, {
        method: "PATCH",
        body: JSON.stringify(req),
      }),
    onSuccess: invalidate,
  });
}

/** A hard delete: tags, project links and feedback go with it. */
export function useDeleteContribution(positionID: string) {
  const invalidate = useInvalidateContributions(positionID);
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/contributions/${id}`, { method: "DELETE" }),
    onSuccess: invalidate,
  });
}
