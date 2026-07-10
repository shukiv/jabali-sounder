/// <reference types="vite/client" />

declare global {
  // Injected at build time from package.json version (see vite.config.ts).
  const __APP_VERSION__: string;
}

export {};
