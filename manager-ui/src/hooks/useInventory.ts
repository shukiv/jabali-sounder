import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";

export interface DomainRow {
  id: string;
  name: string;
  user_id: string;
  is_enabled: boolean;
  server_id: string;
  server_name: string;
}

export interface UserRow {
  id: string;
  email: string;
  username: string;
  package_id: string;
  is_admin: boolean;
  server_id: string;
  server_name: string;
}

interface ListEnvelope<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export function useDomains() {
  return useQuery({
    queryKey: ["domains"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<DomainRow>>("/admin/domains");
      return resp.data.data;
    },
  });
}

export function useUsers() {
  return useQuery({
    queryKey: ["users"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<UserRow>>("/admin/users");
      return resp.data.data;
    },
  });
}
