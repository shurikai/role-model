import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
