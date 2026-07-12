// Bridge to native capabilities exposed by the Wails desktop/mobile build.
// In Wails v3 the SPA calls Go service methods via @wailsio/runtime
// Call.ByName("main.Bridge.<Method>"). Absent in the plain browser/server
// build, where the DOM's own behavior (downloads, target="_blank") works.
import { Call, System } from "@wailsio/runtime";

export interface UpdateResult {
  ok: boolean;
  message: string;
  installed_version?: string;
}

export interface DesktopBridge {
  SaveFile?: (name: string, content: string) => Promise<string>;
  OpenExternal?: (url: string) => void;
  // Desktop self-update: download the latest release for this OS, verify its
  // checksum, swap the running binary, and relaunch. Desktop only — the app
  // stores forbid self-update, so this is hidden on iOS/Android.
  InstallUpdate?: () => Promise<UpdateResult>;
}

// inWails is true inside any Wails webview (desktop or mobile). It reads the
// native-injected environment, so it is false in a plain browser and never
// throws.
function inWails(): boolean {
  try {
    return System.IsDesktop() || System.IsMobile();
  } catch {
    return false;
  }
}

export function desktopBridge(): DesktopBridge | undefined {
  if (!inWails()) return undefined;
  const bridge: DesktopBridge = {
    SaveFile: (name, content) =>
      Call.ByName("main.Bridge.SaveFile", name, content) as Promise<string>,
    OpenExternal: (url) => {
      void Call.ByName("main.Bridge.OpenExternal", url);
    },
  };
  // Self-update is a desktop-only capability.
  if (System.IsDesktop()) {
    bridge.InstallUpdate = () =>
      Call.ByName("main.Bridge.InstallUpdate") as Promise<UpdateResult>;
  }
  return bridge;
}

// installExternalLinkHandler routes clicks on external http(s) links to the
// system browser when running in the Wails webview (which does not open
// target="_blank" links itself). No-op in the browser. Returns a cleanup fn.
export function installExternalLinkHandler(): () => void {
  const bridge = desktopBridge();
  if (!bridge?.OpenExternal) return () => {};

  const handler = (e: MouseEvent) => {
    if (e.defaultPrevented || e.button !== 0) return;
    const anchor = (e.target as HTMLElement | null)?.closest?.("a");
    const href = anchor?.getAttribute("href");
    if (href && /^https?:\/\//i.test(href)) {
      e.preventDefault();
      bridge.OpenExternal?.(href);
    }
  };

  document.addEventListener("click", handler, true);
  return () => document.removeEventListener("click", handler, true);
}
