import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import Link from "next/link";
import { cookies } from "next/headers";
import { DevLogin } from "@/components/dev-login";
import { ThemeToggle } from "@/components/theme-toggle";
import "./globals.css";

const geist = Geist({ subsets: ["latin"], variable: "--font-sans" });
const mono = Geist_Mono({ subsets: ["latin"], variable: "--font-mono" });

export const metadata: Metadata = { title: "Go Bot — Learn Go by building", description: "Graded, hands-on Go lessons." };

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const cookieStore = await cookies();
  const devAuth = process.env.DEV_AUTH === "true";
  const email = cookieStore.get("dev_email")?.value;
  return <html lang="en" data-theme="dark"><body className={`${geist.variable} ${mono.variable}`}>
    <header className="site-header shell">
      <Link href="/" className="brand"><span className="brand-mark">Go</span> Go Bot</Link>
      <nav className="header-actions" aria-label="Main navigation"><Link href="/courses">Courses</Link><ThemeToggle /><DevLogin enabled={devAuth} email={email} /></nav>
    </header>
    {children}
  </body></html>;
}
