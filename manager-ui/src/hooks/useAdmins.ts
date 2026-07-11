import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type { Admin, ListEnvelope } from "../types";

export function useAdmins() {
  return useQuery({
    queryKey: ["admins"],
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<Admin>>("/admin/admins");
      return resp.data.data;
    },
  });
}

export function useCreateAdmin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { username: string; password: string; role: string }) => {
      const resp = await apiClient.post<Admin>("/admin/admins", input);
      return resp.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admins"] }),
  });
}

export function useUpdateAdmin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; role?: string; password?: string }) => {
      const { id, ...body } = input;
      const resp = await apiClient.patch<Admin>(`/admin/admins/${id}`, body);
      return resp.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admins"] }),
  });
}

export function useDeleteAdmin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient.delete(`/admin/admins/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admins"] }),
  });
}
