import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useOnboardingStatus, usePools, useSession } from "./query-hooks";
import { onboardingClient, poolClient, authClient } from "./transport";

describe("TanStack Query hooks with ConnectRPC", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    vi.restoreAllMocks();
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("useOnboardingStatus queries onboarding status", async () => {
    vi.spyOn(onboardingClient, "getOnboardingStatus").mockResolvedValue({
      adminCreated: true,
      authProfileExists: false,
      poolExists: false,
      setupComplete: false,
    } as any);

    const { result } = renderHook(() => useOnboardingStatus(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.adminCreated).toBe(true);
    expect(result.current.data?.setupComplete).toBe(false);
  });

  it("usePools queries runner pools list", async () => {
    vi.spyOn(poolClient, "listPools").mockResolvedValue({
      pools: [
        {
          id: 1n,
          name: "pool-arm64",
          provider: "github",
          repositoryUrl: "https://github.com/owner/repo",
          activeRunners: 2,
          minIdleRunners: 1,
          maxConcurrency: 5,
        },
      ],
    } as any);

    const { result } = renderHook(() => usePools(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(1);
    expect(result.current.data?.[0].name).toBe("pool-arm64");
  });

  it("useSession queries current user session", async () => {
    vi.spyOn(authClient, "getSession").mockResolvedValue({
      username: "admin",
      isAdmin: true,
    } as any);

    const { result } = renderHook(() => useSession(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.username).toBe("admin");
    expect(result.current.data?.isAdmin).toBe(true);
  });

  it("useRenovateStatus queries status and useTriggerRenovateRun triggers run", async () => {
    const { renovateClient } = await import("./transport");
    vi.spyOn(renovateClient, "getRenovateStatus").mockResolvedValue({
      lastRun: {
        id: 42n,
        poolId: 1n,
        status: "success",
        startedAt: "2026-09-04T00:00:00Z",
        completedAt: "2026-09-04T00:02:00Z",
        summary: "1 PR created",
      },
      nextScheduledRun: "2026-09-05T03:00:00Z",
    } as any);

    vi.spyOn(renovateClient, "triggerRenovateRun").mockResolvedValue({
      success: true,
      runId: 43n,
    } as any);

    const { useRenovateStatus, useTriggerRenovateRun } = await import("./query-hooks");

    const { result: statusResult } = renderHook(() => useRenovateStatus(1n), { wrapper });
    await waitFor(() => expect(statusResult.current.isSuccess).toBe(true));
    expect(statusResult.current.data?.lastRun?.id).toBe(42n);

    const { result: triggerResult } = renderHook(() => useTriggerRenovateRun(), { wrapper });
    const triggerRes = await triggerResult.current.mutateAsync(1n);
    expect(triggerRes.success).toBe(true);
    expect(triggerRes.runId).toBe(43n);
  });

  it("useCompleteOnboarding invokes completeOnboarding and invalidates queries", async () => {
    vi.spyOn(onboardingClient, "completeOnboarding").mockResolvedValue({
      success: true,
    } as any);

    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { useCompleteOnboarding } = await import("./query-hooks");

    const { result } = renderHook(() => useCompleteOnboarding(), { wrapper });
    const res = await result.current.mutateAsync();

    expect(res.success).toBe(true);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["onboarding", "status"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["auth", "session"] });
  });
});
