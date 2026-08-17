import { proxyAgentRequest } from "@/lib/agent-proxy";

type RouteContext = { params: Promise<{ path: string[] }> };

async function handle(request: Request, context: RouteContext) {
  const { path } = await context.params;
  return proxyAgentRequest(request, path);
}

export const GET = handle;
export const POST = handle;
