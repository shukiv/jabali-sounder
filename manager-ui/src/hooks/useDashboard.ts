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

export interface FleetSLA {
  window_days: number;
  fleet_ratio: number | null;
  servers: { id: string; name: string; ratio: number | null }[];
}

export function useFleetSLA() {
  return useQuery({
    queryKey: ["fleet-sla"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<{ sla: FleetSLA }>("/admin/dashboard");
      return resp.data.sla;
    },
    refetchInterval: 60000,
  });
}
