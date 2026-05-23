import { checkRateLimit } from "./ratelimit";
import { verifyGitHubSignature } from "./verify";

export interface Env {
  ORIGIN_URL: string;
  GITHUB_SECRET: string;
  ORIGIN_API_KEY: string;
  RATELIMIT: KVNamespace;
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    if (request.method !== "POST") {
      return new Response("method not allowed", {
        status: 405,
        headers: {
          allow: "POST"
        }
      });
    }

    const validSignature = await verifyGitHubSignature(request, env.GITHUB_SECRET);
    if (!validSignature) {
      return new Response("unauthorized", { status: 401 });
    }

    const tenantId = getTenantId(request);
    const allowed = await checkRateLimit(env.RATELIMIT, tenantId);
    if (!allowed) {
      return new Response("too many requests", { status: 429 });
    }

    const headers = new Headers(request.headers);
    headers.delete("Authorization");
    headers.set("Authorization", `Bearer ${env.ORIGIN_API_KEY}`);
    const loggedHeaders = Object.fromEntries(headers.entries());
    if ("authorization" in loggedHeaders) {
      loggedHeaders.authorization = "[redacted]";
    }
    console.log("forwarding inbound webhook", {
      method: request.method,
      url: request.url,
      tenantId,
      headers: loggedHeaders
    });

    const originURL = new URL(env.ORIGIN_URL);
    originURL.pathname = joinPaths(originURL.pathname, "/ingest");

    return fetch(originURL.toString(), {
      method: "POST",
      headers,
      body: request.body,
      redirect: "manual"
    });
  }
};

function joinPaths(basePath: string, childPath: string): string {
  const base = basePath.replace(/\/+$/, "");
  const child = childPath.replace(/^\/+/, "");
  return `${base}/${child}`;
}

function getTenantId(request: Request): string {
  const installationTarget = request.headers.get("X-GitHub-Hook-Installation-Target-ID");
  if (installationTarget) {
    return installationTarget;
  }

  const event = request.headers.get("X-GitHub-Event");
  if (event) {
    return `event:${event}`;
  }

  const connectingIP = request.headers.get("CF-Connecting-IP");
  if (connectingIP) {
    return `ip:${connectingIP}`;
  }

  return "anonymous";
}
