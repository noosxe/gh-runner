import { useState, type FormEvent } from "react";
import {
  useOnboardingStatus,
  useSetupAdmin,
  useCreateAuthProfile,
  useSetAppSetting,
} from "../lib/api/query-hooks";
import { useTheme } from "../hooks/use-theme";
import {
  ShieldCheck,
  KeyRound,
  Sliders,
  CheckCircle2,
  AlertCircle,
  Sun,
  Moon,
  Monitor,
  Eye,
  EyeOff,
  ArrowRight,
  ArrowLeft,
  Server,
  Rocket,
} from "lucide-react";

export function OnboardingPage() {
  const { data: status } = useOnboardingStatus();
  const { theme, setTheme } = useTheme();

  // Active step state (1: Admin, 2: Provider, 3: Safeguards, 4: Pool, 5: Review)
  const defaultStep = status?.adminCreated ? (!status.authProfileExists ? 2 : 3) : 1;
  const [stepOverride, setStepOverride] = useState<number | null>(null);
  const currentStep = stepOverride ?? defaultStep;
  const setCurrentStep = setStepOverride;
  const [error, setError] = useState<string | null>(null);

  // Step 1: Admin Credentials State
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  // Step 2: Git Provider State
  const [profileName, setProfileName] = useState("github-primary");
  const [authMethod, setAuthMethod] = useState<
    "github_app" | "github_pat" | "gitea_pat" | "forgejo_pat"
  >("github_pat");
  const [appId, setAppId] = useState("");
  const [privateKeyPem, setPrivateKeyPem] = useState("");
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);

  // Step 3: Global Safeguards State
  const [totalAllowedRunners, setTotalAllowedRunners] = useState(20);
  const [totalIdleWarmPool, setTotalIdleWarmPool] = useState(5);
  const [shutdownTimeoutSeconds, setShutdownTimeoutSeconds] = useState(300);
  const [jobRetentionDays, setJobRetentionDays] = useState(30);

  // Mutations
  const setupAdminMutation = useSetupAdmin();
  const createAuthProfileMutation = useCreateAuthProfile();
  const setAppSettingMutation = useSetAppSetting();

  // Step 1 Submission
  const handleAdminSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password.length < 10) {
      setError("Password must be at least 10 characters long");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    try {
      await setupAdminMutation.mutateAsync({ username, password });
      setCurrentStep(2);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create administrator");
    }
  };

  // Step 2 Submission
  const handleProviderSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!profileName.trim()) {
      setError("Profile name is required");
      return;
    }

    try {
      if (authMethod === "github_app") {
        if (!appId || !privateKeyPem.trim()) {
          setError("GitHub App ID and Private Key PEM are required");
          return;
        }
        const encoder = new TextEncoder();
        await createAuthProfileMutation.mutateAsync({
          name: profileName.trim(),
          authMethod: "github_app",
          appId: BigInt(appId.trim()),
          privateKey: encoder.encode(privateKeyPem.trim()),
          token: "",
        });
      } else {
        if (!token.trim()) {
          setError("Personal Access Token is required");
          return;
        }
        await createAuthProfileMutation.mutateAsync({
          name: profileName.trim(),
          authMethod,
          appId: 0n,
          privateKey: new Uint8Array(),
          token: token.trim(),
        });
      }
      setCurrentStep(3);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to register Git auth profile");
    }
  };

  // Step 3 Submission
  const handleSafeguardsSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      await Promise.all([
        setAppSettingMutation.mutateAsync({
          key: "total_allowed_runners",
          value: String(totalAllowedRunners),
        }),
        setAppSettingMutation.mutateAsync({
          key: "total_idle_warm_pool",
          value: String(totalIdleWarmPool),
        }),
        setAppSettingMutation.mutateAsync({
          key: "shutdown_timeout_seconds",
          value: String(shutdownTimeoutSeconds),
        }),
        setAppSettingMutation.mutateAsync({
          key: "job_retention_days",
          value: String(jobRetentionDays),
        }),
      ]);
      setCurrentStep(4);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save global constraints");
    }
  };

  const steps = [
    { num: 1, label: "Admin", icon: ShieldCheck },
    { num: 2, label: "Git Auth", icon: KeyRound },
    { num: 3, label: "Safeguards", icon: Sliders },
    { num: 4, label: "Initial Pool", icon: Server },
    { num: 5, label: "Review", icon: Rocket },
  ];

  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center p-4 bg-slate-50 text-slate-900 transition-colors dark:bg-slate-950 dark:text-slate-50">
      {/* Theme Switcher */}
      <div className="absolute top-4 right-4 flex items-center rounded-xl border border-slate-200 bg-white p-1 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <button
          type="button"
          onClick={() => setTheme("light")}
          title="Light Theme"
          className={`rounded-lg p-1.5 transition-colors ${
            theme === "light"
              ? "bg-slate-100 text-blue-600 dark:bg-slate-800 dark:text-blue-400"
              : "text-slate-400 hover:text-slate-900 dark:hover:text-slate-200"
          }`}
        >
          <Sun className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={() => setTheme("dark")}
          title="Dark Theme"
          className={`rounded-lg p-1.5 transition-colors ${
            theme === "dark"
              ? "bg-slate-100 text-blue-600 dark:bg-slate-800 dark:text-blue-400"
              : "text-slate-400 hover:text-slate-900 dark:hover:text-slate-200"
          }`}
        >
          <Moon className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={() => setTheme("system")}
          title="System Theme"
          className={`rounded-lg p-1.5 transition-colors ${
            theme === "system"
              ? "bg-slate-100 text-blue-600 dark:bg-slate-800 dark:text-blue-400"
              : "text-slate-400 hover:text-slate-900 dark:hover:text-slate-200"
          }`}
        >
          <Monitor className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* Main Wizard Container */}
      <div className="w-full max-w-2xl rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900 sm:p-8">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-blue-600 text-white shadow-sm">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            System Onboarding
          </h1>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Configure master administrator, connect Git provider, and set concurrency safeguards
          </p>
        </div>

        {/* Step Progress Bar */}
        <div className="mt-8 flex items-center justify-between border-b border-slate-100 pb-6 dark:border-slate-800">
          {steps.map((step) => {
            const Icon = step.icon;
            const isDone = currentStep > step.num;
            const isCurrent = currentStep === step.num;

            return (
              <div key={step.num} className="flex flex-1 flex-col items-center">
                <div
                  className={`flex h-9 w-9 items-center justify-center rounded-xl text-xs font-semibold transition-all ${
                    isDone
                      ? "bg-emerald-600 text-white"
                      : isCurrent
                        ? "bg-blue-600 text-white shadow-sm"
                        : "bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-500"
                  }`}
                >
                  {isDone ? <CheckCircle2 className="h-4 w-4" /> : <Icon className="h-4 w-4" />}
                </div>
                <span
                  className={`mt-1.5 text-[11px] font-medium ${
                    isCurrent
                      ? "font-bold text-blue-600 dark:text-blue-400"
                      : isDone
                        ? "text-slate-700 dark:text-slate-300"
                        : "text-slate-400"
                  }`}
                >
                  {step.label}
                </span>
              </div>
            );
          })}
        </div>

        {/* Error Alert */}
        {error && (
          <div className="mt-6 flex items-center gap-2 rounded-xl bg-rose-50 p-3 text-xs text-rose-700 dark:bg-rose-950/50 dark:text-rose-400">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Step 1: Admin Setup */}
        {currentStep === 1 && (
          <form onSubmit={handleAdminSubmit} className="mt-6 space-y-4 text-xs">
            <div>
              <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                Step 1 of 5: Create Master Administrator
              </h2>
              <p className="mt-0.5 text-slate-500 dark:text-slate-400">
                Administrative credentials are protected with bcrypt password hashing and 24h JWT
                sessions.
              </p>
            </div>

            <div>
              <label
                htmlFor="admin-username"
                className="font-semibold text-slate-700 dark:text-slate-300"
              >
                Admin Username
              </label>
              <input
                id="admin-username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                required
                autoFocus
              />
            </div>

            <div>
              <label
                htmlFor="admin-password"
                className="font-semibold text-slate-700 dark:text-slate-300"
              >
                Password (min 10 characters)
              </label>
              <div className="relative mt-1">
                <input
                  id="admin-password"
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 pr-10 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
                <button
                  type="button"
                  aria-label="Toggle password visibility"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                  tabIndex={-1}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>

            <div>
              <label
                htmlFor="admin-confirm-password"
                className="font-semibold text-slate-700 dark:text-slate-300"
              >
                Confirm Password
              </label>
              <input
                id="admin-confirm-password"
                type={showPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                required
              />
            </div>

            <div className="pt-2">
              <button
                type="submit"
                disabled={setupAdminMutation.isPending}
                className="flex w-full items-center justify-center gap-2 rounded-xl bg-blue-600 py-2.5 font-semibold text-white shadow-sm hover:bg-blue-700 disabled:opacity-50"
              >
                <span>
                  {setupAdminMutation.isPending ? "Creating Admin..." : "Next: Git Provider"}
                </span>
                <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          </form>
        )}

        {/* Step 2: Git Provider Auth Profile */}
        {currentStep === 2 && (
          <form onSubmit={handleProviderSubmit} className="mt-6 space-y-4 text-xs">
            <div>
              <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                Step 2 of 5: Connect Git Provider
              </h2>
              <p className="mt-0.5 text-slate-500 dark:text-slate-400">
                Register authentication credentials to fetch runner registration tokens and
                orchestrate pools.
              </p>
            </div>

            {/* Provider Type Selection */}
            <div>
              <label className="font-semibold text-slate-700 dark:text-slate-300">
                Provider Method
              </label>
              <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
                {[
                  { id: "github_pat", label: "GitHub PAT" },
                  { id: "github_app", label: "GitHub App" },
                  { id: "gitea_pat", label: "Gitea PAT" },
                  { id: "forgejo_pat", label: "Forgejo PAT" },
                ].map((m) => (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => setAuthMethod(m.id as any)}
                    className={`rounded-xl border p-2.5 text-center font-medium transition-all ${
                      authMethod === m.id
                        ? "border-blue-500 bg-blue-50/50 text-blue-700 font-semibold dark:border-blue-500 dark:bg-blue-950/30 dark:text-blue-300"
                        : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                    }`}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label
                htmlFor="profile-name"
                className="font-semibold text-slate-700 dark:text-slate-300"
              >
                Profile Name
              </label>
              <input
                id="profile-name"
                type="text"
                value={profileName}
                onChange={(e) => setProfileName(e.target.value)}
                className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                required
              />
            </div>

            {authMethod === "github_app" ? (
              <>
                <div>
                  <label
                    htmlFor="app-id"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    GitHub App ID
                  </label>
                  <input
                    id="app-id"
                    type="number"
                    value={appId}
                    onChange={(e) => setAppId(e.target.value)}
                    placeholder="e.g. 123456"
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="private-key"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Private Key PEM
                  </label>
                  <textarea
                    id="private-key"
                    rows={4}
                    value={privateKeyPem}
                    onChange={(e) => setPrivateKeyPem(e.target.value)}
                    placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 font-mono text-[11px] text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>
              </>
            ) : (
              <div>
                <label
                  htmlFor="provider-token"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Personal Access Token (PAT)
                </label>
                <div className="relative mt-1">
                  <input
                    id="provider-token"
                    type={showToken ? "text" : "password"}
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    placeholder="ghp_..."
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 pr-10 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                  <button
                    type="button"
                    aria-label="Toggle token visibility"
                    onClick={() => setShowToken(!showToken)}
                    className="absolute inset-y-0 right-0 flex items-center pr-3 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                    tabIndex={-1}
                  >
                    {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                type="button"
                onClick={() => setCurrentStep(1)}
                className="flex items-center gap-1.5 rounded-xl border border-slate-200 px-4 py-2.5 font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                <ArrowLeft className="h-4 w-4" />
                <span>Back</span>
              </button>
              <button
                type="submit"
                disabled={createAuthProfileMutation.isPending}
                className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-blue-600 py-2.5 font-semibold text-white shadow-sm hover:bg-blue-700 disabled:opacity-50"
              >
                <span>
                  {createAuthProfileMutation.isPending ? "Connecting..." : "Next: Safeguards"}
                </span>
                <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          </form>
        )}

        {/* Step 3: Global Scaling Safeguards */}
        {currentStep === 3 && (
          <form onSubmit={handleSafeguardsSubmit} className="mt-6 space-y-4 text-xs">
            <div>
              <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                Step 3 of 5: Global Scaling Safeguards
              </h2>
              <p className="mt-0.5 text-slate-500 dark:text-slate-400">
                Configure supervisor-level guardrails to prevent host resource starvation.
              </p>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="max-runners"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Total Allowed Runners
                </label>
                <p className="text-[10px] text-slate-400">
                  Maximum concurrent runners across all pools
                </p>
                <input
                  id="max-runners"
                  type="number"
                  min={1}
                  max={100}
                  value={totalAllowedRunners}
                  onChange={(e) => setTotalAllowedRunners(Number(e.target.value))}
                  className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
              </div>

              <div>
                <label
                  htmlFor="idle-warm-pool"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Warm Idle Reserve Ceiling
                </label>
                <p className="text-[10px] text-slate-400">
                  Max idle standby runners across all pools
                </p>
                <input
                  id="idle-warm-pool"
                  type="number"
                  min={0}
                  max={50}
                  value={totalIdleWarmPool}
                  onChange={(e) => setTotalIdleWarmPool(Number(e.target.value))}
                  className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
              </div>

              <div>
                <label
                  htmlFor="shutdown-timeout"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Graceful Shutdown Timeout (seconds)
                </label>
                <p className="text-[10px] text-slate-400">
                  Runner drain deadline upon SIGTERM / SIGINT
                </p>
                <input
                  id="shutdown-timeout"
                  type="number"
                  min={10}
                  max={3600}
                  value={shutdownTimeoutSeconds}
                  onChange={(e) => setShutdownTimeoutSeconds(Number(e.target.value))}
                  className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
              </div>

              <div>
                <label
                  htmlFor="retention-days"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Job History Retention (days)
                </label>
                <p className="text-[10px] text-slate-400">
                  Historical execution log prune interval
                </p>
                <input
                  id="retention-days"
                  type="number"
                  min={1}
                  max={365}
                  value={jobRetentionDays}
                  onChange={(e) => setJobRetentionDays(Number(e.target.value))}
                  className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <button
                type="button"
                onClick={() => setCurrentStep(2)}
                className="flex items-center gap-1.5 rounded-xl border border-slate-200 px-4 py-2.5 font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                <ArrowLeft className="h-4 w-4" />
                <span>Back</span>
              </button>
              <button
                type="submit"
                disabled={setAppSettingMutation.isPending}
                className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-blue-600 py-2.5 font-semibold text-white shadow-sm hover:bg-blue-700 disabled:opacity-50"
              >
                <span>
                  {setAppSettingMutation.isPending ? "Saving Safeguards..." : "Next: Initial Pool"}
                </span>
                <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          </form>
        )}

        {/* Step 4 & 5 Milestone Status (Completed in RUN-56) */}
        {currentStep >= 4 && (
          <div className="mt-6 space-y-4 text-center text-xs">
            <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400">
              <CheckCircle2 className="h-6 w-6" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                Steps 1–3 Complete!
              </h2>
              <p className="mt-1 text-slate-500 dark:text-slate-400">
                Master administrator, Git authentication profile, and global safeguards are
                configured.
              </p>
            </div>

            <div className="rounded-xl border border-slate-100 bg-slate-50 p-4 text-left dark:border-slate-800 dark:bg-slate-800/40">
              <p className="font-semibold text-slate-700 dark:text-slate-300">Next Up (RUN-56):</p>
              <ul className="mt-2 list-disc space-y-1 pl-4 text-slate-500 dark:text-slate-400">
                <li>
                  Step 4: Initial Runner Pool Setup (Repository/Org URL, Scope, Concurrency Quotas)
                </li>
                <li>Step 5: Review Configuration & Launch Supervisor Reconciler</li>
              </ul>
            </div>

            <div className="flex justify-start">
              <button
                type="button"
                onClick={() => setCurrentStep(3)}
                className="flex items-center gap-1.5 rounded-xl border border-slate-200 px-4 py-2.5 font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                <ArrowLeft className="h-4 w-4" />
                <span>Back to Safeguards</span>
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
