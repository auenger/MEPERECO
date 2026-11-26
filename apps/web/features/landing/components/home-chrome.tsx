"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ArrowUpRight, Menu } from "lucide-react";
import { useAuthStore } from "@silieco/core/auth";
import { useDashboardCtaHref } from "../utils/use-dashboard-cta";
import { useLocale, locales, localeLabels } from "../i18n";

const labels = {
  "zh-Hans": {
    login: "登录",
    ecosystem: "协作生态",
    workflow: "如何协作",
    film: "协作短片",
    dark: "切换到暗色主题",
    light: "切换到亮色主题",
  },
  en: {
    login: "Log in",
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
  const signedIn = useAuthStore((state) => !!state.user);
  const accountHref = useDashboardCtaHref();
  const menu = useRef<HTMLDetailsElement>(null);
  const topMarker = useRef<HTMLSpanElement>(null);
  const [compact, setCompact] = useState(false);
  useEffect(() => {
    if (!topMarker.current) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry) setCompact(!entry.isIntersecting);
    });
    observer.observe(topMarker.current);
    return () => observer.disconnect();
  }, []);
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
    <>
      <span className="home-header-marker" ref={topMarker} aria-hidden="true" />
      <header className="home-header" data-compact={compact}>
        <div className="home-shell home-header-inner">
          <a href="#product" className="home-brand">
            <span>
              <img
                src={theme === "dark" ? "/brand/meper-studio-horizontal-white.svg" : "/brand/meper-studio-horizontal-color.svg"}
                alt="MEPER Studio"
                width={544}
                height={112}
                className="h-auto w-[160px] sm:w-[180px]"
              />
              <small aria-hidden={compact}>HUMAN × AGENT</small>
            </span>
          </a>
          <nav className="home-desktop-nav" aria-label={t.header.navigation}>
            {links.map((link) => (
              <a key={link.href} href={link.href}>
                {link.label}
              </a>
            ))}
          </nav>
          <Link href={accountHref} className="home-header-cta">
            {signedIn ? t.header.dashboard : c.login}
            <ArrowUpRight size={15} aria-hidden="true" />
          </Link>
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
    </>
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
