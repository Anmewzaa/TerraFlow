import { useCallback, useEffect, useState } from "react";
import type { Category, RegisteredProvider } from "../lib/provider";

/** The cloud providers an admin registered, and the palette categories (both are data from the server). */
export function useProviders() {
  const [providers, setProviders] = useState<RegisteredProvider[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    try {
      const [p, m] = await Promise.all([fetch("/api/catalog/providers"), fetch("/api/catalog/meta")]);
      setProviders(p.ok ? ((await p.json()) as RegisteredProvider[]) : []);
      setCategories(m.ok ? (((await m.json()) as { categories: Category[] }).categories ?? []) : []);
    } catch {
      setProviders([]);
      setCategories([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  return { providers, categories, loading, reload };
}
