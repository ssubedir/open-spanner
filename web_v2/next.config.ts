import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  distDir: process.env.OPEN_SPANNER_NEXT_DIST_DIR || ".next",
  output: "standalone",
  reactStrictMode: true,
};

export default nextConfig;
