import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  CreateSkillRequest,
  Preference,
  PreferenceRequest,
  ProficiencyLevel,
  Skill,
  UpdateSkillRequest,
} from "../lib/types";

/**
 * Skills and preferences — the claims the fit gate scores a posting against.
 *
 * Both had a full API and nothing in front of it, which meant a preference
 * could be created exactly once, by an import, and never corrected. The
 * profile is also the part of this system that changes most as a search goes
 * on: a dealbreaker learned from a bad interview is the normal way one gets
 * added, and that happens long after any import.
 */

export function useProficiencyLevels() {
  return useQuery({
    queryKey: ["vocabulary", "proficiency-levels"],
    queryFn: () =>
      apiFetch<ProficiencyLevel[]>("/vocabulary/proficiency-levels"),
    // Installed at signup and edited rarely; refetching it per mount is noise.
    staleTime: 5 * 60 * 1000,
  });
}

export function useSkills() {
  return useQuery({
    queryKey: ["skills"],
    queryFn: () => apiFetch<Skill[]>("/skills"),
  });
}

export function useCreateSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateSkillRequest) =>
      apiFetch<Skill>("/skills", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["skills"] });
      // Creating a skill can create the tag and its category too, so anything
      // reading the tag vocabulary is now stale.
      queryClient.invalidateQueries({ queryKey: ["tags"] });
    },
  });
}

export function useUpdateSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...req }: UpdateSkillRequest & { id: string }) =>
      apiFetch<Skill>(`/skills/${id}`, {
        method: "PATCH",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}

export function useDeleteSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/skills/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["skills"] });
    },
  });
}

export function usePreferences() {
  return useQuery({
    queryKey: ["preferences"],
    queryFn: () => apiFetch<Preference[]>("/preferences"),
  });
}

export function useCreatePreference() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: PreferenceRequest) =>
      apiFetch<Preference>("/preferences", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["preferences"] });
    },
  });
}

export function useUpdatePreference() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...req }: PreferenceRequest & { id: string }) =>
      apiFetch<Preference>(`/preferences/${id}`, {
        method: "PATCH",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["preferences"] });
    },
  });
}

export function useDeletePreference() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/preferences/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["preferences"] });
    },
  });
}
