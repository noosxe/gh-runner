import { useState } from "react";
import type { ImageUpdate } from "../../gen/api_pb";
import { usePullImage, useDismissImageUpdate } from "../../lib/api/query-hooks";
import { AlertCircle, DownloadCloud, X, Loader2, Check } from "lucide-react";

export interface ImageUpdateNotificationProps {
  updates: ImageUpdate[];
  poolNameLookup?: Record<string, string>;
}

export function ImageUpdateNotification({
  updates,
  poolNameLookup = {},
}: ImageUpdateNotificationProps) {
  const pullMutation = usePullImage();
  const dismissMutation = useDismissImageUpdate();
  const [activePullId, setActivePullId] = useState<bigint | null>(null);
  const [pulledIds, setPulledIds] = useState<Set<bigint>>(new Set());

  if (!updates || updates.length === 0) return null;

  const handlePull = async (poolId: bigint) => {
    setActivePullId(poolId);
    try {
      await pullMutation.mutateAsync(poolId);
      setPulledIds((prev) => new Set([...prev, poolId]));
    } finally {
      setActivePullId(null);
    }
  };

  const handleDismiss = async (updateId: bigint) => {
    await dismissMutation.mutateAsync(updateId);
  };

  return (
    <div className="space-y-3">
      {updates.map((up) => {
        const poolName = poolNameLookup[up.poolId.toString()] ?? `Pool #${up.poolId}`;
        const isPulling = activePullId === up.poolId;
        const isPulled = pulledIds.has(up.poolId);

        return (
          <div
            key={up.id.toString()}
            className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-4 shadow-xs dark:border-amber-900/60 dark:bg-amber-950/30"
          >
            <div className="flex items-start gap-3">
              <div className="rounded-xl bg-amber-500/10 p-2 text-amber-600 dark:text-amber-400 shrink-0">
                <AlertCircle className="h-5 w-5" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold text-amber-900 dark:text-amber-300 uppercase tracking-wider">
                    Runner Image Update Available
                  </span>
                  <span className="rounded-md bg-amber-200/60 dark:bg-amber-900/60 px-2 py-0.5 text-[10px] font-mono font-semibold text-amber-900 dark:text-amber-200">
                    {poolName}
                  </span>
                </div>
                <p className="mt-1 text-xs text-amber-800 dark:text-amber-300/90 font-mono">
                  Current: <span className="font-semibold">{up.currentImage}</span> &rarr; Latest:{" "}
                  <span className="font-semibold">{up.latestDigest}</span>
                </p>
                <p className="mt-0.5 text-[11px] text-amber-750/80 dark:text-amber-400/80">
                  New containers in this pool will use the updated image without interrupting active
                  runners.
                </p>
              </div>
            </div>

            <div className="flex items-center gap-2 shrink-0 self-end sm:self-auto">
              {isPulled ? (
                <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                  <Check className="h-4 w-4" />
                  <span>Image Pulled</span>
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => handlePull(up.poolId)}
                  disabled={isPulling}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-amber-700 disabled:opacity-50 dark:bg-amber-500 dark:hover:bg-amber-600 transition-colors"
                >
                  {isPulling ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <DownloadCloud className="h-3.5 w-3.5" />
                  )}
                  <span>{isPulling ? "Pulling..." : "Pull Update"}</span>
                </button>
              )}

              <button
                type="button"
                onClick={() => handleDismiss(up.id)}
                className="rounded-xl p-1.5 text-amber-700 hover:bg-amber-200/50 dark:text-amber-400 dark:hover:bg-amber-900/50 transition-colors"
                title="Dismiss update notification"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
