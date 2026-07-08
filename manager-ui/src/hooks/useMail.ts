import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";
import type { ListEnvelope, MailSnapshotEntry } from "../types";

export function useMail() {
  return useQuery({
    queryKey: ["mail"] as const,
    queryFn: async () => {
      const resp = await apiClient.get<ListEnvelope<MailSnapshotEntry>>("/admin/mail");
      return resp.data.data;
    },
    refetchInterval: 60000,
    refetchIntervalInBackground: false,
  });
}
