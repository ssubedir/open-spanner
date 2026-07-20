import type { Metadata } from "next";
import "@/product/index.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "Open Spanner",
  description: "Usage metering and entitlement operations",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
