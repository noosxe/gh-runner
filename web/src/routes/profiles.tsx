import { useState, type FormEvent } from "react";
import {
  useAuthProfiles,
  useCreateAuthProfile,
  useDeleteAuthProfile,
} from "../lib/api/query-hooks";
import {
  KeyRound,
  ShieldCheck,
  Plus,
  Trash2,
  X,
  AlertCircle,
  ExternalLink,
  CheckCircle2,
} from "lucide-react";

export function ProfilesPage() {
  const { data: profiles, isLoading } = useAuthProfiles();
  const createProfileMutation = useCreateAuthProfile();
  const deleteProfileMutation = useDeleteAuthProfile();

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form State
  const [profileName, setProfileName] = useState("");
  const [authMethod, setAuthMethod] = useState<
    "github_app" | "github_pat" | "gitea_pat" | "forgejo_pat"
  >("github_pat");
  const [appId, setAppId] = useState("");
  const [privateKeyPem, setPrivateKeyPem] = useState("");
  const [token, setToken] = useState("");

  const resetForm = () => {
    setProfileName("");
    setAuthMethod("github_pat");
    setAppId("");
    setPrivateKeyPem("");
    setToken("");
    setError(null);
  };

  const handleOpenModal = () => {
    resetForm();
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    resetForm();
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!profileName.trim()) {
      setError("Profile name is required");
      return;
    }

    try {
      if (authMethod === "github_app") {
        if (!appId.trim() || !privateKeyPem.trim()) {
          setError("GitHub App ID and Private Key PEM are required");
          return;
        }
        const encoder = new TextEncoder();
        await createProfileMutation.mutateAsync({
          name: profileName.trim(),
          authMethod: "github_app",
          appId: BigInt(appId.trim()),
          privateKey: encoder.encode(privateKeyPem.trim()),
          token: "",
        });
      } else {
        if (!token.trim()) {
          setError("Personal Access Token (PAT) is required");
          return;
        }
        await createProfileMutation.mutateAsync({
          name: profileName.trim(),
          authMethod,
          appId: 0n,
          privateKey: new Uint8Array(),
          token: token.trim(),
        });
      }
      handleCloseModal();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create authentication profile");
    }
  };

  const handleDelete = async (id: bigint, name: string) => {
    if (confirm(`Are you sure you want to delete authentication profile "${name}"?`)) {
      try {
        await deleteProfileMutation.mutateAsync(id);
      } catch (err: unknown) {
        alert(err instanceof Error ? err.message : "Failed to delete authentication profile");
      }
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Git Auth Profiles
          </h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Credentials for requesting ephemeral runner registration tokens from Git providers.
          </p>
        </div>

        <button
          type="button"
          onClick={handleOpenModal}
          className="inline-flex items-center justify-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-xs hover:bg-blue-500 transition-colors"
        >
          <Plus className="h-4 w-4" />
          <span>Add Auth Profile</span>
        </button>
      </div>

      {isLoading ? (
        <div className="text-sm text-slate-400">Loading auth profiles...</div>
      ) : !profiles || profiles.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 p-12 text-center text-slate-500 dark:border-slate-800 dark:text-slate-400">
          <KeyRound className="mx-auto h-8 w-8 text-slate-400 mb-2" />
          <p className="text-base font-semibold text-slate-800 dark:text-slate-200">
            No auth profiles configured
          </p>
          <p className="text-xs text-slate-500 mt-1 max-w-md mx-auto">
            Connect a GitHub App or Personal Access Token (PAT) for GitHub, Gitea, or Forgejo to
            begin orchestrating runner pools.
          </p>
          <div className="mt-4">
            <button
              type="button"
              onClick={handleOpenModal}
              className="inline-flex items-center gap-1.5 rounded-xl bg-blue-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition-colors hover:bg-blue-500"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>Add First Profile</span>
            </button>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {profiles.map((prof) => (
            <div
              key={prof.id.toString()}
              className="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-slate-800 dark:bg-slate-900 flex flex-col justify-between"
            >
              <div>
                <div className="flex items-center justify-between">
                  <span className="font-bold text-slate-900 dark:text-white">{prof.name}</span>
                  <span className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-semibold uppercase text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                    {prof.authMethod}
                  </span>
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-slate-500">
                  <div className="flex items-center gap-1.5">
                    <ShieldCheck className="h-4 w-4 text-emerald-500" />
                    <span>Encrypted AES-256 (Write-Only)</span>
                  </div>

                  {prof.authMethod === "github_app" &&
                    (prof.installationsCount > 0 ? (
                      <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 border border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800">
                        <CheckCircle2 className="h-3 w-3" />
                        <span>
                          Installed on {prof.installationsCount}{" "}
                          {prof.installationsCount === 1 ? "account" : "accounts"}
                        </span>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 border border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800">
                        <AlertCircle className="h-3 w-3" />
                        <span>Not Installed</span>
                      </span>
                    ))}
                </div>
              </div>

              <div className="mt-5 flex items-center justify-between border-t border-slate-100 pt-3 dark:border-slate-800">
                <div>
                  {prof.authMethod === "github_app" && prof.installUrl && (
                    <a
                      href={prof.installUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`inline-flex items-center gap-1 text-xs font-semibold ${
                        prof.installationsCount === 0
                          ? "text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                          : "text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"
                      }`}
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      <span>
                        {prof.installationsCount === 0 ? "Install App" : "Configure Access"}
                      </span>
                    </a>
                  )}
                </div>

                <button
                  type="button"
                  onClick={() => handleDelete(prof.id, prof.name)}
                  disabled={deleteProfileMutation.isPending}
                  className="inline-flex items-center gap-1 text-xs font-medium text-rose-600 hover:text-rose-700 dark:text-rose-400 dark:hover:text-rose-300"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  <span>Delete Profile</span>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add Profile Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 backdrop-blur-xs p-4">
          <div className="w-full max-w-lg rounded-2xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-800 dark:bg-slate-900 text-xs">
            <div className="flex items-center justify-between border-b border-slate-100 pb-3 dark:border-slate-800">
              <div className="flex items-center gap-2">
                <KeyRound className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  Add Git Auth Profile
                </h3>
              </div>
              <button
                type="button"
                onClick={handleCloseModal}
                className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {error && (
              <div className="mt-3 flex items-center gap-2 rounded-xl border border-rose-200 bg-rose-50/80 p-3 text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="mt-4 space-y-4">
              <div>
                <label className="font-semibold text-slate-700 dark:text-slate-300">
                  Provider Method
                </label>
                <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
                  {[
                    { id: "github_pat", label: "GitHub PAT" },
                    { id: "github_app", label: "GitHub App" },
                    { id: "gitea_pat", label: "Gitea PAT" },
                    { id: "forgejo_pat", label: "Forgejo PAT" },
                  ].map((m) => (
                    <button
                      key={m.id}
                      type="button"
                      onClick={() => setAuthMethod(m.id as any)}
                      className={`rounded-xl border p-2 text-center font-medium transition-all ${
                        authMethod === m.id
                          ? "border-blue-500 bg-blue-50/50 text-blue-700 font-semibold dark:border-blue-500 dark:bg-blue-950/30 dark:text-blue-300"
                          : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                      }`}
                    >
                      {m.label}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label
                  htmlFor="modal-profile-name"
                  className="font-semibold text-slate-700 dark:text-slate-300"
                >
                  Profile Name
                </label>
                <input
                  id="modal-profile-name"
                  type="text"
                  placeholder="e.g. github-production"
                  value={profileName}
                  onChange={(e) => setProfileName(e.target.value)}
                  className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  required
                />
              </div>

              {authMethod === "github_app" ? (
                <>
                  <div>
                    <label
                      htmlFor="modal-app-id"
                      className="font-semibold text-slate-700 dark:text-slate-300"
                    >
                      GitHub App ID
                    </label>
                    <input
                      id="modal-app-id"
                      type="number"
                      placeholder="e.g. 123456"
                      value={appId}
                      onChange={(e) => setAppId(e.target.value)}
                      className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                      required
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="modal-private-key"
                      className="font-semibold text-slate-700 dark:text-slate-300"
                    >
                      Private Key (.pem)
                    </label>
                    <textarea
                      id="modal-private-key"
                      rows={4}
                      placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                      value={privateKeyPem}
                      onChange={(e) => setPrivateKeyPem(e.target.value)}
                      className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 font-mono text-[11px] text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                      required
                    />
                  </div>
                </>
              ) : (
                <div>
                  <label
                    htmlFor="modal-token"
                    className="font-semibold text-slate-700 dark:text-slate-300"
                  >
                    Personal Access Token (PAT)
                  </label>
                  <input
                    id="modal-token"
                    type="password"
                    placeholder="ghp_... or gitea_pat_..."
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900 focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    required
                  />
                  <p className="mt-1 text-[11px] text-slate-400">
                    Encrypted at rest using AES-256 in supervisor database.
                  </p>
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
                <button
                  type="button"
                  onClick={handleCloseModal}
                  className="rounded-xl border border-slate-200 px-4 py-2 font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createProfileMutation.isPending}
                  className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white shadow-xs hover:bg-blue-500 disabled:opacity-50"
                >
                  {createProfileMutation.isPending ? "Saving..." : "Save Profile"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
