// ThemeModeContext — tracks whether the sounder is in light or dark mode.
// Mirrors jabali2's ThemeModeContext exactly.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type ThemeMode = "light" | "dark";

const STORAGE_KEY = "jabali-sounder.themeMode";
const MEDIA_QUERY = "(prefers-color-scheme: dark)";

const readStored = (): ThemeMode | null => {
  if (typeof window === "undefined") return null;
  try {
    const v = window.localStorage.getItem(STORAGE_KEY);
    return v === "light" || v === "dark" ? v : null;
  } catch {
    return null;
  }
};

const readSystem = (): ThemeMode => {
  if (typeof window === "undefined" || !window.matchMedia) return "light";
  return window.matchMedia(MEDIA_QUERY).matches ? "dark" : "light";
};

interface ThemeModeContextValue {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  toggle: () => void;
}

const ThemeModeContext = createContext<ThemeModeContextValue | null>(null);

export function ThemeModeProvider({ children }: { children: ReactNode }) {
  const [pinned, setPinned] = useState<boolean>(() => readStored() !== null);
  const [mode, setModeState] = useState<ThemeMode>(
    () => readStored() ?? readSystem(),
  );

  useEffect(() => {
    if (typeof document === "undefined") return;
    document.documentElement.dataset.theme = mode;
    document.body.dataset.theme = mode;
  }, [mode]);

  useEffect(() => {
    if (pinned) return;
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia(MEDIA_QUERY);
    const handler = (e: MediaQueryListEvent) => setModeState(e.matches ? "dark" : "light");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [pinned]);

  const setMode = useCallback((m: ThemeMode) => {
    setModeState(m);
    setPinned(true);
    try {
      window.localStorage.setItem(STORAGE_KEY, m);
    } catch {
      // ignore
    }
  }, []);

  const toggle = useCallback(() => {
    setModeState((prev) => {
      const next = prev === "dark" ? "light" : "dark";
      setPinned(true);
      try {
        window.localStorage.setItem(STORAGE_KEY, next);
      } catch {
        // ignore
      }
      return next;
    });
  }, []);

  const value = useMemo<ThemeModeContextValue>(
    () => ({ mode, setMode, toggle }),
    [mode, setMode, toggle],
  );

  return (
    <ThemeModeContext.Provider value={value}>
      {children}
    </ThemeModeContext.Provider>
  );
}

export function useThemeMode(): ThemeModeContextValue {
  const ctx = useContext(ThemeModeContext);
  if (!ctx) throw new Error("useThemeMode must be used within ThemeModeProvider");
  return ctx;
}
