import axios, { type AxiosAdapter, type AxiosResponse } from "axios";
import { Call, System } from "@wailsio/runtime";

const client = axios.create({
  baseURL: "/api/v1",
  headers: { "Content-Type": "application/json" },
});

// On mobile (Wails iOS/Android) the WebView's asset loader cannot convey a
// request's method, headers, or body — every request would arrive as a bodyless
// GET with no Authorization header. So route all API calls through the Go
// backend via the runtime (main.Bridge.ApiCall), which carries the full
// payload. No-op in the browser/desktop build (they use the normal XHR adapter).
function isMobileApp(): boolean {
  try {
    return System.IsMobile();
  } catch {
    return false;
  }
}

const wailsAdapter: AxiosAdapter = async (config) => {
  const method = (config.method || "get").toUpperCase();
  const base = config.baseURL || "";
  const url = config.url || "";
  // Join baseURL + url without duplicating the slash.
  const path = url.startsWith("http")
    ? url
    : (base.replace(/\/$/, "") + "/" + url.replace(/^\//, "")).replace(/^([^/])/, "/$1");

  const headers: Record<string, string> = {};
  const raw =
    config.headers && typeof (config.headers as { toJSON?: unknown }).toJSON === "function"
      ? (config.headers as { toJSON: () => Record<string, unknown> }).toJSON()
      : (config.headers as Record<string, unknown>) || {};
  for (const k of Object.keys(raw)) {
    const v = raw[k];
    if (typeof v === "string") headers[k] = v;
  }

  const body =
    config.data == null
      ? ""
      : typeof config.data === "string"
        ? config.data
        : JSON.stringify(config.data);

  const res = (await Call.ByName(
    "main.Bridge.ApiCall",
    method,
    path,
    JSON.stringify(headers),
    body,
  )) as { status: number; body: string };

  let data: unknown = res.body;
  if (config.responseType === "blob") {
    data = new Blob([res.body], { type: "application/json" });
  } else if (res.body) {
    try {
      data = JSON.parse(res.body);
    } catch {
      /* leave as string */
    }
  } else {
    data = null;
  }

  const response: AxiosResponse = {
    data,
    status: res.status,
    statusText: "",
    headers: {},
    config,
    request: {},
  };

  if (res.status >= 200 && res.status < 300) return response;
  // Mirror axios: reject non-2xx with the response attached so the interceptor
  // and callers see error.response.data.error.
  const err = new Error("Request failed with status code " + res.status) as Error & {
    response?: AxiosResponse;
    config?: unknown;
    isAxiosError?: boolean;
  };
  err.response = response;
  err.config = config;
  err.isAxiosError = true;
  throw err;
};

if (isMobileApp()) {
  client.defaults.adapter = wailsAdapter;
}

// Error envelope — extract the message from the standard error shape.
// On 401, clear auth and redirect to login IF we were authenticated (session
// expired). If there was no session (e.g. a failed login POST), don't reload —
// let the form show the error instead of flashing the page.
client.interceptors.response.use(
  (resp) => resp,
  (error) => {
    if (error.response?.status === 401) {
      const wasAuthed =
        !!localStorage.getItem("jabali-sounder-auth") ||
        !!localStorage.getItem("jabali-manager-auth") ||
        !!client.defaults.headers.common["Authorization"];
      localStorage.removeItem("jabali-sounder-auth");
      localStorage.removeItem("jabali-manager-auth");
      delete client.defaults.headers.common["Authorization"];
      if (wasAuthed) {
        // Full reload to a clean, unauthenticated state -> Login screen.
        window.location.assign("/");
        return new Promise(() => {}); // never resolves; page is navigating away
      }
    }
    if (error.response?.data?.error) {
      return Promise.reject(
        new Error(error.response.data.error + (error.response.data.detail ? ": " + error.response.data.detail : "")),
      );
    }
    return Promise.reject(error);
  },
);

export default client;
