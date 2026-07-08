import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type { Server, ListEnvelope, CheckResult } from "../types";

const QK = ["servers"] as const;

export function useServers() {
  return useQuery({
    queryKey: QK,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<Server>>("/admin/servers");
      return resp.data.data;
    },
  });
}

export function useCreateServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      name: string;
      base_url: string;
      token_id: string;
      token_secret: string;
      scopes?: string[];
      insecure_skip_verify?: boolean;
    }) => {
      const resp = await apiClient.post<Server>("/admin/servers", input);
      return resp.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}

export function useUpdateServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      id: string;
      name?: string;
      base_url?: string;
      scopes?: string[];
      insecure_skip_verify?: boolean;
    }) => {
      const { id, ...payload } = input;
      const resp = await apiClient.patch<Server>(`/admin/servers/${id}`, payload);
      return resp.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}

export function useDeleteServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient.delete(`/admin/servers/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}

export function useCheckHealth() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const resp = await apiClient.post<CheckResult>(`/admin/servers/${id}/check`);
      return resp.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}
