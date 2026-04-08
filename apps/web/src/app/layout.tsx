import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";

const geist = Geist({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-geist",
});

export const metadata: Metadata = {
  title: "Reform Lab — Conversion de archivos inteligente",
  description:
    "Reform Lab detecta el tipo real del archivo y muestra solo conversiones compatibles en una interfaz clara y segura.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="es" className={geist.variable}>
      <body className="min-h-screen font-sans text-stone-950">{children}</body>
    </html>
  );
}
