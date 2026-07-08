import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type { ListEnvelope, MonitorLiveEntry, MonitorSummaryEntry } from "../types";

export function useMonitorLive() {
  return useQuery({
    queryKey: ["monitor", "live"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<MonitorLiveEntry>>("/admin/monitor/live");
      return resp.data.data;
    },
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
  });
}

export function useMonitorSummary() {
  return useQuery({
    queryKey: ["monitor", "summary"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<MonitorSummaryEntry>>("/admin/monitor/summary");
      return resp.data.data;
    },
    refetchInterval: 60000,
    refetchIntervalInBackground: false,
  });
}
