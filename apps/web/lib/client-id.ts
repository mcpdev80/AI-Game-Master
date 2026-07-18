let fallbackCounter = 0;

export function createClientId(prefix = "client"): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }

  fallbackCounter += 1;
  const randomPart = Math.random().toString(36).slice(2, 10);
  return `${prefix}-${Date.now().toString(36)}-${fallbackCounter.toString(36)}-${randomPart}`;
}
