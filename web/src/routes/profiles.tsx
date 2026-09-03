import { useAuthProfiles } from "../lib/api/query-hooks";
import { KeyRound, ShieldCheck } from "lucide-react";

export function ProfilesPage() {
  const { data: profiles, isLoading } = useAuthProfiles();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
          Git Auth Profiles
        </h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Credentials for requesting ephemeral runner registration tokens.
        </p>
      </div>

      {isLoading ? (
        <div className="text-sm text-slate-400">Loading auth profiles...</div>
      ) : !profiles || profiles.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <KeyRound className="mx-auto h-8 w-8 text-slate-400 mb-2" />
          <p className="text-base font-semibold text-slate-800 dark:text-slate-200">
            No auth profiles
          </p>
          <p className="text-xs text-slate-500 mt-1">Configure GitHub App or PAT credentials.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {profiles.map((prof) => (
            <div
              key={prof.id.toString()}
              className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900"
            >
              <div className="flex items-center justify-between">
                <span className="font-bold text-slate-900 dark:text-white">{prof.name}</span>
                <span className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                  {prof.authMethod}
                </span>
              </div>
              <div className="mt-3 flex items-center gap-2 text-xs text-slate-500">
                <ShieldCheck className="h-4 w-4 text-emerald-500" />
                <span>Encrypted AES-256 (Write-Only)</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
