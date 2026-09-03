import { CheckCircle2, AlertTriangle, AlertOctagon } from "lucide-react";

export interface CapacityHealthProps {
  avgQueueSeconds: number;
}

export function getCapacityStatus(avgQueueSeconds: number): {
  status: "optimal" | "moderate" | "constrained";
  label: string;
  description: string;
  badgeClass: string;
  dotClass: string;
} {
  if (avgQueueSeconds < 5.0) {
    return {
      status: "optimal",
      label: "Optimal Capacity",
      description:
        "Warm idle runners immediately pick up incoming workflow jobs with sub-5s latency.",
      badgeClass:
        "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400",
      dotClass: "bg-emerald-500",
    };
  }
  if (avgQueueSeconds <= 30.0) {
    return {
      status: "moderate",
      label: "Moderate Load",
      description:
        "Cold container spin-up overhead observed. Consider increasing warm idle runner targets.",
      badgeClass:
        "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-400",
      dotClass: "bg-amber-500",
    };
  }
  return {
    status: "constrained",
    label: "Capacity Constrained",
    description:
      "High queue wait latency detected (>30s). Scaling bottleneck; increase max concurrency.",
    badgeClass:
      "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-400",
    dotClass: "bg-rose-500",
  };
}

export function CapacityHealthBadge({ avgQueueSeconds }: CapacityHealthProps) {
  const info = getCapacityStatus(avgQueueSeconds);

  return (
    <div
      className={`inline-flex items-center gap-2 rounded-xl border px-3 py-1.5 text-xs font-semibold shadow-2xs ${info.badgeClass}`}
      title={info.description}
    >
      <span className={`h-2 w-2 rounded-full ${info.dotClass} animate-pulse`} />
      {info.status === "optimal" ? (
        <CheckCircle2 className="h-3.5 w-3.5" />
      ) : info.status === "moderate" ? (
        <AlertTriangle className="h-3.5 w-3.5" />
      ) : (
        <AlertOctagon className="h-3.5 w-3.5" />
      )}
      <span>{info.label}</span>
    </div>
  );
}
