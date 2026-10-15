import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { motion } from "framer-motion";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "ghost" };

export function Button({ className = "", variant = "primary", ...props }: ButtonProps) {
  return <button className={`button button-${variant} ${className}`} {...props} />;
}

export function Card({ className = "", ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={`card ${className}`} {...props} />;
}

export function Badge({ children, className = "", ...props }: HTMLAttributes<HTMLSpanElement>) {
  return <span className={`badge ${className}`} {...props}>{children}</span>;
}

export function DifficultyBadge({ difficulty }: { difficulty: string }) {
  return <Badge className={`difficulty difficulty-${difficulty}`}>{difficulty}</Badge>;
}

export function BossBadge() {
  return <Badge className="boss-badge">Boss</Badge>;
}

export function Skeleton({ className = "", ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`skeleton ${className}`} {...props} aria-hidden="true" />;
}

export function XpBar({ total, min, next, animate = false }: { total: number; min: number; next: number | null; animate?: boolean }) {
  const width = next === null ? 100 : Math.max(0, Math.min(100, ((total - min) / (next - min)) * 100));
  const fill = <motion.div className="xp-fill" initial={animate ? { width: "0%" } : false} animate={{ width: `${width}%` }} transition={{ duration: 0.8, ease: "easeOut" }} />;
  return <div className="xp-track" role="progressbar" aria-label="Rank progress" aria-valuemin={min} aria-valuemax={next ?? total} aria-valuenow={total}>{fill}</div>;
}

export function PageTitle({ children }: { children: ReactNode }) {
  return <h1 className="page-title">{children}</h1>;
}
