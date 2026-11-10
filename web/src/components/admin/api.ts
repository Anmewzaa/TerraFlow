/** The admin screens' one way to call the API: never throws, always says what happened. */
export interface Result<T> {
  ok: boolean;
  status: number;
  data: T | null;
  error: string;
}

export async function call<T>(path: string, init?: Omit<RequestInit, "body"> & { json?: unknown }): Promise<Result<T>> {
  const { json, ...rest } = init ?? {};
  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      ...rest,
      headers: json !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: json !== undefined ? JSON.stringify(json) : undefined,
    });
  } catch {
    return { ok: false, status: 0, data: null, error: "Cannot reach the server." };
  }
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    /* empty body */
  }
  const error = (data as { error?: string } | null)?.error ?? (res.ok ? "" : `Request failed (${res.status})`);
  return { ok: res.ok, status: res.status, data: data as T | null, error };
}
