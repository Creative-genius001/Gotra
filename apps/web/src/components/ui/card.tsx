import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

/** Elevated card surface (not glassmorphic) per the Shadow System spec. */
export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--card)] p-6 shadow-md shadow-black/20",
        className,
      )}
      {...props}
    />
  );
}

export function CardTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return <h3 className={cn("text-sm font-medium text-muted-foreground", className)} {...props} />;
}

export function CardValue({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("mt-2 text-3xl font-semibold tracking-tight", className)} {...props} />;
}
