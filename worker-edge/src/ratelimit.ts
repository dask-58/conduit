const LIMIT = 100;
const WINDOW_TTL_SECONDS = 60;

export async function checkRateLimit(kv: KVNamespace, tenantId: string): Promise<boolean> {
  const key = `ratelimit:${tenantId}`;
  const raw = await kv.get(key);
  const count = Number.parseInt(raw ?? "0", 10);

  if (count >= LIMIT) {
    return false;
  }

  await kv.put(key, String(count + 1), { expirationTtl: WINDOW_TTL_SECONDS });
  return true;
}
