import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntdApp } from "antd";
import { BrowserRouter } from "react-router";

import "@fontsource/inter/latin-400.css";
import "@fontsource/inter/latin-500.css";
import "@fontsource/inter/latin-600.css";
import "@fontsource/inter/latin-700.css";
import "antd/dist/reset.css";
import "./global.css";
import "./i18n";

import App from "./App";
import { isNativeApp } from "./lib/desktop";
import { ThemeModeProvider } from "./theme/ThemeModeContext";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ThemeModeProvider>
          <AntdApp>
            <App />
          </AntdApp>
        </ThemeModeProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);

// The Wails runtime (imported for Call/System) installs a global contextmenu
// handler that hides the browser menu except on selected text — wanted in the
// desktop app, but not in the browser-served web build. Re-enable the normal
// right-click menu there via Wails' documented --default-contextmenu: show.
if (!isNativeApp()) {
  document.documentElement.style.setProperty("--default-contextmenu", "show");
}

// Register the PWA service worker for the web/server build only. Skip it inside
// the Wails desktop/mobile apps (they serve via the WebViewAssetLoader) and in
// dev. Failures are non-fatal.
if (import.meta.env.PROD && "serviceWorker" in navigator && !isNativeApp()) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  });
}
