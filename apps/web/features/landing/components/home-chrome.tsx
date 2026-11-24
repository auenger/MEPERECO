"use client";
import { useEffect, useRef, useState } from "react";
import { Menu } from "lucide-react";
import { SiliecoIcon } from "@silieco/ui/components/common/silieco-icon";
import { useLocale, locales, localeLabels } from "../i18n";

const labels = {
  "zh-Hans": {
    ecosystem: "协作生态",
    workflow: "如何协作",
    film: "协作短片",
    dark: "切换到暗色主题",
    light: "切换到亮色主题",
  },
  en: {
    ecosystem: "Ecosystem",
    workflow: "How it works",
    film: "Watch the film",
    dark: "Switch to dark theme",
    light: "Switch to light theme",
  },
};
export function HomeHeader({
  theme,
  onToggleTheme,
}: {
  theme: "light" | "dark";
  onToggleTheme: () => void;
}) {
  const { t, locale } = useLocale();
  const c = labels[locale];
  const menu = useRef<HTMLDetailsElement>(null);
  const [turning, setTurning] = useState(false);
  useEffect(() => {
    if (!turning) return;
    const timer = window.setTimeout(() => setTurning(false), 520);
    return () => window.clearTimeout(timer);
  }, [turning, theme]);
  const links = [
    { href: "#ecosystem", label: c.ecosystem },
    { href: "#how-it-works", label: c.workflow },
    { href: "#film", label: c.film },
  ];
  return (
    <header className="home-header">
      <div className="home-shell home-header-inner">
        <a href="#product" className="home-brand">
          <SiliecoIcon className="size-8" noSpin />
          <span>
            Silieco<small>HUMAN × AGENT</small>
          </span>
        </a>
        <nav className="home-desktop-nav" aria-label={t.header.navigation}>
          {links.map((link) => (
            <a key={link.href} href={link.href}>
              {link.label}
            </a>
          ))}
        </nav>
        <button
          className={`home-theme-toggle${turning ? " is-turning" : ""}`}
          type="button"
          aria-label={theme === "light" ? c.dark : c.light}
          aria-pressed={theme === "dark"}
          onClick={() => {
            setTurning(true);
            onToggleTheme();
          }}
        >
          <span className="home-theme-corner" aria-hidden="true" />
          <span className="home-theme-fold" aria-hidden="true" />
          <span className="home-theme-hint" aria-hidden="true">
            {theme === "light" ? c.dark : c.light}
          </span>
        </button>
        <details
          className="home-mobile-menu"
          ref={menu}
          onKeyDown={(event) => {
            if (event.key === "Escape" && menu.current) {
              menu.current.open = false;
              menu.current.querySelector("summary")?.focus();
            }
          }}
        >
          <summary aria-label={t.header.openMenu}>
            <Menu size={18} />
          </summary>
          <nav aria-label={t.header.navigation}>
            {links.map((link) => (
              <a
                key={link.href}
                href={link.href}
                onClick={() => {
                  if (menu.current) menu.current.open = false;
                }}
              >
                {link.label}
              </a>
            ))}
          </nav>
        </details>
      </div>
    </header>
  );
}
export function HomeFooter() {
  const { t, locale, setLocale } = useLocale();
  return (
    <footer className="home-footer home-shell">
      <div className="home-footer-bottom">
        <p>
          {t.footer.copyright.replace(
            "{year}",
            String(new Date().getFullYear()),
          )}
        </p>
        <div>
          {locales.map((l) => (
            <button
              key={l}
              aria-pressed={locale === l}
              onClick={() => setLocale(l)}
            >
              {localeLabels[l]}
            </button>
          ))}
        </div>
      </div>
    </footer>
  );
}
