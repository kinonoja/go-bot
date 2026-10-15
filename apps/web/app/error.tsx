"use client";

import { Button } from "@/components/ui";

export default function ErrorPage({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return <main className="shell error-page"><span className="eyebrow">A small snag</span><h1 className="page-title">We couldn’t load this page.</h1><p className="muted">The response didn’t match what Go Bot expects, or the API is taking a break.</p><Button onClick={reset}>Try again</Button></main>;
}
