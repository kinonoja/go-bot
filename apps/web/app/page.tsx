import Link from "next/link";
import { Button } from "@/components/ui";

export default function HomePage() {
  return <main className="shell hero">
    <span className="eyebrow">A hands-on Go learning path</span>
    <h1>Go Bot</h1>
    <p>Learn Go by writing real code, solving focused challenges, and watching your skills grow one lesson at a time.</p>
    <Link href="/courses"><Button>Explore the courses <span aria-hidden="true">→</span></Button></Link>
  </main>;
}
