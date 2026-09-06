import { useState, useMemo, type FormEvent } from "react";
import { usePools, useAuthProfiles, useCreatePool } from "../lib/api/query-hooks";
import { useWatchPools } from "../lib/api/streaming-hooks";
import { Link } from "@tanstack/react-router";
import {
  Server,
  Search,
  Cpu,
  HardDrive,
  Shield,
  Activity,
  ArrowUpRight,
  Info,
  Plus,
  X,
  AlertCircle,
} from "lucide-react";

export function PoolsPage() {
  const { data: pools, isLoading } = usePools();
  const { data: authProfiles, isLoading: authProfilesLoading } = useAuthProfiles();
  const { isConnected } = useWatchPools();
  const createPoolMutation = useCreatePool();
  const hasAuthProfiles = Boolean(authProfiles && authProfiles.length > 0);

  const [search, setSearch] = useState("");
  const [providerFilter, setProviderFilter] = useState("all");
  const [scopeFilter, setScopeFilter] = useState("all");

  // Create Pool Modal State
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [poolName, setPoolName] = useState("");
  const [authProfileId, setAuthProfileId] = useState<string>("");
  const [repositoryUrl, setRepositoryUrl] = useState("");
  const [scope, setScope] = useState<"repo" | "org">("repo");
  const [minIdleRunners, setMinIdleRunners] = useState(1);
  const [maxConcurrency, setMaxConcurrency] = useState(5);
  const [labels, setLabels] = useState("self-hosted,linux,arm64");
  const [runnerImage, setRunnerImage] = useState("ghcr.io/noosxe/gh-runner:latest");
  const [allowDocker, setAllowDocker] = useState(true);
  const [cpuLimit, setCpuLimit] = useState("2.0");
  const [memoryLimit, setMemoryLimit] = useState("4GB");

  // Renovate State
  const [renovateEnabled, setRenovateEnabled] = useState(false);
  const [renovateCron, setRenovateCron] = useState("0 2 * * *");
  const [renovateImage, setRenovateImage] = useState("renovate/renovate:latest");

  const selectedAuthProfile = useMemo(() => {
    if (!authProfiles || authProfiles.length === 0) return null;
    if (authProfileId) {
      return authProfiles.find((p) => p.id.toString() === authProfileId) ?? authProfiles[0];
    }
    return authProfiles[0];
  }, [authProfiles, authProfileId]);

  const deducedProvider = useMemo(() => {
    const m = selectedAuthProfile?.authMethod;
    if (!m) return "github";
    if (m.startsWith("gitea")) return "gitea";
    if (m.startsWith("forgejo")) return "forgejo";
    return "github";
  }, [selectedAuthProfile]);

  const filteredPools = useMemo(() => {
    if (!pools) return [];
    return pools.filter((p) => {
      const matchesSearch =
        search === "" ||
        p.name.toLowerCase().includes(search.toLowerCase()) ||
        p.repositoryUrl.toLowerCase().includes(search.toLowerCase());

      const matchesProvider =
        providerFilter === "all" || p.provider.toLowerCase() === providerFilter.toLowerCase();

      const matchesScope =
        scopeFilter === "all" || (p.scope || "repo").toLowerCase() === scopeFilter.toLowerCase();

      return matchesSearch && matchesProvider && matchesScope;
    });
  }, [pools, search, providerFilter, scopeFilter]);

  const handleOpenModal = () => {
    setPoolName("");
    setRepositoryUrl("");
    setScope("repo");
    setMinIdleRunners(1);
    setMaxConcurrency(5);
    setLabels("self-hosted,linux,arm64");
    setRunnerImage("ghcr.io/noosxe/gh-runner:latest");
    setAllowDocker(true);
    setCpuLimit("2.0");
    setMemoryLimit("4GB");
    setRenovateEnabled(false);
    setRenovateCron("0 2 * * *");
    setRenovateImage("renovate/renovate:latest");
    if (authProfiles && authProfiles.length > 0) {
      setAuthProfileId(authProfiles[0].id.toString());
    }
    setError(null);
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setError(null);
  };

  const handleCreatePool = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!poolName.trim()) {
      setError("Pool name is required");
      return;
    }
    if (!repositoryUrl.trim()) {
      setError("Repository or Organization URL is required");
      return;
    }
    if (minIdleRunners > maxConcurrency) {
      setError("Min idle runners cannot exceed max concurrency");
      return;
    }
    if (!selectedAuthProfile) {
      setError("An authentication profile is required to create a runner pool");
      return;
    }

    const parsedLabels = labels
      .split(",")
      .map((l) => l.trim())
      .filter(Boolean);

    try {
      await createPoolMutation.mutateAsync({
        pool: {
          $typeName: "supervisor.v1.Pool",
          id: 0n,
          name: poolName.trim(),
          provider: deducedProvider,
          repositoryUrl: repositoryUrl.trim(),
          minIdleRunners,
          maxConcurrency,
          labels: parsedLabels.length > 0 ? parsedLabels : ["self-hosted", "linux", "arm64"],
          runnerImage: runnerImage.trim() || "ghcr.io/noosxe/gh-runner:latest",
          allowDocker,
          renovate: renovateEnabled
            ? {
                $typeName: "supervisor.v1.RenovateConfig",
                enabled: true,
                cronSchedule: renovateCron.trim() || "0 2 * * *",
                image: renovateImage.trim() || "renovate/renovate:latest",
              }
            : undefined,
          activeRunners: 0,
          idleRunners: 0,
          authProfileId: selectedAuthProfile.id,
          scope,
          cpuLimit: cpuLimit.trim() || "2.0",
          memoryLimit: memoryLimit.trim() || "4GB",
          maxRunnerLifetimeSeconds: 7200,
          imageUpdateAvailable: false,
          latestImage: "",
        },
      });
      handleCloseModal();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create runner pool");
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
              Runner Pools
            </h1>
            <span
              className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium border ${
                isConnected
                  ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800/60"
                  : "bg-amber-50 text-amber-700 dark:bg-amber-950/60 dark:text-amber-400 border-amber-200 dark:border-amber-800/60"
              }`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  isConnected ? "bg-emerald-500 animate-pulse" : "bg-amber-500"
                }`}
              />
              <span className="font-mono text-[10px]">
                {isConnected ? "Live Stream" : "Connecting"}
              </span>
            </span>
          </div>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Manage ephemeral worker pools, runtime scaling targets, and provider bindings.
          </p>
        </div>

        {hasAuthProfiles ? (
          <button
            type="button"
            onClick={handleOpenModal}
            className="inline-flex items-center justify-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-xs hover:bg-blue-500 transition-colors"
          >
            <Plus className="h-4 w-4" />
            <span>+ Add Runner Pool</span>
          </button>
        ) : (
          <Link
            to="/profiles"
            className="inline-flex items-center justify-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-xs hover:bg-blue-500 transition-colors"
          >
            <Plus className="h-4 w-4" />
            <span>+ Add Runner Pool</span>
          </Link>
        )}
      </div>

      {/* Missing Auth Profile Warning Banner */}
      {!hasAuthProfiles && !authProfilesLoading && (
        <div className="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-4 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
          <div className="flex-1 space-y-1">
            <p className="font-semibold text-slate-900 dark:text-white">
              Git Authentication Profile Required
            </p>
            <p className="text-[11px] text-amber-700 dark:text-amber-400">
              Runner pools require upstream credentials to register ephemeral runners with GitHub,
              Gitea, or Forgejo. Connect an auth profile first or run through the setup wizard.
            </p>
          </div>
          <Link
            to="/profiles"
            className="shrink-0 rounded-xl bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs transition-colors hover:bg-amber-700 dark:bg-amber-500 dark:hover:bg-amber-600"
          >
            Configure Profile &rarr;
          </Link>
        </div>
      )}

      {/* Filters Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between rounded-2xl border border-slate-200 bg-white p-3 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search pools by name or repository URL..."
            className="w-full rounded-xl border-0 bg-slate-50 py-2 pl-9 pr-4 text-xs text-slate-900 placeholder-slate-400 focus:bg-white focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:text-white dark:focus:bg-slate-900"
          />
        </div>

        <div className="flex items-center gap-2">
          <select
            value={providerFilter}
            onChange={(e) => setProviderFilter(e.target.value)}
            className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-700 shadow-xs focus:ring-2 focus:ring-blue-500 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200"
          >
            <option value="all">All Providers</option>
            <option value="github">GitHub</option>
            <option value="gitea">Gitea</option>
            <option value="forgejo">Forgejo</option>
          </select>

          <select
            value={scopeFilter}
            onChange={(e) => setScopeFilter(e.target.value)}
            className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-700 shadow-xs focus:ring-2 focus:ring-blue-500 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200"
          >
            <option value="all">All Scopes</option>
            <option value="repo">Repository</option>
            <option value="org">Organization</option>
            <option value="global">Global</option>
          </select>
        </div>
      </div>

      {/* Pools Grid */}
      {isLoading ? (
        <div className="rounded-2xl border border-slate-200 bg-white p-12 text-center text-sm text-slate-400 dark:border-slate-800 dark:bg-slate-900">
          Loading runner pools...
        </div>
      ) : filteredPools.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <Server className="mx-auto mb-2 h-8 w-8 text-slate-400" />
          <p className="text-base font-semibold text-slate-800 dark:text-slate-200">
            {pools?.length === 0 ? "No runner pools configured" : "No pools match your filters"}
          </p>
          <p className="mx-auto mt-1 max-w-md text-xs text-slate-500 dark:text-slate-400">
            {pools?.length === 0
              ? hasAuthProfiles
                ? "Git authentication profile is ready. Create your first runner pool to start processing CI workflows."
                : "No Git authentication profiles are configured yet. Connect a Git profile before creating your first pool."
              : "Try adjusting your search terms or filter criteria."}
          </p>
          {pools?.length === 0 && (
            <div className="mt-4 flex items-center justify-center gap-3">
              {hasAuthProfiles ? (
                <button
                  type="button"
                  onClick={handleOpenModal}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition-colors hover:bg-blue-500"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>+ Add Runner Pool</span>
                </button>
              ) : (
                <Link
                  to="/profiles"
                  className="inline-flex items-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition-colors hover:bg-blue-500"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Configure Git Profile</span>
                </Link>
              )}
            </div>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          {filteredPools.map((p) => {
            const maxConcurrency = p.maxConcurrency > 0 ? p.maxConcurrency : 1;
            const utilization = Math.min(100, Math.round((p.activeRunners / maxConcurrency) * 100));

            return (
              <div
                key={p.id.toString()}
                className="group relative flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-6 shadow-xs transition-all hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
              >
                <div>
                  {/* Pool Header */}
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="text-lg font-bold text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
                        {p.name}
                      </h3>
                      <p className="mt-0.5 text-xs font-mono text-slate-500 truncate max-w-sm">
                        {p.repositoryUrl}
                      </p>
                    </div>

                    <div className="flex items-center gap-1.5">
                      <span className="rounded-md bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-700 uppercase tracking-wider dark:bg-blue-950/50 dark:text-blue-400 border border-blue-200 dark:border-blue-900">
                        {p.provider}
                      </span>
                      <span className="rounded-md bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-600 uppercase tracking-wider dark:bg-slate-800 dark:text-slate-300">
                        {p.scope || "repo"}
                      </span>
                    </div>
                  </div>

                  {/* Utilization Progress Bar */}
                  <div className="mt-5">
                    <div className="flex items-center justify-between text-xs text-slate-500">
                      <span className="flex items-center gap-1">
                        <Activity className="h-3.5 w-3.5 text-emerald-500" />
                        <span>Capacity Utilization</span>
                      </span>
                      <span className="font-semibold text-slate-700 dark:text-slate-300">
                        {p.activeRunners} / {p.maxConcurrency} ({utilization}%)
                      </span>
                    </div>
                    <div className="mt-1.5 h-2 w-full overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                      <div
                        className={`h-full transition-all duration-500 rounded-full ${
                          utilization > 85
                            ? "bg-rose-500"
                            : utilization > 60
                              ? "bg-amber-500"
                              : "bg-emerald-500"
                        }`}
                        style={{ width: `${utilization}%` }}
                      />
                    </div>
                  </div>

                  {/* Metrics Grid */}
                  <div className="mt-5 grid grid-cols-3 gap-2 rounded-xl bg-slate-50 p-3 text-center text-xs dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800/80">
                    <div>
                      <span className="text-slate-400">Active</span>
                      <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                        {p.activeRunners}
                      </div>
                    </div>
                    <div>
                      <span className="text-slate-400">Idle Warm Target</span>
                      <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                        {p.minIdleRunners}
                      </div>
                    </div>
                    <div>
                      <span className="text-slate-400">Max Limit</span>
                      <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                        {p.maxConcurrency}
                      </div>
                    </div>
                  </div>

                  {/* Badges / Specs Strip */}
                  <div className="mt-4 flex flex-wrap items-center gap-2 text-xs text-slate-500">
                    <span className="inline-flex items-center gap-1 rounded-md bg-slate-100 px-2 py-0.5 font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      <Cpu className="h-3 w-3 text-slate-400" />
                      {p.cpuLimit || "2"} CPU
                    </span>
                    <span className="inline-flex items-center gap-1 rounded-md bg-slate-100 px-2 py-0.5 font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      <HardDrive className="h-3 w-3 text-slate-400" />
                      {p.memoryLimit || "4G"} Mem
                    </span>
                    {p.allowDocker && (
                      <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-900">
                        <Shield className="h-3 w-3" />
                        Docker Enabled
                      </span>
                    )}
                  </div>
                </div>

                {/* Footer Link */}
                <div className="mt-6 flex items-center justify-between border-t border-slate-100 pt-4 dark:border-slate-800">
                  <span className="text-xs text-slate-400">
                    Image:{" "}
                    <span className="font-mono text-slate-600 dark:text-slate-300">
                      {p.runnerImage ? p.runnerImage.split("/").pop() : "gh-runner:latest"}
                    </span>
                  </span>

                  <Link
                    to="/pools/$poolId"
                    params={{ poolId: p.id.toString() }}
                    className="inline-flex items-center gap-1 text-xs font-semibold text-blue-600 hover:text-blue-500 dark:text-blue-400 dark:hover:text-blue-300 transition-colors"
                  >
                    <span>View Pool Details</span>
                    <ArrowUpRight className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create Pool Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 backdrop-blur-xs p-4 overflow-y-auto">
          <div className="w-full max-w-xl rounded-2xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-800 dark:bg-slate-900 text-xs my-8">
            <div className="flex items-center justify-between border-b border-slate-100 pb-3 dark:border-slate-800">
              <div className="flex items-center gap-2">
                <Server className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  Add Runner Pool
                </h3>
              </div>
              <button
                type="button"
                onClick={handleCloseModal}
                className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {error && (
              <div className="mt-3 flex items-center gap-2 rounded-xl border border-rose-200 bg-rose-50/80 p-3 text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <form onSubmit={handleCreatePool} className="mt-4 space-y-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label
                    htmlFor="modal-pool-name"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Pool Name
                  </label>
                  <input
                    id="modal-pool-name"
                    type="text"
                    placeholder="e.g. arm64-ci-pool"
                    value={poolName}
                    onChange={(e) => setPoolName(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-auth-profile"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Git Auth Profile
                  </label>
                  <select
                    id="modal-auth-profile"
                    value={authProfileId}
                    onChange={(e) => setAuthProfileId(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  >
                    {authProfiles?.map((prof) => (
                      <option key={prof.id.toString()} value={prof.id.toString()}>
                        {prof.name} ({prof.authMethod})
                      </option>
                    ))}
                  </select>
                </div>

                <div className="sm:col-span-2">
                  <label
                    htmlFor="modal-repo-url"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Repository / Organization URL
                  </label>
                  <input
                    id="modal-repo-url"
                    type="url"
                    placeholder="https://github.com/my-org/my-repo"
                    value={repositoryUrl}
                    onChange={(e) => setRepositoryUrl(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-scope"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Pool Scope
                  </label>
                  <select
                    id="modal-scope"
                    value={scope}
                    onChange={(e) => setScope(e.target.value as "repo" | "org")}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  >
                    <option value="repo">Repository Level (Single Repo)</option>
                    <option value="org">Organization Level (Org-wide)</option>
                  </select>
                </div>

                <div>
                  <label
                    htmlFor="modal-min-idle"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Min Idle Runners
                  </label>
                  <input
                    id="modal-min-idle"
                    type="number"
                    min={0}
                    max={20}
                    value={minIdleRunners}
                    onChange={(e) => setMinIdleRunners(Number(e.target.value))}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-max-concurrency"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Max Concurrency
                  </label>
                  <input
                    id="modal-max-concurrency"
                    type="number"
                    min={1}
                    max={50}
                    value={maxConcurrency}
                    onChange={(e) => setMaxConcurrency(Number(e.target.value))}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-labels"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Runner Labels
                  </label>
                  <input
                    id="modal-labels"
                    type="text"
                    value={labels}
                    onChange={(e) => setLabels(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-cpu"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    CPU Limit
                  </label>
                  <input
                    id="modal-cpu"
                    type="text"
                    value={cpuLimit}
                    onChange={(e) => setCpuLimit(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  />
                </div>

                <div>
                  <label
                    htmlFor="modal-mem"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Memory Limit
                  </label>
                  <input
                    id="modal-mem"
                    type="text"
                    value={memoryLimit}
                    onChange={(e) => setMemoryLimit(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  />
                </div>
              </div>

              <div className="pt-2 border-t border-slate-100 dark:border-slate-800">
                <label className="flex items-center gap-2 cursor-pointer font-semibold text-slate-700 dark:text-slate-300">
                  <input
                    type="checkbox"
                    checked={allowDocker}
                    onChange={(e) => setAllowDocker(e.target.checked)}
                    className="h-4 w-4 rounded-md border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span>Enable Docker-in-Docker socket access</span>
                </label>
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
                <button
                  type="button"
                  onClick={handleCloseModal}
                  className="rounded-xl border border-slate-200 px-4 py-2 font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createPoolMutation.isPending}
                  className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white shadow-xs hover:bg-blue-500 disabled:opacity-50"
                >
                  {createPoolMutation.isPending ? "Creating Pool..." : "Create Pool"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
