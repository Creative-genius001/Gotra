import type { AnalysisResult } from "@/lib/ai";
import { ConfidenceBadge, SeverityBadge } from "@/components/ai/badges";

/** AIAnalysisPanel — renders a structured AnalysisResult (AI Design Language). */
export function AnalysisPanel({
  result,
  provider,
}: {
  result: AnalysisResult;
  provider?: string;
}) {
  return (
    <div className="space-y-4 rounded-[var(--radius-lg)] border border-purple-800/30 bg-[linear-gradient(135deg,rgba(76,29,149,0.12),rgba(124,58,237,0.06))] p-5">
      <div className="flex items-center gap-2">
        <SeverityBadge value={result.severity} />
        <ConfidenceBadge value={result.confidence} />
        {provider && (
          <span className="ml-auto text-xs text-muted-foreground">via {provider}</span>
        )}
      </div>
      <Field label="Explanation" value={result.explanation} />
      <Field label="Root cause" value={result.root_cause} />
      <Field label="Suggested fix" value={result.suggested_fix} />
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="mb-1 text-xs font-medium uppercase tracking-wider text-purple-300/80">
        {label}
      </p>
      <p className="text-sm text-foreground/90">{value}</p>
    </div>
  );
}
