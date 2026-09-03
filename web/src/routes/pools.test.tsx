import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PoolsPage } from "./pools";

const mockPools = [
  {
    id: 1n,
    name: "arm64-prod-pool",
    provider: "github",
    repositoryUrl: "https://github.com/noosxe/gh-runner",
    scope: "repo",
    minIdleRunners: 2,
    maxConcurrency: 10,
    activeRunners: 3,
    cpuLimit: "4",
    memoryLimit: "8G",
    allowDocker: true,
    runnerImage: "ghcr.io/noosxe/runner-aio:latest",
    maxRunnerLifetimeSeconds: 7200,
  },
  {
    id: 2n,
    name: "gitea-org-pool",
    provider: "gitea",
    repositoryUrl: "https://gitea.example.com/devops",
    scope: "org",
    minIdleRunners: 1,
    maxConcurrency: 5,
    activeRunners: 0,
    cpuLimit: "2",
    memoryLimit: "4G",
    allowDocker: false,
    runnerImage: "",
    maxRunnerLifetimeSeconds: 3600,
  },
];

let mockIsLoading = false;
let mockIsConnected = true;

vi.mock("../lib/api/query-hooks", () => ({
  usePools: () => ({
    data: mockIsLoading ? undefined : mockPools,
    isLoading: mockIsLoading,
  }),
}));

vi.mock("../lib/api/streaming-hooks", () => ({
  useWatchPools: () => ({
    isConnected: mockIsConnected,
  }),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...props }: any) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

describe("PoolsPage", () => {
  beforeEach(() => {
    mockIsLoading = false;
    mockIsConnected = true;
    vi.clearAllMocks();
  });

  it("renders pool cards with capacity utilization and quotas", () => {
    render(<PoolsPage />);

    expect(screen.getByText("Runner Pools")).toBeInTheDocument();
    expect(screen.getByText("Live Stream")).toBeInTheDocument();
    expect(screen.getByText("arm64-prod-pool")).toBeInTheDocument();
    expect(screen.getByText("gitea-org-pool")).toBeInTheDocument();
    expect(screen.getByText("3 / 10 (30%)")).toBeInTheDocument();
    expect(screen.getByText("4 CPU")).toBeInTheDocument();
    expect(screen.getByText("8G Mem")).toBeInTheDocument();
    expect(screen.getByText("Docker Enabled")).toBeInTheDocument();
  });

  it("filters pools by search term", () => {
    render(<PoolsPage />);

    const searchInput = screen.getByPlaceholderText("Search pools by name or repository URL...");
    fireEvent.change(searchInput, { target: { value: "gitea" } });

    expect(screen.queryByText("arm64-prod-pool")).not.toBeInTheDocument();
    expect(screen.getByText("gitea-org-pool")).toBeInTheDocument();
  });

  it("filters pools by provider", () => {
    render(<PoolsPage />);

    const select = screen.getAllByRole("combobox")[0];
    fireEvent.change(select, { target: { value: "github" } });

    expect(screen.getByText("arm64-prod-pool")).toBeInTheDocument();
    expect(screen.queryByText("gitea-org-pool")).not.toBeInTheDocument();
  });
});
