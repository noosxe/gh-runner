import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ProfilesPage } from "./profiles";

let mockProfiles = [
  {
    id: 1n,
    name: "installed-gh-app",
    authMethod: "github_app",
    appId: 1001n,
    installUrl: "https://github.com/apps/installed-app/installations/new",
    installationsCount: 2,
    hasPrivateKey: true,
    hasToken: false,
  },
  {
    id: 2n,
    name: "uninstalled-gh-app",
    authMethod: "github_app",
    appId: 1002n,
    installUrl: "https://github.com/apps/uninstalled-app/installations/new",
    installationsCount: 0,
    hasPrivateKey: true,
    hasToken: false,
  },
  {
    id: 3n,
    name: "personal-pat",
    authMethod: "github_pat",
    appId: 0n,
    installUrl: "",
    installationsCount: 0,
    hasPrivateKey: false,
    hasToken: true,
  },
];

let mockIsLoading = false;
const mockCreateMutateAsync = vi.fn();
const mockDeleteMutateAsync = vi.fn();

vi.mock("../lib/api/query-hooks", () => ({
  useAuthProfiles: () => ({
    data: mockProfiles,
    isLoading: mockIsLoading,
  }),
  useCreateAuthProfile: () => ({
    mutateAsync: mockCreateMutateAsync,
    isPending: false,
  }),
  useDeleteAuthProfile: () => ({
    mutateAsync: mockDeleteMutateAsync,
    isPending: false,
  }),
}));

describe("ProfilesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsLoading = false;
    mockCreateMutateAsync.mockResolvedValue({
      profile: {
        id: 10n,
        name: "new-profile",
        authMethod: "github_app",
        installUrl: "https://github.com/apps/new-profile/installations/new",
        installationsCount: 0,
      },
    });
    mockDeleteMutateAsync.mockResolvedValue({});
  });

  it("renders profiles with installation status badges and action links", () => {
    render(<ProfilesPage />);

    expect(screen.getByText("Git Auth Profiles")).toBeInTheDocument();
    expect(screen.getByText("installed-gh-app")).toBeInTheDocument();
    expect(screen.getByText("uninstalled-gh-app")).toBeInTheDocument();
    expect(screen.getByText("personal-pat")).toBeInTheDocument();

    // Installed GitHub App
    expect(screen.getByText("Installed on 2 accounts")).toBeInTheDocument();
    const configureLink = screen.getByRole("link", { name: /Configure Access/i });
    expect(configureLink).toBeInTheDocument();
    expect(configureLink).toHaveAttribute(
      "href",
      "https://github.com/apps/installed-app/installations/new",
    );
    expect(configureLink).toHaveAttribute("target", "_blank");

    // Uninstalled GitHub App
    expect(screen.getByText("Not Installed")).toBeInTheDocument();
    const installLink = screen.getByRole("link", { name: /Install App/i });
    expect(installLink).toBeInTheDocument();
    expect(installLink).toHaveAttribute(
      "href",
      "https://github.com/apps/uninstalled-app/installations/new",
    );
    expect(installLink).toHaveAttribute("target", "_blank");
  });

  it("deletes a profile after user confirmation", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);

    render(<ProfilesPage />);

    const deleteButtons = screen.getAllByRole("button", { name: /Delete Profile/i });
    fireEvent.click(deleteButtons[0]);

    expect(window.confirm).toHaveBeenCalledWith(
      'Are you sure you want to delete authentication profile "installed-gh-app"?',
    );
    await waitFor(() => {
      expect(mockDeleteMutateAsync).toHaveBeenCalledWith(1n);
    });
  });

  it("opens modal and submits new auth profile", async () => {
    render(<ProfilesPage />);

    fireEvent.click(screen.getByRole("button", { name: /Add Auth Profile/i }));
    expect(screen.getByRole("heading", { name: /Add Git Auth Profile/i })).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/e\.g\. github-production/i), {
      target: { value: "test-pat-profile" },
    });
    fireEvent.change(screen.getByPlaceholderText(/ghp_\.\.\. or gitea_pat_\.\.\./i), {
      target: { value: "ghp_secrettoken123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /Save Profile/i }));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "test-pat-profile",
          authMethod: "github_pat",
          token: "ghp_secrettoken123",
        }),
      );
    });
  });
});
