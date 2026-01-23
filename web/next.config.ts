import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  // For static export, we don't need image optimization
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
