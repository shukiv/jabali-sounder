import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntdApp } from "antd";
import { BrowserRouter } from "react-router";

import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "antd/dist/reset.css";
import "./global.css";

import App from "./App";
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
