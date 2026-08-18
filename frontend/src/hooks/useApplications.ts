import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch, apiFetchBlob } from "../lib/api-client";
import type {
  Application,
  CreateApplicationRequest,
  FitReport,
  ResumeVersion,
} from "../lib/types";

export function useApplications() {
  return useQuery({
    queryKey: ["applications"],
    queryFn: () => apiFetch<Application[]>("/applications"),
  });
}

export function useApplication(id: string | undefined) {
  return useQuery({
    queryKey: ["applications", id],
    queryFn: () => apiFetch<Application>(`/applications/${id}`),
    enabled: !!id,
  });
}

export function useCreateApplication() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateApplicationRequest) =>
      apiFetch<Application>("/applications", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

export function useExtractSignals() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (applicationId: string) =>
      apiFetch<Application>(`/applications/${applicationId}/extract-signals`, {
        method: "POST",
      }),
    onSuccess: (data, applicationId) => {
      queryClient.setQueryData(["applications", applicationId], data);
      queryClient.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

export function useFitReports(applicationId: string | undefined) {
  return useQuery({
    queryKey: ["applications", applicationId, "fit"],
    queryFn: () => apiFetch<FitReport[]>(`/applications/${applicationId}/fit`),
    enabled: !!applicationId,
  });
}

export function useRunFitEvaluation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (applicationId: string) =>
      apiFetch<FitReport>(`/applications/${applicationId}/fit`, {
        method: "POST",
      }),
    onSuccess: (_data, applicationId) => {
      queryClient.invalidateQueries({
        queryKey: ["applications", applicationId, "fit"],
      });
    },
  });
}

export function useResumeVersions(applicationId: string | undefined) {
  return useQuery({
    queryKey: ["applications", applicationId, "versions"],
    queryFn: () =>
      apiFetch<ResumeVersion[]>(`/applications/${applicationId}/versions`),
    enabled: !!applicationId,
  });
}

export function useGenerateResume() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (applicationId: string) =>
      apiFetch<ResumeVersion>(`/applications/${applicationId}/generate`, {
        method: "POST",
      }),
    onSuccess: (_data, applicationId) => {
      queryClient.invalidateQueries({
        queryKey: ["applications", applicationId, "versions"],
      });
    },
  });
}

export function useRenderResumeVersion() {
  return useMutation({
    mutationFn: (resumeVersionId: string) =>
      apiFetchBlob(`/resume-versions/${resumeVersionId}/render`, {
        method: "POST",
      }),
  });
}
