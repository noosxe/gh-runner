import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AppRouter, router } from "./router";

// Mock the query hooks to return instant data without network
vi.mock("./lib/api/query-hooks", () => ({
  useSystemStats: () => ({
    data: { totalActiveRunners: 3, totalIdleRunners: 2 },
    isLoading: false,
  }),
  useSession: () => ({
    data: { username: "admin", isAdmin: true },
    isLoading: false,
  }),
  usePools: () => ({
    data: [
      {
        id: 1n,
        name: "test-pool",
        provider: "github",
        repositoryUrl: "https://github.com/test/repo",
        activeRunners: 1,
        minIdleRunners: 1,
        maxConcurrency: 5,
      },
    ],
    isLoading: false,
  }),
  useJobHistory: () => ({
    data: { jobs: [], totalCount: 0 },
    isLoading: false,
  }),
}));

describe("AppRouter", () => {
  it("renders AppShell with navigation and dashboard overview", async () => {
    await router.load();
    render(<AppRouter />);

    await waitFor(() => {
      expect(screen.getByText("Runnero")).toBeInTheDocument();
      expect(screen.getByText("Supervisor")).toBeInTheDocument();
      expect(screen.getByText("Dashboard Overview")).toBeInTheDocument();
    });
  });
});
