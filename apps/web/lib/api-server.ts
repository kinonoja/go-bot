import { cookies } from "next/headers";
import { proxyHeaders } from "@/lib/proxy";

export async function apiFetch(path: string): Promise<Response> {
  const cookieStore = await cookies();
  const headers = proxyHeaders(null, process.env.DEV_AUTH === "true", cookieStore.get("dev_email")?.value);
  return fetch(`${process.env.API_URL || "http://localhost:8081"}/api/${path.replace(/^\//, "")}`, { headers, cache: "no-store" });
}
