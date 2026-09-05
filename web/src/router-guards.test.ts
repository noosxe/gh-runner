import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { fetchOnboardingStatus, fetchSession } from "./lib/api/query-hooks";
import { onboardingClient, authClient } from "./lib/api/transport";
import { loginRoute, onboardingRoute, authenticatedRoute, queryClient } from "./router";

describe("Route Guards & Redirect Matrix Logic", () => {
  let testQueryClient: QueryClient;

  beforeEach(() => {
    testQueryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });
    queryClient.setDefaultOptions({
      queries: { retry: false },
    });
    queryClient.clear();
    vi.clearAllMocks();
  });

  describe("Query Fetchers", () => {
    it("Case 1: setupComplete is false -> uninitialized system", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: false,
        adminCreated: false,
        authProfileExists: false,
        poolExists: false,
      } as any);

      const status = await fetchOnboardingStatus(testQueryClient);
      expect(status.setupComplete).toBe(false);
    });

    it("Case 2: setupComplete is true, unauthenticated session -> returns null session", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: true,
        adminCreated: true,
        authProfileExists: true,
        poolExists: true,
      } as any);
      vi.spyOn(authClient, "getSession").mockRejectedValue(new Error("unauthenticated"));

      const status = await fetchOnboardingStatus(testQueryClient);
      const session = await fetchSession(testQueryClient);

      expect(status.setupComplete).toBe(true);
      expect(session).toBeNull();
    });

    it("Case 3: setupComplete is true, authenticated session -> returns active session", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: true,
        adminCreated: true,
        authProfileExists: true,
        poolExists: true,
      } as any);
      vi.spyOn(authClient, "getSession").mockResolvedValue({
        username: "admin",
        isAdmin: true,
      } as any);

      const status = await fetchOnboardingStatus(testQueryClient);
      const session = await fetchSession(testQueryClient);

      expect(status.setupComplete).toBe(true);
      expect(session).not.toBeNull();
      expect(session?.username).toBe("admin");
    });
  });

  describe("Route beforeLoad Redirection Matrix", () => {
    it("Uninitialized system: redirects login and authenticated routes to /onboarding", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: false,
        adminCreated: false,
        authProfileExists: false,
        poolExists: false,
      } as any);

      // loginRoute beforeLoad should redirect to /onboarding
      await expect(loginRoute.options.beforeLoad?.({} as any)).rejects.toMatchObject({
        options: { to: "/onboarding" },
      });

      // authenticatedRoute beforeLoad should redirect to /onboarding
      await expect(
        authenticatedRoute.options.beforeLoad?.({
          location: { pathname: "/pools" },
        } as any),
      ).rejects.toMatchObject({
        options: { to: "/onboarding" },
      });

      // onboardingRoute beforeLoad should succeed without redirection
      await expect(onboardingRoute.options.beforeLoad?.({} as any)).resolves.toBeUndefined();
    });

    it("Initialized but unauthenticated: redirects /onboarding and authenticated routes to /login", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: true,
        adminCreated: true,
        authProfileExists: true,
        poolExists: true,
      } as any);
      vi.spyOn(authClient, "getSession").mockRejectedValue(new Error("unauthenticated"));

      // onboardingRoute beforeLoad should redirect to /login
      await expect(onboardingRoute.options.beforeLoad?.({} as any)).rejects.toMatchObject({
        options: { to: "/login" },
      });

      // authenticatedRoute beforeLoad should redirect to /login with target redirect query
      await expect(
        authenticatedRoute.options.beforeLoad?.({
          location: { pathname: "/pools" },
        } as any),
      ).rejects.toMatchObject({
        options: {
          to: "/login",
          search: { redirect: "/pools" },
        },
      });

      // loginRoute beforeLoad should succeed without redirection
      await expect(loginRoute.options.beforeLoad?.({} as any)).resolves.toBeUndefined();
    });

    it("Initialized and authenticated: redirects /login and /onboarding to /", async () => {
      vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
        setupComplete: true,
        adminCreated: true,
        authProfileExists: true,
        poolExists: true,
      } as any);
      vi.spyOn(authClient, "getSession").mockResolvedValue({
        username: "admin",
        isAdmin: true,
      } as any);

      // loginRoute beforeLoad should redirect to /
      await expect(loginRoute.options.beforeLoad?.({} as any)).rejects.toMatchObject({
        options: { to: "/" },
      });

      // onboardingRoute beforeLoad should redirect to /
      await expect(onboardingRoute.options.beforeLoad?.({} as any)).rejects.toMatchObject({
        options: { to: "/" },
      });

      // authenticatedRoute beforeLoad should return session and onboarding
      const context = await authenticatedRoute.options.beforeLoad?.({
        location: { pathname: "/pools" },
      } as any);
      expect(context).toMatchObject({
        session: { username: "admin" },
        onboarding: { setupComplete: true },
      });
    });
  });
});
