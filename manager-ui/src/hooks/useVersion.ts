import { useQuery } from "@tanstack/react-query";
import apiClient from "../apiClient";

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  is_dev: boolean;
  update_available: boolean;
  latest?: string;
  release_url?: string;
  published_at?: string;
  update_error?: string;
}

// useVersion polls the build/update status. Cached long (updates are rare); the
// server side caches the GitHub call for an hour regardless.
export function useVersion() {
  return useQuery({
    queryKey: ["version"],
    queryFn: async () => (await apiClient.get<VersionInfo>("/version")).data,
    staleTime: 30 * 60 * 1000,
    refetchInterval: 6 * 60 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
}

// isDesktop reports whether the app runs inside the Wails desktop shell (which
// exposes the update-install bridge).
export function isDesktop(): boolean {
  const w = window as unknown as { go?: { main?: { Bridge?: unknown } } };
  return !!w.go?.main?.Bridge;
}
