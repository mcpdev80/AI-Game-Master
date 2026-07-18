/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: "standalone",
  experimental: {
    proxyClientMaxBodySize: "250mb",
  },
  async rewrites() {
    const apiOrigin = process.env.API_INTERNAL_BASE_URL ?? "http://dungeon-master-api:8080";
    return {
      beforeFiles: [
        {
          source: "/api/:path*",
          destination: `${apiOrigin.replace(/\/$/, "")}/api/:path*`,
        },
      ],
    };
  },
};

export default nextConfig;
