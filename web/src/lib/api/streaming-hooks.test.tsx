import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useStreamSubscription, useWatchDashboard, useWatchPools } from "./streaming-hooks";
import { queryKeys } from "./query-hooks";
import { analyticsClient, poolClient } from "./transport";

vi.mock("./transport", () => ({
  analyticsClient: {
    watchDashboard: vi.fn(),
  },
  poolClient: {
    watchPools: vi.fn(),
  },
  logClient: {
    streamRunnerLogs: vi.fn(),
  },
}));

function createTestWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return {
    queryClient,
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  };
}

describe("streaming-hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useStreamSubscription", () => {
    it("connects and receives streamed items", async () => {
      const messages = ["msg1", "msg2"];
      async function* mockStream(_signal: AbortSignal) {
        for (const msg of messages) {
          yield msg;
        }
      }

      const streamFn = vi.fn((signal: AbortSignal) => mockStream(signal));
      const onData = vi.fn();

      const { result } = renderHook(() =>
        useStreamSubscription(streamFn, onData, { initialBackoffMs: 50 }),
      );

      await waitFor(() => {
        expect(onData).toHaveBeenCalledWith("msg1");
        expect(onData).toHaveBeenCalledWith("msg2");
        expect(result.current.isConnected).toBe(true);
      });
    });

    it("aborts stream when unmounted", async () => {
      let capturedSignal: AbortSignal | null = null;
      // An infinite generator that waits for abort
      async function* infiniteStream(signal: AbortSignal) {
        capturedSignal = signal;
        while (!signal.aborted) {
          yield "ping";
          await new Promise((r) => setTimeout(r, 100));
        }
      }

      const streamFn = vi.fn((signal: AbortSignal) => infiniteStream(signal));
      const onData = vi.fn();

      const { unmount } = renderHook(() => useStreamSubscription(streamFn, onData));

      await waitFor(() => {
        expect(onData).toHaveBeenCalledWith("ping");
      });

      unmount();
      expect((capturedSignal as AbortSignal | null)?.aborted).toBe(true);
    });

    it("transitions to reconnecting on error", async () => {
      async function* errorStream(_signal: AbortSignal) {
        yield "pre-error";
        throw new Error("stream connection lost");
      }

      const streamFn = vi.fn((signal: AbortSignal) => errorStream(signal));
      const onData = vi.fn();
      const onError = vi.fn();

      const { result } = renderHook(() =>
        useStreamSubscription(streamFn, onData, {
          initialBackoffMs: 50,
          onError,
        }),
      );

      await waitFor(() => {
        expect(onError).toHaveBeenCalled();
        expect(result.current.isReconnecting).toBe(true);
      });
    });
  });

  describe("useWatchDashboard", () => {
    it("populates query cache with systemStats and pools", async () => {
      const { queryClient, wrapper } = createTestWrapper();

      const mockSnapshot = {
        stats: {
          totalActiveRunners: 7,
          totalIdleRunners: 3,
          averageQueueTimeSeconds: 1.2,
          totalJobs24h: 42,
          successfulJobs24h: 40,
          failedJobs24h: 2,
          averageRuntimeSeconds: 35.5,
          successRatePercent: 95.2,
        },
        pools: [
          {
            id: 10n,
            name: "prod-pool",
            activeRunners: 5,
            idleRunners: 2,
          },
        ],
        recentJobs: [
          {
            id: 1n,
            runnerName: "runner-1",
            status: "success",
          },
        ],
      };

      async function* mockDashboardStream(_signal: AbortSignal) {
        yield mockSnapshot;
      }

      vi.mocked(analyticsClient.watchDashboard).mockReturnValue(
        mockDashboardStream(new AbortController().signal) as any,
      );

      const { result } = renderHook(() => useWatchDashboard(), { wrapper });

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });

      const cachedStats = queryClient.getQueryData(queryKeys.systemStats);
      expect(cachedStats).toEqual(mockSnapshot.stats);

      const cachedPools = queryClient.getQueryData(queryKeys.pools);
      expect(cachedPools).toEqual(mockSnapshot.pools);

      const cachedPool = queryClient.getQueryData(queryKeys.pool(10n));
      expect(cachedPool).toEqual(mockSnapshot.pools[0]);
    });
  });

  describe("useWatchPools", () => {
    it("populates query cache with watched pools", async () => {
      const { queryClient, wrapper } = createTestWrapper();

      const mockSnapshot = {
        pools: [
          {
            id: 25n,
            name: "arm64-pool",
            activeRunners: 2,
            idleRunners: 1,
          },
        ],
      };

      async function* mockPoolStream(_signal: AbortSignal) {
        yield mockSnapshot;
      }

      vi.mocked(poolClient.watchPools).mockReturnValue(
        mockPoolStream(new AbortController().signal) as any,
      );

      const { result } = renderHook(() => useWatchPools(), { wrapper });

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });

      const cachedPools = queryClient.getQueryData(queryKeys.pools);
      expect(cachedPools).toEqual(mockSnapshot.pools);

      const cachedPool = queryClient.getQueryData(queryKeys.pool(25n));
      expect(cachedPool).toEqual(mockSnapshot.pools[0]);
    });
  });
});
