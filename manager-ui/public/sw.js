/*
 * Service worker for the Jabali Sounder web/server build.
 *
 * Goal: make the control plane installable as a PWA and give it an offline app
 * shell — WITHOUT ever caching API or auth data. A fleet-management console must
 * stay fresh, so:
 *   - /api/* and /health are never intercepted (always straight to the network),
 *   - navigations (the app shell) are network-first, falling back to a cached
 *     index.html only when offline,
 *   - content-hashed static assets (js/css/fonts/icons) are cache-first, since
 *     their URLs change on every deploy so a cached copy is never stale.
 */
const CACHE = "sounder-shell-v1";
const SHELL = ["/", "/index.html", "/manifest.webmanifest"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(SHELL))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;

  // Never cache the API or health endpoint — freshness + auth correctness.
  if (url.pathname.startsWith("/api/") || url.pathname === "/health") return;

  // App shell / SPA navigations: network-first, offline fallback to index.html.
  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req).catch(() => caches.match("/index.html")),
    );
    return;
  }

  // Static, content-hashed assets: cache-first, then network (and cache it).
  event.respondWith(
    caches.match(req).then(
      (hit) =>
        hit ||
        fetch(req).then((res) => {
          if (res.ok && res.type === "basic") {
            const copy = res.clone();
            caches.open(CACHE).then((cache) => cache.put(req, copy));
          }
          return res;
        }),
    ),
  );
});
