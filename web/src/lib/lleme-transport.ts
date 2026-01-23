import { createOpenAICompatible } from "@ai-sdk/openai-compatible";
import { ToolLoopAgent, DirectChatTransport } from "ai";
import { LLEME_BASE_URL } from "./lleme-api";
const DEFAULT_MODEL = "default";

export interface LlemeConfig {
  baseUrl?: string;
  model?: string;
}

/**
 * Creates a lleme provider using @ai-sdk/openai-compatible.
 */
export function createLlemeProvider(baseUrl: string = LLEME_BASE_URL) {
  return createOpenAICompatible({
    name: "lleme",
    baseURL: `${baseUrl}/v1`,
  });
}

/**
 * Creates a transport for connecting to lleme's OpenAI-compatible API.
 * Uses DirectChatTransport with a ToolLoopAgent for proper client-side streaming.
 */
export function createLlemeTransport(config: LlemeConfig = {}) {
  const baseUrl = config.baseUrl ?? LLEME_BASE_URL;
  const model = config.model ?? DEFAULT_MODEL;

  const provider = createLlemeProvider(baseUrl);

  const agent = new ToolLoopAgent({
    model: provider(model),
  });

  return new DirectChatTransport({
    agent,
  });
}
