import { LocaleProvider } from "@/features/landing/i18n";
import { getRequestLocale } from "@/lib/request-locale";

const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      name: "MEPER Studio",
    },
    {
      "@type": "SoftwareApplication",
      name: "MEPER Studio",
      applicationCategory: "ProjectManagement",
      operatingSystem: "Web",
      description:
        "Open-source workspace for human and agent collaboration through tasks, SOPs, and evidence-based handoffs.",
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
    },
  ],
};

export default async function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const initialLocale = await getRequestLocale();

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <div className="landing-light h-full overflow-x-hidden overflow-y-auto bg-white">
        <LocaleProvider initialLocale={initialLocale}>
          {children}
        </LocaleProvider>
      </div>
    </>
  );
}
