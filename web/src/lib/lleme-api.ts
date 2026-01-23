import useSWR from "swr";

// In dev mode, Vite proxies /v1 to localhost:11313
// In production (embedded), requests go to the same origin
// Either way, we can use relative URLs
export const LLEME_BASE_URL = "";

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
