import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  reactStrictMode: true,
  async rewrites() {
    const apiTarget = process.env.OPEN_SPANNER_API_PROXY_URL ?? "http://127.0.0.1:18081";

    return [{ source: "/v1/:path*", destination: `${apiTarget}/v1/:path*` }];
  },
};

export default nextConfig;
