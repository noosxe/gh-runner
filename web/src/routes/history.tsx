import { useJobHistory } from "../lib/api/query-hooks";
import { Link } from "@tanstack/react-router";
import { History, CheckCircle2, XCircle } from "lucide-react";

export function HistoryPage() {
  const { data: history, isLoading } = useJobHistory(undefined, 25, 0);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          Job Execution History
        </h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Historical records of ephemeral runner jobs, execution durations, and logs.
        </p>
      </div>

      {isLoading ? (
        <div className="text-sm text-slate-400">Loading history...</div>
      ) : !history?.jobs || history.jobs.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <History className="mx-auto h-8 w-8 text-slate-400 mb-2" />
          <p className="text-base font-semibold text-slate-800 dark:text-slate-200">
            No job history
          </p>
          <p className="text-xs text-slate-500 mt-1">
            Completed runner executions will appear here.
          </p>
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
                <th className="p-3.5 font-medium">Actions</th>
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
                  <td className="p-3.5">
                    <Link
                      to="/history/$jobId"
                      params={{ jobId: job.id.toString() }}
                      className="font-semibold text-blue-600 hover:underline dark:text-blue-400"
                    >
                      Logs &rarr;
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
