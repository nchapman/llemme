import useSWR from "swr";

// Use current origin when embedded, allowing the app to work from any host
export const LLEME_BASE_URL =
  typeof window !== "undefined" ? window.location.origin : "";

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
    fetcher
  );

  return {
    models: data?.data ?? [],
    isLoading,
    error,
  };
}
