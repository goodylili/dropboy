import type { NextConfig } from "next";

const daemon = process.env.DROPBOY_API ?? "http://127.0.0.1:7777";
const isExport = process.env.DROPBOY_EXPORT === "1";

// When building for embedding into the Go binary (DROPBOY_EXPORT=1), Next.js
// emits a static export under frontend/out. The Go daemon then serves the
// SPA same-origin, so no rewrites are needed. In dev, rewrites proxy /api/*
// to the daemon at DROPBOY_API.
const nextConfig: NextConfig = isExport
  ? {
      output: "export",
      trailingSlash: true,
      images: { unoptimized: true },
    }
  : {
      async rewrites() {
        return [{ source: "/api/:path*", destination: `${daemon}/api/:path*` }];
      },
    };

export default nextConfig;
