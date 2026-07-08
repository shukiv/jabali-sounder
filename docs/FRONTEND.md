# Frontend

The UI is a React/Vite SPA using Ant Design, TanStack Query, axios, and
react-router.

## Structure

```text
manager-ui/src/App.tsx
  Auth gate and route tree.

manager-ui/src/shells/AdminLayout.tsx
  Authenticated app shell, side navigation, header, footer.

manager-ui/src/admin/
  Page-level views.

manager-ui/src/hooks/
  API hooks and shared state hooks.

manager-ui/src/types.ts
  API response and table row types.

manager-ui/src/apiClient.ts
  Shared axios client with `/api/v1` base URL.
```

## Routes

Authenticated routes:

| Path | Page |
| --- | --- |
| `/` | Dashboard |
| `/servers` | Managed server enrollment and checks |
| `/monitor` | Live and summary server metrics |
| `/mail` | Mail stack inventory |
| `/domains` | Cross-server domains |
| `/users` | Cross-server users |

Unauthenticated users see the login page for all routes.

## API Client

The shared axios client uses:

```text
baseURL: /api/v1
```

On `401`, it clears browser auth storage and reloads the app.

Current auth storage keys:

- `jabali-sounder-auth`
- `jabali-manager-auth` is also cleared for rename compatibility.

## Data Fetching

TanStack Query hooks live in `manager-ui/src/hooks`.

Current hooks:

- `useAuth`
- `useDashboard`
- `useServers`
- `useInventory`
- `useMonitor`
- `useMail`
- `useTheme`

Monitor polling:

- live metrics: 5 seconds
- summary: 60 seconds

Mail polling:

- 60 seconds

## Design Notes

This is an operational admin app. Prefer:

- Dense tables.
- Clear status tags.
- Small, direct controls.
- Responsive layouts that still preserve data visibility.
- Feature-specific warnings when a managed server cannot supply data.

Avoid marketing-style landing pages or decorative components. The first screen
after login should be useful operational state.

## Build

```bash
cd manager-ui
npm run build
```

Output:

```text
manager-ui/dist/
```

## Lint and Test

```bash
cd manager-ui
npm run lint
npm test
```

Known warning:

```text
src/theme/ThemeModeContext.tsx
Fast refresh only works when a file only exports components.
```

## Adding a Page

1. Add response types in `src/types.ts`.
2. Add a TanStack Query hook in `src/hooks`.
3. Add a page component in `src/admin`.
4. Add the route in `src/App.tsx`.
5. Add navigation in `src/shells/AdminLayout.tsx`.
6. Run `npm run lint` and `npm run build`.
