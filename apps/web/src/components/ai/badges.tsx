import { cn } from "@/lib/utils";

/** AIConfidenceBadge — shows a 0–100 confidence score with a Low/Medium/High band. */
export function ConfidenceBadge({ value }: { value: number }) {
  const band = value >= 70 ? "High" : value >= 40 ? "Medium" : "Low";
  const color =
    value >= 70 ? "text-emerald-300" : value >= 40 ? "text-amber-300" : "text-red-300";
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-purple-700/40 bg-purple-950/30 px-2.5 py-0.5 text-xs">
      <span className="text-purple-300">AI</span>
      <span className={color}>
        {band} · {value}
      </span>
    </span>
  );
}

const severityColor: Record<string, string> = {
  low: "text-emerald-300 border-emerald-700/40 bg-emerald-950/20",
  medium: "text-amber-300 border-amber-700/40 bg-amber-950/20",
  high: "text-orange-300 border-orange-700/40 bg-orange-950/20",
  critical: "text-red-300 border-red-700/40 bg-red-950/20",
};

/** SeverityBadge — color-coded severity chip. */
export function SeverityBadge({ value }: { value: string }) {
  const cls = severityColor[value?.toLowerCase()] ?? severityColor.medium;
  return (
    <span className={cn("rounded-full border px-2.5 py-0.5 text-xs capitalize", cls)}>
      {value || "unknown"}
    </span>
  );
}
