import { NextRequest, NextResponse } from "next/server";
import { proxyHeaders } from "@/lib/proxy";

async function forward(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  const destination = new URL(`/api/${path.map(encodeURIComponent).join("/")}`, process.env.API_URL || "http://localhost:8081");
  destination.search = request.nextUrl.search;
  const response = await fetch(destination, {
    method: request.method,
    headers: proxyHeaders(request.headers.get("content-type"), process.env.DEV_AUTH === "true", request.cookies.get("dev_email")?.value),
    body: request.method === "POST" ? await request.text() : undefined,
    cache: "no-store",
  });
  return new NextResponse(response.body, { status: response.status, headers: { "content-type": response.headers.get("content-type") || "application/json" } });
}

export const GET = forward;
export const POST = forward;
