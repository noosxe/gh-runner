import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppShell } from "./components/layout/app-shell";
import { DashboardPage } from "./routes/dashboard";
import { PoolsPage } from "./routes/pools";
import { PoolDetailPage } from "./routes/pool-detail";
import { HistoryPage } from "./routes/history";
import { HistoryDetailPage } from "./routes/history-detail";
import { ProfilesPage } from "./routes/profiles";
import { RenovatePage } from "./routes/renovate";
import { SettingsPage } from "./routes/settings";
import { LoginPage } from "./routes/login";
import { OnboardingPage } from "./routes/onboarding";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// Root Route
const rootRoute = createRootRoute({
  component: () => (
    <QueryClientProvider client={queryClient}>
      <Outlet />
    </QueryClientProvider>
  ),
});

// Public Routes
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

const onboardingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/onboarding",
  component: OnboardingPage,
});

// Authenticated App Shell Layout
const authenticatedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "_authenticated",
  component: AppShell,
});

// Nested Authenticated Child Routes
const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/",
  component: DashboardPage,
});

const poolsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/pools",
  component: PoolsPage,
});

const poolDetailRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/pools/$poolId",
  component: PoolDetailPage,
});

const historyRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/history",
  component: HistoryPage,
});

const historyDetailRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/history/$jobId",
  component: HistoryDetailPage,
});

const profilesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/profiles",
  component: ProfilesPage,
});

const renovateRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/renovate",
  component: RenovatePage,
});

const settingsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: "/settings",
  component: SettingsPage,
});

// Route Tree
const routeTree = rootRoute.addChildren([
  loginRoute,
  onboardingRoute,
  authenticatedRoute.addChildren([
    indexRoute,
    poolsRoute,
    poolDetailRoute,
    historyRoute,
    historyDetailRoute,
    profilesRoute,
    renovateRoute,
    settingsRoute,
  ]),
]);

export const router = createRouter({
  routeTree,
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

export function AppRouter() {
  return <RouterProvider router={router} />;
}
