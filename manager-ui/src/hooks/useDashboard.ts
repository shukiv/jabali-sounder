import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type { DashboardEntry, ListEnvelope } from "../types";

export function useDashboard() {
  return useQuery({
    queryKey: ["dashboard"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<DashboardEntry>>("/admin/dashboard");
      return resp.data.data;
    },
    refetchInterval: 30000,
  });
}
