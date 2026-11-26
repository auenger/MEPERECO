import type { MetadataRoute } from "next";
export default function sitemap(): MetadataRoute.Sitemap {
  return [
    {
      url: "https://studio.meper.cn",
      lastModified: new Date("2026-10-26"),
      changeFrequency: "monthly",
      priority: 1,
    },
  ];
}
