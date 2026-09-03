import { useSystemStats, usePools, useJobHistory } from "../lib/api/query-hooks";
import { Activity, CheckCircle2, Clock, Server, XCircle } from "lucide-react";
import { Link } from "@tanstack/react-router";

export function DashboardPage() {
  const { data: stats, isLoading: statsLoading } = useSystemStats();
  const { data: pools, isLoading: poolsLoading } = usePools();
  const { data: history } = useJobHistory(undefined, 5, 0);

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Dashboard Overview
          </h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Real-time supervisor metrics, runner utilization, and health state.
          </p>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
            <span>ACTIVE RUNNERS</span>
            <Activity className="h-4 w-4 text-emerald-500" />
          </div>
          <div className="mt-2 text-3xl font-bold text-slate-900 dark:text-white">
            {statsLoading ? "..." : `${stats?.totalActiveRunners ?? 0} active`}
          </div>
          <div className="mt-1 text-xs text-slate-400">
            {stats?.totalIdleRunners ?? 0} warm idle standby
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
            <span>24H JOBS EXECUTED</span>
            <Server className="h-4 w-4 text-blue-500" />
          </div>
          <div className="mt-2 text-3xl font-bold text-slate-900 dark:text-white">
            {statsLoading ? "..." : (stats?.totalJobs24h ?? 0)}
          </div>
          <div className="mt-1 text-xs text-slate-400">
            Avg queue wait: {(stats?.averageQueueTimeSeconds ?? 0).toFixed(1)}s
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
            <span>SUCCESS RATE</span>
            <CheckCircle2 className="h-4 w-4 text-emerald-500" />
          </div>
          <div className="mt-2 text-3xl font-bold text-slate-900 dark:text-white">
            {statsLoading ? "..." : `${(stats?.successRatePercent ?? 100).toFixed(1)}%`}
          </div>
          <div className="mt-1 text-xs text-slate-400">
            {stats?.successfulJobs24h ?? 0} passed / {stats?.failedJobs24h ?? 0} failed
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400">
            <span>AVG RUNTIME</span>
            <Clock className="h-4 w-4 text-indigo-500" />
          </div>
          <div className="mt-2 text-3xl font-bold text-slate-900 dark:text-white">
            {statsLoading ? "..." : `${(stats?.averageRuntimeSeconds ?? 0).toFixed(0)}s`}
          </div>
          <div className="mt-1 text-xs text-slate-400">Job completion latency</div>
        </div>
      </div>

      {/* Pools Summary */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
            Configured Runner Pools
          </h2>
          <Link
            to="/pools"
            className="text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
          >
            View All Pools &rarr;
          </Link>
        </div>

        {poolsLoading ? (
          <div className="text-sm text-slate-400">Loading pools...</div>
        ) : !pools || pools.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-300 p-8 text-center text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
            No runner pools configured yet.
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {pools.map((p) => (
              <Link
                key={p.id.toString()}
                to="/pools/$poolId"
                params={{ poolId: p.id.toString() }}
                className="group rounded-2xl border border-slate-200 bg-white p-5 shadow-xs transition-all hover:border-blue-400 hover:shadow-md dark:border-slate-800 dark:bg-slate-900 dark:hover:border-blue-600"
              >
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-slate-900 group-hover:text-blue-600 dark:text-white dark:group-hover:text-blue-400">
                    {p.name}
                  </span>
                  <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium uppercase tracking-wider text-slate-600 dark:bg-slate-800 dark:text-slate-400">
                    {p.provider}
                  </span>
                </div>
                <div className="mt-2 truncate text-xs text-slate-500 dark:text-slate-400">
                  {p.repositoryUrl}
                </div>
                <div className="mt-4 flex items-center justify-between border-t border-slate-100 pt-3 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-400">
                  <span>
                    Active:{" "}
                    <strong className="text-slate-900 dark:text-white">{p.activeRunners}</strong>
                  </span>
                  <span>
                    Idle Target:{" "}
                    <strong className="text-slate-900 dark:text-white">{p.minIdleRunners}</strong>
                  </span>
                  <span>
                    Max:{" "}
                    <strong className="text-slate-900 dark:text-white">{p.maxConcurrency}</strong>
                  </span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Recent History */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-slate-900 dark:text-white">Recent Executions</h2>
        {!history?.jobs || history.jobs.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-300 p-8 text-center text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
            No executions recorded in the last 24h.
          </div>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-slate-200 bg-slate-50 text-slate-500 dark:border-slate-800 dark:bg-slate-800/50 dark:text-slate-400">
                <tr>
                  <th className="p-3.5 font-medium">Status</th>
                  <th className="p-3.5 font-medium">Runner Name</th>
                  <th className="p-3.5 font-medium">Started At</th>
                  <th className="p-3.5 font-medium">Completed At</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {history.jobs.map((job) => (
                  <tr key={job.id.toString()}>
                    <td className="p-3.5">
                      <span
                        className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                          job.status === "success"
                            ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400"
                            : "bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-400"
                        }`}
                      >
                        {job.status === "success" ? (
                          <CheckCircle2 className="h-3 w-3" />
                        ) : (
                          <XCircle className="h-3 w-3" />
                        )}
                        {job.status}
                      </span>
                    </td>
                    <td className="p-3.5 font-mono text-slate-900 dark:text-slate-100">
                      {job.runnerName}
                    </td>
                    <td className="p-3.5 text-slate-500">{job.startedAt || "-"}</td>
                    <td className="p-3.5 text-slate-500">{job.completedAt || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
