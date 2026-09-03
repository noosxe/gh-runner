import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { OnboardingPage } from "./onboarding";

const mockSetupAdmin = vi.fn();
const mockCreateAuthProfile = vi.fn();
const mockSetAppSetting = vi.fn();

let mockOnboardingStatus = {
  adminCreated: false,
  authProfileExists: false,
  poolExists: false,
  setupComplete: false,
};

vi.mock("../lib/api/query-hooks", () => ({
  useOnboardingStatus: () => ({
    data: mockOnboardingStatus,
    isLoading: false,
  }),
  useSetupAdmin: () => ({
    mutateAsync: mockSetupAdmin,
    isPending: false,
  }),
  useCreateAuthProfile: () => ({
    mutateAsync: mockCreateAuthProfile,
    isPending: false,
  }),
  useSetAppSetting: () => ({
    mutateAsync: mockSetAppSetting,
    isPending: false,
  }),
}));

describe("OnboardingPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockOnboardingStatus = {
      adminCreated: false,
      authProfileExists: false,
      poolExists: false,
      setupComplete: false,
    };
  });

  it("renders Step 1 for uninitialized supervisor", () => {
    render(<OnboardingPage />);

    expect(screen.getByText("System Onboarding")).toBeInTheDocument();
    expect(screen.getByText("Step 1 of 5: Create Master Administrator")).toBeInTheDocument();
    expect(screen.getByLabelText("Admin Username")).toHaveValue("admin");
  });

  it("enforces password length and confirmation in Step 1", async () => {
    render(<OnboardingPage />);

    const passwordInput = screen.getByLabelText("Password (min 10 characters)");
    const confirmInput = screen.getByLabelText("Confirm Password");
    const nextBtn = screen.getByRole("button", { name: /Next: Git Provider/i });

    // Short password
    fireEvent.change(passwordInput, { target: { value: "short" } });
    fireEvent.change(confirmInput, { target: { value: "short" } });
    fireEvent.click(nextBtn);

    await waitFor(() => {
      expect(screen.getByText("Password must be at least 10 characters long")).toBeInTheDocument();
    });

    // Mismatched passwords
    fireEvent.change(passwordInput, { target: { value: "validpassword1" } });
    fireEvent.change(confirmInput, { target: { value: "validpassword2" } });
    fireEvent.click(nextBtn);

    await waitFor(() => {
      expect(screen.getByText("Passwords do not match")).toBeInTheDocument();
    });
  });

  it("advances through Steps 1, 2, and 3 successfully", async () => {
    mockSetupAdmin.mockResolvedValueOnce({});
    mockCreateAuthProfile.mockResolvedValueOnce({ id: 1n });
    mockSetAppSetting.mockResolvedValue({});

    render(<OnboardingPage />);

    // --- STEP 1: Admin Setup ---
    const passwordInput = screen.getByLabelText("Password (min 10 characters)");
    const confirmInput = screen.getByLabelText("Confirm Password");
    fireEvent.change(passwordInput, { target: { value: "longenoughpass123" } });
    fireEvent.change(confirmInput, { target: { value: "longenoughpass123" } });

    fireEvent.click(screen.getByRole("button", { name: /Next: Git Provider/i }));

    await waitFor(() => {
      expect(mockSetupAdmin).toHaveBeenCalledWith({
        username: "admin",
        password: "longenoughpass123",
      });
      expect(screen.getByText("Step 2 of 5: Connect Git Provider")).toBeInTheDocument();
    });

    // --- STEP 2: Git Provider Setup ---
    const tokenInput = screen.getByLabelText("Personal Access Token (PAT)");
    fireEvent.change(tokenInput, { target: { value: "ghp_testtoken12345" } });

    fireEvent.click(screen.getByRole("button", { name: /Next: Safeguards/i }));

    await waitFor(() => {
      expect(mockCreateAuthProfile).toHaveBeenCalledWith({
        name: "github-primary",
        authMethod: "github_pat",
        appId: 0n,
        privateKey: new Uint8Array(),
        token: "ghp_testtoken12345",
      });
      expect(screen.getByText("Step 3 of 5: Global Scaling Safeguards")).toBeInTheDocument();
    });

    // --- STEP 3: Safeguards Setup ---
    const maxRunnersInput = screen.getByLabelText("Total Allowed Runners");
    fireEvent.change(maxRunnersInput, { target: { value: "25" } });

    fireEvent.click(screen.getByRole("button", { name: /Next: Initial Pool/i }));

    await waitFor(() => {
      expect(mockSetAppSetting).toHaveBeenCalled();
      expect(screen.getByText("Steps 1–3 Complete!")).toBeInTheDocument();
    });
  });

  it("supports navigating back across wizard steps", async () => {
    mockSetupAdmin.mockResolvedValueOnce({});
    render(<OnboardingPage />);

    // Step 1 -> Step 2
    fireEvent.change(screen.getByLabelText("Password (min 10 characters)"), {
      target: { value: "longenoughpass123" },
    });
    fireEvent.change(screen.getByLabelText("Confirm Password"), {
      target: { value: "longenoughpass123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Next: Git Provider/i }));

    await waitFor(() => {
      expect(screen.getByText("Step 2 of 5: Connect Git Provider")).toBeInTheDocument();
    });

    // Click Back to return to Step 1
    fireEvent.click(screen.getByRole("button", { name: /Back/i }));

    await waitFor(() => {
      expect(screen.getByText("Step 1 of 5: Create Master Administrator")).toBeInTheDocument();
    });
  });
});
