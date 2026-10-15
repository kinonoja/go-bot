import Link from "next/link";
import { Button } from "@/components/ui";

export default function NotFound() {
  return <main className="shell not-found"><span className="eyebrow">404 · Lost in the gopherspace</span><h1 className="page-title">We couldn’t find that lesson.</h1><p className="muted">The link may have moved, or the lesson may not exist yet.</p><Link href="/courses"><Button>Browse courses</Button></Link></main>;
}
