import { useQueries, useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type {
  ListEnvelope,
  MetricHistory,
  MonitorLiveEntry,
  MonitorSummaryEntry,
} from "../types";

// useMonitorLive polls the real-time fleet metrics. Polling is gated by
// `enabled` so historical Monitor views can stop the 5s live poll entirely
// (SND-78: historical mode must not keep high-frequency polling).
export function useMonitorLive(enabled = true, intervalMs: number | false = 5000) {
  return useQuery({
    queryKey: ["monitor", "live"] as const,
    enabled,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<MonitorLiveEntry>>("/admin/monitor/live");
      return resp.data.data;
    },
    refetchInterval: enabled ? intervalMs : false,
    refetchIntervalInBackground: false,
  });
}

export function useMonitorSummary(intervalMs: number | false = 60000) {
  return useQuery({
    queryKey: ["monitor", "summary"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<MonitorSummaryEntry>>("/admin/monitor/summary");
      return resp.data.data;
    },
    refetchInterval: intervalMs,
    refetchIntervalInBackground: false,
  });
}

// useMonitorHistory fetches stored metric samples per server for the selected
// range, so the Monitor overview can show trends and range averages without
// the live poll (SND-78). Results align to the `ids` order.
export function useMonitorHistory(ids: string[], range: string, enabled: boolean) {
  return useQueries({
    queries: ids.map((id) => ({
      queryKey: ["server-metrics", id, range] as const,
      enabled: enabled && !!id,
      staleTime: 30000,
      queryFn: async () => {
        const resp = await apiClient.get<MetricHistory>(
          `/admin/servers/${id}/metrics?range=${range}`,
        );
        return resp.data;
      },
    })),
  });
}
