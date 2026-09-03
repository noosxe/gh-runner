import { useParams, Link } from "@tanstack/react-router";
import { usePools } from "../lib/api/query-hooks";
import { ArrowLeft } from "lucide-react";

export function PoolDetailPage() {
  const { poolId } = useParams({ strict: false }) as { poolId?: string };
  const { data: pools } = usePools();
  const pool = pools?.find((p) => p.id.toString() === poolId);

  if (!pool) {
    return (
      <div className="space-y-4">
        <Link
          to="/pools"
          className="inline-flex items-center gap-1 text-xs text-blue-600 hover:underline"
        >
          <ArrowLeft className="h-3 w-3" /> Back to Pools
        </Link>
        <div className="rounded-2xl border border-slate-200 bg-white p-8 text-center text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900">
          Pool not found or loading...
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <Link
        to="/pools"
        className="inline-flex items-center gap-1 text-xs text-blue-600 hover:underline"
      >
        <ArrowLeft className="h-3 w-3" /> Back to Pools
      </Link>

      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-slate-900 dark:text-white">{pool.name}</h1>
            <p className="text-xs text-slate-500">{pool.repositoryUrl}</p>
          </div>
          <span className="inline-flex items-center rounded-md bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-600 dark:bg-blue-950/40 dark:text-blue-400">
            {pool.provider}
          </span>
        </div>

        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-4 border-t border-slate-100 pt-6 dark:border-slate-800 text-xs">
          <div>
            <span className="text-slate-400">Scope</span>
            <div className="mt-1 font-semibold text-slate-900 dark:text-white">
              {pool.scope || "repo"}
            </div>
          </div>
          <div>
            <span className="text-slate-400">CPU Limit</span>
            <div className="mt-1 font-semibold text-slate-900 dark:text-white">
              {pool.cpuLimit || "unlimited"}
            </div>
          </div>
          <div>
            <span className="text-slate-400">Memory Limit</span>
            <div className="mt-1 font-semibold text-slate-900 dark:text-white">
              {pool.memoryLimit || "unlimited"}
            </div>
          </div>
          <div>
            <span className="text-slate-400">Docker Privileges</span>
            <div className="mt-1 font-semibold text-slate-900 dark:text-white">
              {pool.allowDocker ? "Enabled" : "Disabled"}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
