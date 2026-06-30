import type { InputHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
}

/** Labelled text input used across the auth forms. */
export function Field({ label, className, ...props }: FieldProps) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium">{label}</span>
      <input
        className={cn(
          "h-10 w-full rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500",
          className,
        )}
        {...props}
      />
    </label>
  );
}
