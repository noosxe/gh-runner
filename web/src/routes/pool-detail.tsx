import { useState } from "react";
import { useParams, Link } from "@tanstack/react-router";
import { usePools, useRunners, useTerminateRunner } from "../lib/api/query-hooks";
import { useWatchRunners, useStreamRunnerLogs } from "../lib/api/streaming-hooks";
import { LogTerminal } from "../components/terminal/log-terminal";
import type { RunnerInstance } from "../gen/api_pb";
import {
  ArrowLeft,
  Server,
  Activity,
  Cpu,
  HardDrive,
  Shield,
  Clock,
  Terminal,
  Trash2,
  X,
  AlertTriangle,
} from "lucide-react";

function formatUptime(seconds: number | bigint): string {
  const sec = Number(seconds);
  if (sec < 60) return `${sec}s`;
  const mins = Math.floor(sec / 60);
  const remSec = sec % 60;
  if (mins < 60) return `${mins}m ${remSec}s`;
  const hours = Math.floor(mins / 60);
  const remMins = mins % 60;
  return `${hours}h ${remMins}m`;
}

export function PoolDetailPage() {
  const { poolId } = useParams({ strict: false }) as { poolId?: string };
  const poolIdBigInt = poolId ? BigInt(poolId) : 0n;

  const { data: pools } = usePools();
  const pool = pools?.find((p) => p.id === poolIdBigInt);

  const { data: runners, isLoading: runnersLoading } = useRunners(poolIdBigInt);
  const { isConnected: isStreamActive } = useWatchRunners(poolIdBigInt);

  const [activeTab, setActiveTab] = useState<"runners" | "config">("runners");
  const [selectedRunnerForLogs, setSelectedRunnerForLogs] = useState<RunnerInstance | null>(null);
  const [runnerToTerminate, setRunnerToTerminate] = useState<RunnerInstance | null>(null);

  const terminateMutation = useTerminateRunner();

  const handleConfirmTerminate = async () => {
    if (!runnerToTerminate || !pool) return;
    try {
      await terminateMutation.mutateAsync({
        poolId: pool.id,
        containerId: runnerToTerminate.containerId,
      });
      setRunnerToTerminate(null);
    } catch (err) {
      console.error("Failed to terminate runner:", err);
    }
  };

  if (!pool) {
    return (
      <div className="space-y-4">
        <Link
          to="/pools"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-blue-600 hover:underline"
        >
          <ArrowLeft className="h-3.5 w-3.5" /> Back to Pools
        </Link>
        <div className="rounded-2xl border border-slate-200 bg-white p-12 text-center text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900">
          Pool not found or loading...
        </div>
      </div>
    );
  }

  const activeInstances = runners?.filter((r) => r.status === "busy").length ?? 0;
  const idleInstances = runners?.filter((r) => r.status === "idle").length ?? 0;

  return (
    <div className="space-y-6">
      {/* Navigation & Header */}
      <div>
        <Link
          to="/pools"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 transition-colors mb-3"
        >
          <ArrowLeft className="h-3.5 w-3.5" /> Back to Runner Pools
        </Link>

        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
                {pool.name}
              </h1>
              <span
                className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium border ${
                  isStreamActive
                    ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800/60"
                    : "bg-amber-50 text-amber-700 dark:bg-amber-950/60 dark:text-amber-400 border-amber-200 dark:border-amber-800/60"
                }`}
              >
                <span
                  className={`h-1.5 w-1.5 rounded-full ${
                    isStreamActive ? "bg-emerald-500 animate-pulse" : "bg-amber-500"
                  }`}
                />
                <span className="font-mono text-[10px]">
                  {isStreamActive ? "Live Orchestrator Stream" : "Connecting"}
                </span>
              </span>
            </div>
            <p className="mt-1 text-xs font-mono text-slate-500 dark:text-slate-400">
              {pool.repositoryUrl}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <span className="rounded-md bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700 uppercase tracking-wider dark:bg-blue-950/50 dark:text-blue-400 border border-blue-200 dark:border-blue-900">
              {pool.provider}
            </span>
            <span className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-600 uppercase tracking-wider dark:bg-slate-800 dark:text-slate-300">
              {pool.scope || "repo"}
            </span>
          </div>
        </div>
      </div>

      {/* KPI Stats Strip */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Active Running Jobs
          </span>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-900 dark:text-white">
              {activeInstances}
            </span>
            <span className="text-xs text-slate-400">of {pool.maxConcurrency} max</span>
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Idle Warm Pool
          </span>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-900 dark:text-white">
              {idleInstances}
            </span>
            <span className="text-xs text-slate-400">target: {pool.minIdleRunners}</span>
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Resource Quotas
          </span>
          <div className="mt-1 flex items-center gap-3 text-xs font-semibold text-slate-800 dark:text-slate-200">
            <span className="inline-flex items-center gap-1">
              <Cpu className="h-3.5 w-3.5 text-slate-400" />
              {pool.cpuLimit || "Unlimited"}
            </span>
            <span className="inline-flex items-center gap-1">
              <HardDrive className="h-3.5 w-3.5 text-slate-400" />
              {pool.memoryLimit || "Unlimited"}
            </span>
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Docker Privileges
          </span>
          <div className="mt-1 flex items-center gap-1.5 text-xs font-semibold text-slate-800 dark:text-slate-200">
            <Shield className="h-3.5 w-3.5 text-slate-400" />
            <span>{pool.allowDocker ? "Docker Daemon Enabled" : "Rootless Isolation"}</span>
          </div>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex border-b border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onClick={() => setActiveTab("runners")}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-colors ${
            activeTab === "runners"
              ? "border-blue-600 text-blue-600 dark:border-blue-500 dark:text-blue-400"
              : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
          }`}
        >
          <Activity className="h-3.5 w-3.5" />
          <span>Active Containers & Runners</span>
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-bold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
            {runners?.length ?? 0}
          </span>
        </button>

        <button
          type="button"
          onClick={() => setActiveTab("config")}
          className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-colors ${
            activeTab === "config"
              ? "border-blue-600 text-blue-600 dark:border-blue-500 dark:text-blue-400"
              : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
          }`}
        >
          <Server className="h-3.5 w-3.5" />
          <span>Pool Configuration</span>
        </button>
      </div>

      {/* Tab Content: Runners & Containers Table */}
      {activeTab === "runners" && (
        <div className="space-y-4">
          {runnersLoading ? (
            <div className="rounded-2xl border border-slate-200 bg-white p-8 text-center text-xs text-slate-400 dark:border-slate-800 dark:bg-slate-900">
              Auditing active container instances...
            </div>
          ) : !runners || runners.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
              <Server className="mx-auto h-8 w-8 text-slate-400 mb-2" />
              <p className="text-sm font-semibold text-slate-800 dark:text-slate-200">
                No active container instances
              </p>
              <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
                No runners are currently executing or warming in this pool. Ephemeral instances will
                automatically spawn when workflow jobs are queued.
              </p>
            </div>
          ) : (
            <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-slate-600 dark:text-slate-300">
                  <thead className="border-b border-slate-100 bg-slate-50 font-semibold uppercase tracking-wider text-[11px] text-slate-500 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400">
                    <tr>
                      <th className="px-5 py-3">Container ID</th>
                      <th className="px-5 py-3">Runner Name</th>
                      <th className="px-5 py-3">State</th>
                      <th className="px-5 py-3">IP Address</th>
                      <th className="px-5 py-3">Uptime</th>
                      <th className="px-5 py-3">CPU / Mem Limit</th>
                      <th className="px-5 py-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                    {runners.map((r) => {
                      const isBusy = r.status === "busy";
                      const isIdle = r.status === "idle";
                      const isDegraded = r.status === "degraded";

                      return (
                        <tr
                          key={r.containerId}
                          className="hover:bg-slate-50/50 dark:hover:bg-slate-800/50 transition-colors"
                        >
                          <td className="px-5 py-3.5 font-mono text-[11px] font-semibold text-slate-900 dark:text-white">
                            {r.containerId.substring(0, 12)}
                          </td>
                          <td className="px-5 py-3.5 font-mono font-medium text-slate-800 dark:text-slate-200">
                            {r.name}
                          </td>
                          <td className="px-5 py-3.5">
                            <span
                              className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider ${
                                isBusy
                                  ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-900"
                                  : isIdle
                                    ? "bg-sky-50 text-sky-700 dark:bg-sky-950/60 dark:text-sky-400 border border-sky-200 dark:border-sky-900"
                                    : isDegraded
                                      ? "bg-rose-50 text-rose-700 dark:bg-rose-950/60 dark:text-rose-400 border border-rose-200 dark:border-rose-900"
                                      : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
                              }`}
                            >
                              <span
                                className={`h-1.5 w-1.5 rounded-full ${
                                  isBusy
                                    ? "bg-emerald-500 animate-pulse"
                                    : isIdle
                                      ? "bg-sky-500"
                                      : isDegraded
                                        ? "bg-rose-500"
                                        : "bg-slate-400"
                                }`}
                              />
                              <span>{r.status}</span>
                            </span>
                          </td>
                          <td className="px-5 py-3.5 font-mono text-slate-500 dark:text-slate-400">
                            {r.ipAddress || "—"}
                          </td>
                          <td className="px-5 py-3.5 font-mono text-slate-600 dark:text-slate-300">
                            <span className="inline-flex items-center gap-1">
                              <Clock className="h-3 w-3 text-slate-400" />
                              {formatUptime(r.uptimeSeconds)}
                            </span>
                          </td>
                          <td className="px-5 py-3.5 text-slate-500">
                            {r.cpuLimit || pool.cpuLimit || "Unlimited"} /{" "}
                            {r.memoryLimit || pool.memoryLimit || "Unlimited"}
                          </td>
                          <td className="px-5 py-3.5 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <button
                                type="button"
                                onClick={() => setSelectedRunnerForLogs(r)}
                                className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs font-semibold text-slate-700 shadow-2xs hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700 transition-colors"
                              >
                                <Terminal className="h-3.5 w-3.5 text-slate-400" />
                                <span>Logs</span>
                              </button>
                              <button
                                type="button"
                                onClick={() => setRunnerToTerminate(r)}
                                className="inline-flex items-center gap-1 rounded-lg border border-rose-200 bg-rose-50 px-2.5 py-1 text-xs font-semibold text-rose-700 shadow-2xs hover:bg-rose-100 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-400 dark:hover:bg-rose-950/80 transition-colors"
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                                <span>Terminate</span>
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Tab Content: Configuration */}
      {activeTab === "config" && (
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900 space-y-6">
          <h2 className="text-base font-bold text-slate-900 dark:text-white">
            Pool Parameters & Resource Limits
          </h2>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 text-xs">
            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Target Repository URL</span>
              <div className="mt-1 font-mono font-semibold text-slate-900 dark:text-white break-all">
                {pool.repositoryUrl}
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Git Provider</span>
              <div className="mt-1 font-semibold text-slate-900 dark:text-white uppercase">
                {pool.provider}
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Registration Scope</span>
              <div className="mt-1 font-semibold text-slate-900 dark:text-white uppercase">
                {pool.scope || "repo"}
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Runner Container Image</span>
              <div className="mt-1 font-mono font-semibold text-slate-900 dark:text-white">
                {pool.runnerImage || "ghcr.io/noosxe/runner-aio:latest"}
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Max Job Lifetime Limit</span>
              <div className="mt-1 font-semibold text-slate-900 dark:text-white">
                {pool.maxRunnerLifetimeSeconds
                  ? `${pool.maxRunnerLifetimeSeconds} seconds`
                  : "7200s (2 hours)"}
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400">Runner Labels</span>
              <div className="mt-1 flex flex-wrap gap-1">
                {pool.labels && pool.labels.length > 0 ? (
                  pool.labels.map((l) => (
                    <span
                      key={l}
                      className="rounded-md bg-slate-200 px-2 py-0.5 text-[11px] font-mono text-slate-700 dark:bg-slate-800 dark:text-slate-300"
                    >
                      {l}
                    </span>
                  ))
                ) : (
                  <span className="font-mono text-slate-400">self-hosted, linux, arm64</span>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Confirmation Modal: Terminate Runner */}
      {runnerToTerminate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 backdrop-blur-xs p-4">
          <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-800 dark:bg-slate-900">
            <div className="flex items-center gap-3 text-rose-600 dark:text-rose-400">
              <AlertTriangle className="h-6 w-6" />
              <h3 className="text-base font-bold text-slate-900 dark:text-white">
                Terminate Runner Instance?
              </h3>
            </div>

            <p className="mt-3 text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
              Are you sure you want to manually terminate runner{" "}
              <strong className="font-mono text-slate-900 dark:text-white">
                {runnerToTerminate.name}
              </strong>{" "}
              (container ID:{" "}
              <span className="font-mono">{runnerToTerminate.containerId.substring(0, 12)}</span>
              )? If this runner is currently executing a workflow, the job will fail immediately.
            </p>

            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setRunnerToTerminate(null)}
                className="rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmTerminate}
                disabled={terminateMutation.isPending}
                className="rounded-xl bg-rose-600 px-4 py-2 text-xs font-semibold text-white shadow-xs hover:bg-rose-500 disabled:opacity-50"
              >
                {terminateMutation.isPending ? "Terminating..." : "Terminate Instance"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Live Runner Log Modal */}
      {selectedRunnerForLogs && (
        <RunnerLogViewerModal
          runner={selectedRunnerForLogs}
          onClose={() => setSelectedRunnerForLogs(null)}
        />
      )}
    </div>
  );
}

function RunnerLogViewerModal({
  runner,
  onClose,
}: {
  runner: RunnerInstance;
  onClose: () => void;
}) {
  const { logs, isConnected, isConnecting, clearLogs } = useStreamRunnerLogs(runner.name);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 backdrop-blur-xs p-4">
      <div className="relative flex h-[82vh] w-full max-w-5xl flex-col rounded-2xl border border-slate-800 bg-slate-950 shadow-2xl overflow-hidden">
        {/* Close button overlay */}
        <button
          type="button"
          onClick={onClose}
          aria-label="Close runner logs modal"
          className="absolute right-3.5 top-3 z-20 rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
        >
          <X className="h-4 w-4" />
        </button>

        <div className="flex-1 overflow-hidden">
          <LogTerminal
            logs={logs}
            mode="live"
            runnerName={runner.name}
            containerId={runner.containerId}
            isConnected={isConnected}
            isConnecting={isConnecting}
            onClear={clearLogs}
            title={runner.name}
          />
        </div>
      </div>
    </div>
  );
}
