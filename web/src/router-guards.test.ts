import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { fetchOnboardingStatus, fetchSession } from "./lib/api/query-hooks";
import { onboardingClient, authClient } from "./lib/api/transport";

describe("Route Guards & Redirect Matrix Logic", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });
    vi.clearAllMocks();
  });

  it("Case 1: setupComplete is false -> uninitialized system", async () => {
    vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
      setupComplete: false,
      adminCreated: false,
      authProfileExists: false,
      poolExists: false,
    } as any);

    const status = await fetchOnboardingStatus(queryClient);
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

    const status = await fetchOnboardingStatus(queryClient);
    const session = await fetchSession(queryClient);

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

    const status = await fetchOnboardingStatus(queryClient);
    const session = await fetchSession(queryClient);

    expect(status.setupComplete).toBe(true);
    expect(session).not.toBeNull();
    expect(session?.username).toBe("admin");
  });
});
