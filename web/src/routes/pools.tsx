import { usePools } from "../lib/api/query-hooks";
import { Link } from "@tanstack/react-router";
import { Server } from "lucide-react";

export function PoolsPage() {
  const { data: pools, isLoading } = usePools();

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Runner Pools
          </h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Manage ephemeral worker pools and provider scaling limits.
          </p>
        </div>
      </div>

      {isLoading ? (
        <div className="text-sm text-slate-400">Loading runner pools...</div>
      ) : !pools || pools.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <Server className="mx-auto h-8 w-8 text-slate-400 mb-2" />
          <p className="text-base font-semibold text-slate-800 dark:text-slate-200">
            No pools configured
          </p>
          <p className="text-xs text-slate-500 mt-1">
            Configure your first pool to start dispatching jobs.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {pools.map((p) => (
            <div
              key={p.id.toString()}
              className="rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900"
            >
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-bold text-slate-900 dark:text-white">{p.name}</h3>
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    {p.repositoryUrl}
                  </span>
                </div>
                <span className="rounded-md bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-600 dark:bg-blue-950/40 dark:text-blue-400">
                  {p.provider}
                </span>
              </div>

              <div className="mt-4 grid grid-cols-3 gap-2 border-y border-slate-100 py-3 text-center text-xs dark:border-slate-800">
                <div>
                  <span className="text-slate-400">Active</span>
                  <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                    {p.activeRunners}
                  </div>
                </div>
                <div>
                  <span className="text-slate-400">Idle Target</span>
                  <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                    {p.minIdleRunners}
                  </div>
                </div>
                <div>
                  <span className="text-slate-400">Max Capacity</span>
                  <div className="mt-0.5 text-base font-bold text-slate-900 dark:text-white">
                    {p.maxConcurrency}
                  </div>
                </div>
              </div>

              <div className="mt-4 flex items-center justify-between text-xs">
                <span className="text-slate-500">Image: {p.runnerImage || "default"}</span>
                <Link
                  to="/pools/$poolId"
                  params={{ poolId: p.id.toString() }}
                  className="flex items-center gap-1 font-semibold text-blue-600 hover:underline dark:text-blue-400"
                >
                  Manage &rarr;
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
