import path from "node:path";

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  transpilePackages: ["@gotra/design-tokens"],
  // Self-contained server build for Docker. outputFileTracingRoot points at the
  // monorepo root so workspace dependencies are traced correctly.
  output: "standalone",
  outputFileTracingRoot: path.join(process.cwd(), "../../"),
};

export default nextConfig;
