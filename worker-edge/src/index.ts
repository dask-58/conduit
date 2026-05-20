export interface Env {
  ORIGIN_URL: string;
  KV: KVNamespace;
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

    const headers = new Headers(request.headers);
    const loggedHeaders = Object.fromEntries(headers.entries());
    console.log("forwarding inbound webhook", {
      method: request.method,
      url: request.url,
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
