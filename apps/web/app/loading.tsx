import { Card, Skeleton } from "@/components/ui";

export default function Loading() {
  return <main className="shell page-wrap"><Card><Skeleton style={{ width: "45%", height: 34 }} /><Skeleton style={{ width: "75%", marginTop: 22 }} /><Skeleton style={{ width: "100%", marginTop: 12, height: 120 }} /></Card></main>;
}
