import { useMemo, type FC, type PropsWithChildren } from "react";
import {
  RuntimeAdapterProvider,
  unstable_useRemoteThreadListRuntime,
  type AssistantRuntime,
  type unstable_RemoteThreadListAdapter as RemoteThreadListAdapter,
} from "@assistant-ui/react";
import { useChatRuntime } from "@assistant-ui/react-ai-sdk";
import { AssistantStream, type AssistantStreamChunk } from "assistant-stream";
import { useLocalHistoryAdapter } from "./local-history-adapter";
import {
  getAllChatMetas,
  createChat,
  updateChatMeta,
  deleteChat,
  getChatMeta,
} from "./chat-storage";

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
    const id = crypto.randomUUID();
    await createChat(id);
    return { remoteId: id, externalId: undefined };
  }

  async generateTitle(): Promise<AssistantStream> {
    return new ReadableStream<AssistantStreamChunk>();
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
  transport: any;
}): AssistantRuntime {
  const localAdapter = useMemo(() => new LocalThreadListAdapter(), []);

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
