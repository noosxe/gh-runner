import { useAppSettings } from "../lib/api/query-hooks";

export function SettingsPage() {
  const { data: settings, isLoading } = useAppSettings();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          Settings & Administration
        </h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Global supervisor constraints, database snapshots, and audit policies.
        </p>
      </div>

      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <h3 className="text-base font-bold text-slate-900 dark:text-white">App Settings</h3>
        {isLoading ? (
          <div className="mt-2 text-xs text-slate-400">Loading settings...</div>
        ) : !settings || settings.length === 0 ? (
          <div className="mt-2 text-xs text-slate-400">No application settings configured.</div>
        ) : (
          <div className="mt-4 divide-y divide-slate-100 dark:divide-slate-800">
            {settings.map((s) => (
              <div key={s.key} className="flex justify-between py-2 text-xs">
                <span className="font-mono text-slate-700 dark:text-slate-300">{s.key}</span>
                <span className="font-semibold text-slate-900 dark:text-white">{s.value}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
