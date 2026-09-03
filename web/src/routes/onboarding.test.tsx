import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { OnboardingPage } from "./onboarding";

const mockSetupAdmin = vi.fn();
const mockCreateAuthProfile = vi.fn();
const mockSetAppSetting = vi.fn();
const mockCreatePool = vi.fn();
const mockNavigate = vi.fn();

let mockOnboardingStatus = {
  adminCreated: false,
  authProfileExists: false,
  poolExists: false,
  setupComplete: false,
};

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

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
  useCreatePool: () => ({
    mutateAsync: mockCreatePool,
    isPending: false,
  }),
}));

describe("OnboardingPage (Full 5 Steps)", () => {
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

  it("advances through Steps 1 to 5 and launches supervisor", async () => {
    mockSetupAdmin.mockResolvedValueOnce({});
    mockCreateAuthProfile.mockResolvedValueOnce({ profile: { id: 42n } });
    mockSetAppSetting.mockResolvedValue({});
    mockCreatePool.mockResolvedValueOnce({ pool: { id: 101n } });

    render(<OnboardingPage />);

    // --- STEP 1: Admin Setup ---
    fireEvent.change(screen.getByLabelText("Password (min 10 characters)"), {
      target: { value: "longenoughpass123" },
    });
    fireEvent.change(screen.getByLabelText("Confirm Password"), {
      target: { value: "longenoughpass123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Next: Git Provider/i }));

    await waitFor(() => {
      expect(mockSetupAdmin).toHaveBeenCalledWith({
        username: "admin",
        password: "longenoughpass123",
      });
      expect(screen.getByText("Step 2 of 5: Connect Git Provider")).toBeInTheDocument();
    });

    // --- STEP 2: Git Provider Setup ---
    fireEvent.change(screen.getByLabelText("Personal Access Token (PAT)"), {
      target: { value: "ghp_testtoken12345" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Next: Safeguards/i }));

    await waitFor(() => {
      expect(mockCreateAuthProfile).toHaveBeenCalled();
      expect(screen.getByText("Step 3 of 5: Global Scaling Safeguards")).toBeInTheDocument();
    });

    // --- STEP 3: Safeguards Setup ---
    fireEvent.click(screen.getByRole("button", { name: /Next: Initial Pool/i }));

    await waitFor(() => {
      expect(mockSetAppSetting).toHaveBeenCalled();
      expect(screen.getByText("Step 4 of 5: Initial Runner Pool Setup")).toBeInTheDocument();
    });

    // --- STEP 4: Initial Pool Setup ---
    expect(screen.getByLabelText("Pool Name")).toHaveValue("default-pool");
    expect(screen.getByLabelText("Repository / Organization URL")).toBeInTheDocument();

    // Enable Renovate toggle
    const renovateCheckbox = screen.getByLabelText("Enable Renovate Dependency Automation");
    fireEvent.click(renovateCheckbox);
    expect(screen.getByLabelText("Cron Schedule")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Next: Review & Launch/i }));

    await waitFor(() => {
      expect(screen.getByText("Step 5 of 5: Review & Launch Supervisor")).toBeInTheDocument();
      expect(screen.getByText("Ready to Launch")).toBeInTheDocument();
    });

    // --- STEP 5: Confirm & Launch ---
    const launchBtn = screen.getByRole("button", { name: /Confirm & Launch Supervisor/i });
    fireEvent.click(launchBtn);

    await waitFor(() => {
      expect(mockCreatePool).toHaveBeenCalledWith(
        expect.objectContaining({
          pool: expect.objectContaining({
            name: "default-pool",
            provider: "github",
            authProfileId: 42n,
            allowDocker: true,
            renovate: expect.objectContaining({
              enabled: true,
              cronSchedule: "0 2 * * *",
            }),
          }),
        }),
      );
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    });
  });

  it("locks allowDocker to enabled when Gitea or Forgejo provider is selected", async () => {
    mockSetupAdmin.mockResolvedValueOnce({});
    mockCreateAuthProfile.mockResolvedValueOnce({ profile: { id: 7n } });
    mockSetAppSetting.mockResolvedValue({});

    render(<OnboardingPage />);

    // Step 1
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

    // Select Gitea PAT
    fireEvent.click(screen.getByRole("button", { name: "Gitea PAT" }));
    fireEvent.change(screen.getByLabelText("Personal Access Token (PAT)"), {
      target: { value: "gitea_token_abc" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Next: Safeguards/i }));

    await waitFor(() => {
      expect(screen.getByText("Step 3 of 5: Global Scaling Safeguards")).toBeInTheDocument();
    });

    // Step 3 -> Step 4
    fireEvent.click(screen.getByRole("button", { name: /Next: Initial Pool/i }));

    await waitFor(() => {
      expect(screen.getByText("Step 4 of 5: Initial Runner Pool Setup")).toBeInTheDocument();
    });

    // Verify Docker policy lock per docs/05 §4
    const dockerCheckbox = screen.getByLabelText("Allow Docker in Container") as HTMLInputElement;
    expect(dockerCheckbox.checked).toBe(true);
    expect(dockerCheckbox.disabled).toBe(true);
    expect(screen.getByText(/Locked to Enabled for GITEA runners/i)).toBeInTheDocument();
  });
});
