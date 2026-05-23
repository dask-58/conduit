const encoder = new TextEncoder();

export async function verifyGitHubSignature(request: Request, secret: string): Promise<boolean> {
  const signature = request.headers.get("X-Hub-Signature-256");
  if (!signature || !signature.startsWith("sha256=") || secret === "") {
    return false;
  }

  const body = await request.clone().arrayBuffer();
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"]
  );

  const digest = await crypto.subtle.sign("HMAC", key, body);
  const expected = `sha256=${toHex(digest)}`;

  return timingSafeEqual(signature, expected);
}

function toHex(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let hex = "";
  for (const byte of bytes) {
    hex += byte.toString(16).padStart(2, "0");
  }

  return hex;
}

function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) {
    return false;
  }

  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }

  return diff === 0;
}
