"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useAuth } from "@/lib/auth";

/** App-wide client providers (TanStack Query) + session bootstrap. */
export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { staleTime: 30_000, refetchOnWindowFocus: false },
        },
      }),
  );

  // Restore the session once on first mount via the refresh cookie.
  const bootstrap = useAuth((s) => s.bootstrap);
  const started = useRef(false);
  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void bootstrap();
  }, [bootstrap]);

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
