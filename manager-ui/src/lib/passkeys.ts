import { startRegistration, startAuthentication } from "@simplewebauthn/browser";
import apiClient from "../apiClient";
import { isNativeApp } from "./desktop";

// passkeysAvailable reports whether WebAuthn can run here. It is false inside the
// native Wails app (WebAuthn is absent from the desktop WebKitGTK build and the
// Android WebView) and false without a secure context — so the passkey UI never
// appears where it cannot work. It is true for a real browser over https, e.g. a
// panel-proxied deployment.
export function passkeysAvailable(): boolean {
  if (typeof window === "undefined") return false;
  if (isNativeApp()) return false;
  return typeof window.PublicKeyCredential !== "undefined" && window.isSecureContext === true;
}

export interface PasskeyLoginResult {
  token: string;
  admin: { username: string; role?: string };
}

// registerPasskey enrolls a new passkey for the current admin. Requires the
// current password (the server re-verifies it) and returns once stored.
export async function registerPasskey(password: string, label: string): Promise<void> {
  const begin = await apiClient.post("/auth/passkeys/register/begin", { password });
  const attResp = await startRegistration({ optionsJSON: begin.data.publicKey });
  await apiClient.post(
    `/auth/passkeys/register/finish?session=${encodeURIComponent(begin.data.session)}&label=${encodeURIComponent(label)}`,
    attResp,
  );
}

// loginWithPasskey performs a passwordless (discoverable) login and returns the
// issued token + admin, matching the shape useAuth expects.
export async function loginWithPasskey(remember: boolean): Promise<PasskeyLoginResult> {
  const begin = await apiClient.post("/auth/passkeys/login/begin", {});
  const asseResp = await startAuthentication({ optionsJSON: begin.data.publicKey });
  const finish = await apiClient.post(
    `/auth/passkeys/login/finish?session=${encodeURIComponent(begin.data.session)}&remember=${remember ? "true" : "false"}`,
    asseResp,
  );
  return finish.data;
}

export interface PasskeySummary {
  id: string;
  label: string;
  created_at: string;
  last_used_at?: string;
}

export async function listPasskeys(): Promise<PasskeySummary[]> {
  const resp = await apiClient.get<{ passkeys: PasskeySummary[] }>("/auth/passkeys");
  return resp.data.passkeys ?? [];
}

export async function deletePasskey(id: string): Promise<void> {
  await apiClient.delete(`/auth/passkeys/${encodeURIComponent(id)}`);
}
