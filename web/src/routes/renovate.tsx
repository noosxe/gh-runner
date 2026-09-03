import { Bot } from "lucide-react";

export function RenovatePage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          Renovate Bot
        </h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Automated dependency update scheduling and task container runs.
        </p>
      </div>

      <div className="rounded-2xl border border-slate-200 bg-white p-8 text-center text-slate-500 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-400">
        <Bot className="mx-auto h-8 w-8 text-blue-500 mb-2" />
        <h3 className="text-base font-semibold text-slate-800 dark:text-slate-200">
          Renovate Bot Scheduler
        </h3>
        <p className="text-xs mt-1">
          Renovate task containers run periodically per pool cron schedules.
        </p>
      </div>
    </div>
  );
}
