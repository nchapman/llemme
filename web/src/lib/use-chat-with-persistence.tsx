import { useRef, useMemo, type FC, type PropsWithChildren } from "react";
import {
  RuntimeAdapterProvider,
  unstable_useRemoteThreadListRuntime,
  type AssistantRuntime,
  type ThreadMessage,
  type unstable_RemoteThreadListAdapter as RemoteThreadListAdapter,
} from "@assistant-ui/react";
import { useChatRuntime } from "@assistant-ui/react-ai-sdk";
import { createAssistantStream } from "assistant-stream";
import { streamText } from "ai";
import { useLocalHistoryAdapter } from "./local-history-adapter";
import {
  getAllChatMetas,
  createChat,
  updateChatMeta,
  deleteChat,
  getChatMeta,
} from "./chat-storage";
import { createLlemeProvider } from "./lleme-transport";

// crypto.randomUUID() requires a secure context (HTTPS), but this app runs on local networks without SSL.
function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).substring(2);
}

type RemoteThreadMetadata = {
  readonly status: "regular" | "archived";
  readonly remoteId: string;
  readonly externalId?: string | undefined;
  readonly title?: string | undefined;
};

type RemoteThreadListResponse = {
  threads: RemoteThreadMetadata[];
};

type RemoteThreadInitializeResponse = {
  remoteId: string;
  externalId: string | undefined;
};

class LocalThreadListAdapter implements RemoteThreadListAdapter {
  constructor(private getModel: () => string) {}

  async list(): Promise<RemoteThreadListResponse> {
    const metas = await getAllChatMetas();
    return {
      threads: metas.map((meta) => ({
        status: meta.isArchived ? ("archived" as const) : ("regular" as const),
        remoteId: meta.id,
        title: meta.title,
        externalId: undefined,
      })),
    };
  }

  async rename(remoteId: string, newTitle: string): Promise<void> {
    await updateChatMeta(remoteId, { title: newTitle });
  }

  async archive(remoteId: string): Promise<void> {
    await updateChatMeta(remoteId, { isArchived: true });
  }

  async unarchive(remoteId: string): Promise<void> {
    await updateChatMeta(remoteId, { isArchived: false });
  }

  async delete(remoteId: string): Promise<void> {
    await deleteChat(remoteId);
  }

  async initialize(_threadId: string): Promise<RemoteThreadInitializeResponse> {
    const id = generateId();
    await createChat(id);
    return { remoteId: id, externalId: undefined };
  }

  async generateTitle(
    remoteId: string,
    messages: readonly ThreadMessage[],
  ) {
    // Extract first user message text for fallback
    const firstUserMessage = messages.find((m) => m.role === "user");
    const firstMessageText = firstUserMessage?.content
      .filter((part): part is { type: "text"; text: string } => part.type === "text")
      .map((part) => part.text)
      .join(" ");
    const fallbackTitle = firstMessageText?.slice(0, 50).trim() || "New conversation";

    try {
      const provider = createLlemeProvider();

      // Extract text content from messages for title generation
      const conversationText = messages
        .map((msg) => {
          const role = msg.role === "user" ? "User" : "Assistant";
          const text = msg.content
            .filter((part): part is { type: "text"; text: string } => part.type === "text")
            .map((part) => part.text)
            .join(" ");
          return `${role}: ${text}`;
        })
        .join("\n");

      const { textStream } = streamText({
        model: provider(this.getModel()),
        system:
          "Generate a short title (3-6 words) for this conversation. Reply with only the title, no quotes or punctuation.",
        prompt: conversationText,
      });

      return createAssistantStream(async (controller) => {
        let title = "";
        for await (const chunk of textStream) {
          title += chunk;
          controller.appendText(chunk);
        }
        // Persist the title to storage
        await updateChatMeta(remoteId, { title: title.trim() });
      });
    } catch (error) {
      console.error("Title generation failed, using fallback:", error);
      return createAssistantStream(async (controller) => {
        controller.appendText(fallbackTitle);
        // Persist the fallback title
        await updateChatMeta(remoteId, { title: fallbackTitle });
      });
    }
  }

  async fetch(threadId: string): Promise<RemoteThreadMetadata> {
    const meta = await getChatMeta(threadId);
    if (!meta) {
      throw new Error("Chat not found");
    }
    return {
      status: meta.isArchived ? "archived" : "regular",
      remoteId: meta.id,
      title: meta.title,
      externalId: undefined,
    };
  }

  unstable_Provider = LocalAdapterProvider;
}

const LocalAdapterProvider: FC<PropsWithChildren> = ({ children }) => {
  const historyAdapter = useLocalHistoryAdapter();

  const adapters = useMemo(
    () => ({
      history: historyAdapter,
    }),
    [historyAdapter],
  );

  return (
    <RuntimeAdapterProvider adapters={adapters}>
      {children}
    </RuntimeAdapterProvider>
  );
};

export function useChatWithPersistence(options: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  transport: any;
  model: string;
}): AssistantRuntime {
  // Use a ref to always get the current model without recreating the adapter
  const modelRef = useRef(options.model);
  modelRef.current = options.model;

  const localAdapter = useMemo(
    () => new LocalThreadListAdapter(() => modelRef.current),
    [],
  );

  return unstable_useRemoteThreadListRuntime({
    runtimeHook: function RuntimeHook() {
      return useChatRuntime({
        transport: options.transport,
      });
    },
    adapter: localAdapter,
    allowNesting: true,
  });
}
