import { Sun, Moon, Monitor, ShieldCheck, Activity } from "lucide-react";
import { useTheme } from "./hooks/use-theme";

export function App() {
  const { theme, resolvedTheme, setTheme } = useTheme();

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-6 text-center">
      <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-8 shadow-sm transition-all dark:border-slate-800 dark:bg-slate-800/90 dark:shadow-none">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
          <ShieldCheck className="h-6 w-6" />
        </div>

        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          Runnero Supervisor
        </h1>
        <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
          Single Page Application Web Control Interface
        </p>

        <div className="my-6 flex items-center justify-center gap-2 rounded-xl border border-slate-100 bg-slate-50 p-3 text-xs font-medium text-slate-600 dark:border-slate-700/50 dark:bg-slate-800/50 dark:text-slate-300">
          <Activity className="h-4 w-4 text-emerald-500 animate-pulse" />
          <span>Active Theme:</span>
          <span className="font-semibold uppercase tracking-wider text-blue-600 dark:text-blue-400">
            {resolvedTheme} ({theme})
          </span>
        </div>

        <div className="flex items-center justify-center gap-2">
          <button
            type="button"
            onClick={() => setTheme("light")}
            className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              theme === "light"
                ? "bg-blue-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-700 dark:text-slate-300 dark:hover:bg-slate-600"
            }`}
          >
            <Sun className="h-3.5 w-3.5" />
            Light
          </button>
          <button
            type="button"
            onClick={() => setTheme("dark")}
            className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              theme === "dark"
                ? "bg-blue-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-700 dark:text-slate-300 dark:hover:bg-slate-600"
            }`}
          >
            <Moon className="h-3.5 w-3.5" />
            Dark
          </button>
          <button
            type="button"
            onClick={() => setTheme("system")}
            className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              theme === "system"
                ? "bg-blue-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-700 dark:text-slate-300 dark:hover:bg-slate-600"
            }`}
          >
            <Monitor className="h-3.5 w-3.5" />
            System
          </button>
        </div>
      </div>
    </div>
  );
}
