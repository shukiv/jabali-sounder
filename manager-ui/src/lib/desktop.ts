// Bridge to native capabilities exposed by the Wails desktop build
// (window.go.main.Bridge). Absent in the browser/server build, where the DOM's
// own behavior (downloads, target="_blank") already works.

export interface DesktopBridge {
  SaveFile?: (name: string, content: string) => Promise<string>;
  OpenExternal?: (url: string) => void;
}

export function desktopBridge(): DesktopBridge | undefined {
  return (window as unknown as { go?: { main?: { Bridge?: DesktopBridge } } })
    .go?.main?.Bridge;
}

// installExternalLinkHandler routes clicks on external http(s) links to the
// system browser when running in the desktop webview (which does not open
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
