import useSWR from "swr";

// In dev mode, use the configured API URL
// In production (embedded), use current origin (for URL() constructor compatibility)
export const LLEME_BASE_URL =
  import.meta.env.VITE_LLEME_API_URL || window.location.origin;

export interface LlemeModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
  lleme?: {
    status: string;
    port: number;
    last_activity: string;
    loaded_at: string;
  };
}

interface ModelsResponse {
  object: string;
  data: LlemeModel[];
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export function useModels() {
  const { data, error, isLoading } = useSWR<ModelsResponse>(
    `${LLEME_BASE_URL}/v1/models`,
    fetcher,
  );

  return {
    models: data?.data ?? [],
    isLoading,
    error,
  };
}
