import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CreatePoolWizardModal } from "./create-pool-wizard-modal";

const mockMutateAsync = vi.fn();
let mockDiscoveredTargets: Array<{
  name: string;
  fullName: string;
  htmlUrl: string;
  description: string;
  isPrivate: boolean;
  avatarUrl: string;
}> = [];
let mockIsDiscovering = false;
let mockDiscoveryError: Error | null = null;
let mockInstallUrl = "";
let mockInstallations: Array<{
  id: bigint;
  accountLogin: string;
  accountType: string;
  htmlUrl: string;
  repositorySelection: string;
}> = [];
const mockRefetchDiscovery = vi.fn();

vi.mock("../../lib/api/query-hooks", () => ({
  useCreatePool: () => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  }),
  useDiscoverTargets: () => ({
    data: {
      targets: mockDiscoveredTargets,
      installUrl: mockInstallUrl,
      installations: mockInstallations,
    },
    isLoading: mockIsDiscovering,
    error: mockDiscoveryError,
    refetch: mockRefetchDiscovery,
  }),
}));

describe("CreatePoolWizardModal", () => {
  const defaultAuthProfiles = [
    { id: 10n, name: "corp-github-app", authMethod: "github-app" },
    { id: 20n, name: "internal-forgejo", authMethod: "forgejo-token" },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutateAsync.mockResolvedValue({ pool: { id: 101n } });
    mockInstallUrl = "";
    mockInstallations = [];
    mockDiscoveredTargets = [
      {
        name: "frontend-monorepo",
        fullName: "acme-corp/frontend-monorepo",
        htmlUrl: "https://github.com/acme-corp/frontend-monorepo",
        description: "Primary frontend application",
        isPrivate: true,
        avatarUrl: "",
      },
      {
        name: "backend-core",
        fullName: "acme-corp/backend-core",
        htmlUrl: "https://github.com/acme-corp/backend-core",
        description: "Core backend services",
        isPrivate: false,
        avatarUrl: "",
      },
    ];
    mockIsDiscovering = false;
    mockDiscoveryError = null;
  });

  it("does not render when isOpen is false", () => {
    const { container } = render(
      <CreatePoolWizardModal isOpen={false} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("validates pool name slug and enforces profile selection on Step 1", () => {
    render(
      <CreatePoolWizardModal isOpen={true} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );

    expect(screen.getByText("Create Runner Pool Wizard")).toBeInTheDocument();
    expect(screen.getByText("Identity & Auth")).toBeInTheDocument();

    const continueButton = screen.getByRole("button", {
      name: /Continue to Scope & Targets/i,
    });
    expect(continueButton).toBeDisabled();

    // Invalid slug with uppercase or spaces
    const nameInput = screen.getByLabelText(/Pool Name \(Slug\)/i);
    fireEvent.change(nameInput, { target: { value: "INVALID POOL" } });
    expect(continueButton).toBeDisabled();

    // Valid slug
    fireEvent.change(nameInput, { target: { value: "arm64-prod-workers" } });
    expect(continueButton).toBeEnabled();
  });

  it("supports multi-target selection and 'Select All Filtered' on Step 2", () => {
    render(
      <CreatePoolWizardModal isOpen={true} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );

    // Step 1 -> Step 2
    fireEvent.change(screen.getByLabelText(/Pool Name \(Slug\)/i), {
      target: { value: "ci-pool" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Scope & Targets/i }));

    expect(screen.getByText("Scope & Discovery")).toBeInTheDocument();
    expect(screen.getByText("acme-corp/frontend-monorepo")).toBeInTheDocument();
    expect(screen.getByText("acme-corp/backend-core")).toBeInTheDocument();

    // "Continue to Specifications" should be disabled when 0 selected
    const continueBtn = screen.getByRole("button", { name: /Continue to Specifications/i });
    expect(continueBtn).toBeDisabled();

    // Click "Select All Filtered"
    fireEvent.click(screen.getByText("Select All Filtered"));
    expect(screen.getByText(/2 Repositories Selected/i)).toBeInTheDocument();
    expect(continueBtn).toBeEnabled();

    // Deselect one by clicking its card
    fireEvent.click(screen.getByText("acme-corp/frontend-monorepo"));
    expect(screen.getByText(/1 Repositories Selected/i)).toBeInTheDocument();
    expect(continueBtn).toBeEnabled();
  });

  it("completes full 4-step wizard and submits multi-target pool configuration", async () => {
    const handleClose = vi.fn();
    render(
      <CreatePoolWizardModal
        isOpen={true}
        onClose={handleClose}
        authProfiles={defaultAuthProfiles}
        hostOs="linux"
        hostArch="amd64"
      />,
    );

    // Step 1: Identity
    fireEvent.change(screen.getByLabelText(/Pool Name \(Slug\)/i), {
      target: { value: "multi-target-ci" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Continue to Scope & Targets/i }));

    // Step 2: Select all targets
    fireEvent.click(screen.getByText("Select All Filtered"));
    fireEvent.click(screen.getByRole("button", { name: /Continue to Specifications/i }));

    // Step 3: Quotas & specs
    expect(screen.getByText("Runner Specs")).toBeInTheDocument();
    const labelsInput = screen.getByLabelText(/Runner Labels/i) as HTMLInputElement;
    expect(labelsInput.value).toBe("self-hosted,linux,amd64");

    fireEvent.change(screen.getByLabelText(/Min Idle Warm Runners/i), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText(/Max Concurrency/i), { target: { value: "8" } });
    fireEvent.click(screen.getByRole("button", { name: /Review & Confirm/i }));

    // Step 4: Review
    expect(screen.getByText("Review & Create")).toBeInTheDocument();
    expect(screen.getByText("multi-target-ci")).toBeInTheDocument();
    expect(screen.getByText("https://github.com/acme-corp/frontend-monorepo")).toBeInTheDocument();
    expect(screen.getByText("https://github.com/acme-corp/backend-core")).toBeInTheDocument();

    // Submit
    fireEvent.click(screen.getByRole("button", { name: /Create Runner Pool/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledTimes(1);
    });

    const submitted = mockMutateAsync.mock.calls[0][0].pool;
    expect(submitted.name).toBe("multi-target-ci");
    expect(submitted.provider).toBe("github");
    expect(submitted.scope).toBe("repo");
    expect(submitted.repositoryUrl).toBe("https://github.com/acme-corp/frontend-monorepo");
    expect(submitted.targetUrls).toEqual([
      "https://github.com/acme-corp/frontend-monorepo",
      "https://github.com/acme-corp/backend-core",
    ]);
    expect(submitted.minIdleRunners).toBe(2);
    expect(submitted.maxConcurrency).toBe(8);
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it("locks docker-in-docker when provider is Forgejo or Gitea", () => {
    render(
      <CreatePoolWizardModal isOpen={true} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );

    // Switch to internal-forgejo
    const profileSelect = screen.getByLabelText(/Git Authentication Profile/i);
    fireEvent.change(profileSelect, { target: { value: "20" } });
    expect(screen.getByText("forgejo")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Pool Name \(Slug\)/i), {
      target: { value: "forgejo-pool" },
    });
    fireEvent.click(screen.getByText("Continue to Scope & Targets"));

    fireEvent.click(screen.getByText("Select All Filtered"));
    fireEvent.click(screen.getByText("Continue to Specifications"));

    // Check Docker checkbox is disabled and locked
    const dockerCheckbox = screen.getByRole("checkbox", {
      name: /Enable Docker-in-Docker socket access/i,
    }) as HTMLInputElement;
    expect(dockerCheckbox.checked).toBe(true);
    expect(dockerCheckbox.disabled).toBe(true);
  });

  it("renders guided GitHub App installation callout when 0 targets discovered and installUrl is present", () => {
    mockDiscoveredTargets = [];
    mockInstallUrl = "https://github.com/apps/my-app/installations/new";

    render(
      <CreatePoolWizardModal isOpen={true} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );

    fireEvent.change(screen.getByLabelText(/Pool Name \(Slug\)/i), {
      target: { value: "test-pool" },
    });
    fireEvent.click(screen.getByText("Continue to Scope & Targets"));

    expect(screen.getByText("GitHub App Not Installed Yet")).toBeInTheDocument();
    const installBtn = screen.getByRole("link", {
      name: /Install GitHub App on Your Account/i,
    });
    expect(installBtn).toBeInTheDocument();
    expect(installBtn).toHaveAttribute("href", "https://github.com/apps/my-app/installations/new");
    expect(installBtn).toHaveAttribute("target", "_blank");
    expect(installBtn).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("renders 'Manage Access in GitHub' button linking to installation settings when targets are present", () => {
    mockInstallUrl = "https://github.com/apps/my-app/installations/new";
    mockInstallations = [
      {
        id: 12345n,
        accountLogin: "acme-corp",
        accountType: "Organization",
        htmlUrl: "https://github.com/organizations/acme-corp/settings/installations/12345",
        repositorySelection: "selected",
      },
    ];

    render(
      <CreatePoolWizardModal isOpen={true} onClose={vi.fn()} authProfiles={defaultAuthProfiles} />,
    );

    fireEvent.change(screen.getByLabelText(/Pool Name \(Slug\)/i), {
      target: { value: "test-pool" },
    });
    fireEvent.click(screen.getByText("Continue to Scope & Targets"));

    const manageBtn = screen.getByRole("link", {
      name: /Manage Access in GitHub/i,
    });
    expect(manageBtn).toBeInTheDocument();
    expect(manageBtn).toHaveAttribute(
      "href",
      "https://github.com/organizations/acme-corp/settings/installations/12345",
    );
    expect(manageBtn).toHaveAttribute("target", "_blank");
    expect(manageBtn).toHaveAttribute("rel", "noopener noreferrer");
  });
});
