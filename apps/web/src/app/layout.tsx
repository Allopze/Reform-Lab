import type { Metadata, Viewport } from "next";
import { Geist } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages, getTranslations } from "next-intl/server";
import { AuthProvider } from "@/lib/auth-context";
import { ThemeProvider } from "@/lib/theme-context";
import "./globals.css";

export const dynamic = "force-dynamic";

const geist = Geist({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-geist",
});

function publicAppUrl() {
  const configured = process.env.NEXT_PUBLIC_APP_URL || process.env.APP_URL;
  try {
    return new URL(configured || "https://reformlab.app");
  } catch {
    return new URL("https://reformlab.app");
  }
}

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("metadata");
  const appUrl = publicAppUrl();
  return {
    title: t("title"),
    description: t("description"),
    metadataBase: appUrl,
    openGraph: {
      title: t("ogTitle"),
      description: t("ogDescription"),
      url: appUrl.toString(),
      siteName: "Reform Lab",
      locale: "es_ES",
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: t("ogTitle"),
      description: t("ogDescription"),
    },
    icons: {
      icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
      shortcut: ["/favicon.svg"],
      apple: [{ url: "/favicon.svg", type: "image/svg+xml" }],
    },
    manifest: "/manifest.webmanifest",
  };
}

export const viewport: Viewport = {
  colorScheme: "light dark",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f8fafc" },
    { media: "(prefers-color-scheme: dark)", color: "#050506" },
  ],
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html lang={locale} className={geist.variable}>
      <body className="min-h-screen font-sans text-stone-950">
        <NextIntlClientProvider messages={messages}>
          <ThemeProvider>
            <AuthProvider>{children}</AuthProvider>
          </ThemeProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
