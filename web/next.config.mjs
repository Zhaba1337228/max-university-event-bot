/** @type {import('next').NextConfig} */
const apiUpstream = process.env.API_UPSTREAM || "http://localhost:8081";

const nextConfig = {
  reactStrictMode: true,
  output: "standalone",
  // Proxy /api/* в Go-бэкенд, чтобы cookie session_jwt оставались same-origin.
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiUpstream}/api/:path*` }];
  },
};

export default nextConfig;
