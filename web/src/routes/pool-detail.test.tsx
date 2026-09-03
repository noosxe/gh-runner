import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PoolDetailPage } from "./pool-detail";

const mockPool = {
  id: 10n,
  name: "arm64-prod-pool",
  provider: "github",
  repositoryUrl: "https://github.com/noosxe/gh-runner",
  scope: "repo",
  minIdleRunners: 1,
  maxConcurrency: 5,
  activeRunners: 2,
  cpuLimit: "4",
  memoryLimit: "8G",
  allowDocker: true,
  runnerImage: "ghcr.io/noosxe/runner-aio:latest",
  maxRunnerLifetimeSeconds: 7200,
};

const mockRunners = [
  {
    containerId: "cnt-alpha-1234567890",
    name: "ghrs-arm64-alpha",
    poolName: "arm64-prod-pool",
    status: "busy",
    ipAddress: "172.18.0.4",
    uptimeSeconds: 125,
    spawnedAt: "2026-09-04T00:00:00Z",
    cpuLimit: "4",
    memoryLimit: "8G",
  },
  {
    containerId: "cnt-beta-1234567890",
    name: "ghrs-arm64-beta",
    poolName: "arm64-prod-pool",
    status: "idle",
    ipAddress: "172.18.0.5",
    uptimeSeconds: 600,
    spawnedAt: "2026-09-04T00:00:00Z",
    cpuLimit: "4",
    memoryLimit: "8G",
  },
];

const mockMutateAsync = vi.fn();

vi.mock("../lib/api/query-hooks", () => ({
  usePools: () => ({
    data: [mockPool],
    isLoading: false,
  }),
  useRunners: () => ({
    data: mockRunners,
    isLoading: false,
  }),
  useTerminateRunner: () => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  }),
}));

vi.mock("../lib/api/streaming-hooks", () => ({
  useWatchRunners: () => ({
    isConnected: true,
  }),
  useStreamRunnerLogs: () => ({
    logs: [{ content: "Runner connected to GitHub\n" }],
    isConnected: true,
  }),
}));

vi.mock("@tanstack/react-router", () => ({
  useParams: () => ({ poolId: "10" }),
  Link: ({ children, to, ...props }: any) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

describe("PoolDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders pool overview, live badges, and active runners table", () => {
    render(<PoolDetailPage />);

    expect(screen.getByText("arm64-prod-pool")).toBeInTheDocument();
    expect(screen.getByText("Live Orchestrator Stream")).toBeInTheDocument();
    expect(screen.getByText("Active Running Jobs")).toBeInTheDocument();
    expect(screen.getByText("cnt-alpha-12")).toBeInTheDocument();
    expect(screen.getByText("ghrs-arm64-alpha")).toBeInTheDocument();
    expect(screen.getByText("busy")).toBeInTheDocument();
    expect(screen.getByText("idle")).toBeInTheDocument();
    expect(screen.getByText("2m 5s")).toBeInTheDocument();
  });

  it("opens live runner logs viewer modal", () => {
    render(<PoolDetailPage />);

    const logsButtons = screen.getAllByRole("button", { name: /logs/i });
    fireEvent.click(logsButtons[0]);

    expect(screen.getByText("Live Stream")).toBeInTheDocument();
    expect(screen.getByText("Runner connected to GitHub")).toBeInTheDocument();
  });

  it("opens terminate confirmation dialog and triggers terminate mutation", async () => {
    mockMutateAsync.mockResolvedValueOnce({});
    render(<PoolDetailPage />);

    const termButtons = screen.getAllByRole("button", { name: /terminate/i });
    fireEvent.click(termButtons[0]);

    expect(screen.getByText("Terminate Runner Instance?")).toBeInTheDocument();

    const confirmBtn = screen.getByRole("button", {
      name: "Terminate Instance",
    });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        poolId: 10n,
        containerId: "cnt-alpha-1234567890",
      });
    });
  });
});
