import type { Metadata } from "next";
import { Providers } from "@/lib/providers";
import "@/styles/globals.css";

export const metadata: Metadata = {
  title: "Gotra — tunnels & AI debugging for developers",
  description:
    "Securely expose localhost, capture and replay requests, and let an AI debugging layer explain errors and investigate incidents.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  // Dark mode is the default theme.
  return (
    <html lang="en" className="dark">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
