import { useNavigate } from "@tanstack/react-router";
import { useOnboardingStatus } from "../lib/api/query-hooks";
import { ShieldCheck } from "lucide-react";

export function OnboardingPage() {
  const { data: status } = useOnboardingStatus();
  const navigate = useNavigate();

  return (
    <div className="flex min-h-screen items-center justify-center p-4 bg-slate-50 dark:bg-slate-950">
      <div className="w-full max-w-lg rounded-2xl border border-slate-200 bg-white p-8 shadow-xs dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-blue-600 text-white shadow-sm">
          <ShieldCheck className="h-6 w-6" />
        </div>

        <h1 className="text-center text-2xl font-bold text-slate-900 dark:text-white">
          System Onboarding
        </h1>
        <p className="mt-1 text-center text-xs text-slate-500">
          First-time supervisor initialization and configuration wizard
        </p>

        <div className="mt-6 space-y-3 rounded-xl border border-slate-100 bg-slate-50 p-4 text-xs dark:border-slate-800 dark:bg-slate-800/50">
          <div className="flex items-center justify-between">
            <span>Admin Created</span>
            <span
              className={status?.adminCreated ? "font-bold text-emerald-600" : "text-slate-400"}
            >
              {status?.adminCreated ? "Done" : "Pending"}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span>Auth Profile Exists</span>
            <span
              className={
                status?.authProfileExists ? "font-bold text-emerald-600" : "text-slate-400"
              }
            >
              {status?.authProfileExists ? "Done" : "Pending"}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span>Pool Exists</span>
            <span className={status?.poolExists ? "font-bold text-emerald-600" : "text-slate-400"}>
              {status?.poolExists ? "Done" : "Pending"}
            </span>
          </div>
        </div>

        <div className="mt-6">
          <button
            type="button"
            onClick={() => navigate({ to: "/login" })}
            className="w-full rounded-xl bg-blue-600 py-2.5 text-xs font-semibold text-white shadow-sm hover:bg-blue-700"
          >
            Continue to Login
          </button>
        </div>
      </div>
    </div>
  );
}
