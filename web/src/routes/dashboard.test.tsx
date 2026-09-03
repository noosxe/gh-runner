import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DashboardPage } from "./dashboard";

const mockStats = {
  totalActiveRunners: 3,
  totalIdleRunners: 2,
  averageQueueTimeSeconds: 4.2,
  totalJobs24h: 142,
  successfulJobs24h: 139,
  failedJobs24h: 3,
  averageRuntimeSeconds: 192.0,
  successRatePercent: 97.9,
  queueLatencyTrend: [
    {
      timestamp: "2026-09-04T00:00:00Z",
      avgQueueSeconds: 3.5,
      avgRuntimeSeconds: 180.0,
      totalJobs: 20,
      successfulJobs: 20,
      failedJobs: 0,
    },
  ],
};

const mockPools = [
  {
    id: 1n,
    name: "pool-arm64-prod",
    provider: "github",
    repositoryUrl: "https://github.com/org/repo",
    activeRunners: 2,
    minIdleRunners: 1,
    maxConcurrency: 10,
  },
];

const mockHistory = {
  jobs: [
    {
      id: 101n,
      runnerName: "ghrs-arm64-prod-a8f12c",
      status: "success",
      durationSeconds: 165.0,
      queueTimeSeconds: 3.2,
      completedAt: "2026-09-04T01:00:00Z",
    },
  ],
};

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...props }: any) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("../lib/api/query-hooks", () => ({
  useSystemStats: () => ({
    data: mockStats,
    isLoading: false,
  }),
  usePools: () => ({
    data: mockPools,
    isLoading: false,
  }),
  useJobHistory: () => ({
    data: mockHistory,
    isLoading: false,
  }),
}));

describe("DashboardPage", () => {
  it("renders KPI cards, analytics graphs, runner pools, and recent executions", () => {
    render(<DashboardPage />);

    expect(screen.getByText("Dashboard Overview")).toBeInTheDocument();
    expect(screen.getByText("3 active")).toBeInTheDocument();
    expect(screen.getByText("142")).toBeInTheDocument();
    expect(screen.getAllByText("97.9%").length).toBeGreaterThan(0);
    expect(screen.getAllByText("3m 12s").length).toBeGreaterThan(0);

    // Analytics components
    expect(screen.getByText("Queue Wait-Time Latency")).toBeInTheDocument();
    expect(screen.getByText("Execution Health & Ratio")).toBeInTheDocument();

    // Configured pools
    expect(screen.getByText("pool-arm64-prod")).toBeInTheDocument();

    // Recent executions table
    expect(screen.getByText("ghrs-arm64-prod-a8f12c")).toBeInTheDocument();
  });
});
