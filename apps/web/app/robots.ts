import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  const baseUrl = "https://studio.meper.cn";

  return {
    rules: [
      {
        userAgent: "*",
        allow: ["/"],
        disallow: [
          "/api/",
          "/ws",
          "/auth/",
          "/tasks",
          "/board",
          "/inbox",
          "/agents",
          "/settings",
          "/my-tasks",
          "/runtimes",
          "/skills",
        ],
      },
    ],
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}
