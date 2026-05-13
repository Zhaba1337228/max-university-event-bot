// Typed fetch wrapper для admin REST API.
//
// Все запросы идут через Next.js rewrite в /api/* → бэкенд на :8081.
// Это позволяет cookie session_jwt оставаться same-origin (SameSite=Strict),
// никакие CORS headers не нужны.

export type ApiError = {
  code: string;
  message: string;
};

export class HttpError extends Error {
  status: number;
  body: ApiError | null;

  constructor(status: number, body: ApiError | null, msg?: string) {
    super(msg || (body?.message ?? `HTTP ${status}`));
    this.status = status;
    this.body = body;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
  };
  let payload: BodyInit | undefined;
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }
  const res = await fetch(path, {
    method,
    headers,
    body: payload,
    credentials: "include",
    cache: "no-store",
  });
  if (res.status === 204) {
    return undefined as unknown as T;
  }
  const text = await res.text();
  const data = text ? safeJSON(text) : null;
  if (!res.ok) {
    throw new HttpError(res.status, (data as ApiError) ?? null);
  }
  return data as T;
}

function safeJSON(s: string): unknown {
  try {
    return JSON.parse(s);
  } catch {
    return null;
  }
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body ?? {}),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  delete: <T>(path: string) => request<T>("DELETE", path),
};
