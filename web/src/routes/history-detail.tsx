import { useParams, Link } from "@tanstack/react-router";
import { useRunnerLogs } from "../lib/api/query-hooks";
import { ArrowLeft, Terminal } from "lucide-react";

export function HistoryDetailPage() {
  const { jobId } = useParams({ strict: false }) as { jobId?: string };
  const { data: logs, isLoading } = useRunnerLogs(jobId ?? "", !!jobId);

  return (
    <div className="space-y-4">
      <Link
        to="/history"
        className="inline-flex items-center gap-1 text-xs text-blue-600 hover:underline"
      >
        <ArrowLeft className="h-3 w-3" /> Back to History
      </Link>

      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <div className="flex items-center gap-2">
          <Terminal className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          <h1 className="text-lg font-bold text-slate-900 dark:text-white">
            Execution Logs: {jobId}
          </h1>
        </div>

        <div className="mt-4 rounded-xl bg-slate-950 p-4 font-mono text-xs text-slate-300">
          {isLoading ? (
            <div>Loading historical execution logs...</div>
          ) : !logs || logs.length === 0 ? (
            <div className="text-slate-500">No log output recorded for this runner.</div>
          ) : (
            <div className="space-y-1">
              {logs.map((chunk, idx) => (
                <div key={idx} className="flex gap-2">
                  <span className="text-slate-600 select-none">{idx + 1}</span>
                  <span className="text-slate-400">[{chunk.stream}]</span>
                  <span>{chunk.content}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
