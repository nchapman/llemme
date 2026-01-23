import { useState } from "react";
import {
  type ThreadHistoryAdapter,
  type ExportedMessageRepository,
  type ExportedMessageRepositoryItem,
  type MessageFormatAdapter,
  type AssistantApi,
  useAssistantApi,
} from "@assistant-ui/react";
import {
  getChatMessages,
  setChatMessages,
  updateChatMeta,
  type MessageStorageEntry,
} from "./chat-storage";

type MessageFormatItem<TMessage> = {
  parentId: string | null;
  message: TMessage;
};

type MessageFormatRepository<TMessage> = {
  headId?: string | null;
  messages: MessageFormatItem<TMessage>[];
};

type GenericThreadHistoryAdapter<TMessage> = {
  load(): Promise<MessageFormatRepository<TMessage>>;
  append(item: MessageFormatItem<TMessage>): Promise<void>;
};

class FormattedLocalHistoryAdapter<TMessage, TStorageFormat>
  implements GenericThreadHistoryAdapter<TMessage>
{
  constructor(
    private parent: LocalHistoryAdapter,
    private formatAdapter: MessageFormatAdapter<TMessage, TStorageFormat>,
  ) {}

  async append(item: MessageFormatItem<TMessage>) {
    const encoded = this.formatAdapter.encode(item);
    const messageId = this.formatAdapter.getId(item.message);

    return this.parent._appendWithFormat(
      item.parentId,
      messageId,
      this.formatAdapter.format,
      encoded,
    );
  }

  async load(): Promise<MessageFormatRepository<TMessage>> {
    return this.parent._loadWithFormat(
      this.formatAdapter.format,
      (message: MessageStorageEntry<TStorageFormat>) =>
        this.formatAdapter.decode(message),
    );
  }
}

export class LocalHistoryAdapter implements ThreadHistoryAdapter {
  constructor(private store: AssistantApi) {}

  private get currentThreadId(): string | null {
    return this.store.threadListItem().getState().remoteId ?? null;
  }

  withFormat<TMessage, TStorageFormat>(
    formatAdapter: MessageFormatAdapter<TMessage, TStorageFormat>,
  ): GenericThreadHistoryAdapter<TMessage> {
    return new FormattedLocalHistoryAdapter(this, formatAdapter);
  }

  async append(_item: ExportedMessageRepositoryItem): Promise<void> {
    // This is called for the default format, but we use withFormat
  }

  async load(): Promise<ExportedMessageRepository> {
    const threadId = this.currentThreadId;
    if (!threadId) return { messages: [] };

    const stored = await getChatMessages(threadId);
    if (!stored) return { messages: [] };

    return { messages: [], headId: stored.headId ?? null };
  }

  async _appendWithFormat<T>(
    parentId: string | null,
    messageId: string,
    format: string,
    content: T,
  ): Promise<void> {
    const { remoteId } = await this.store.threadListItem().initialize();
    const threadId = remoteId;

    const stored = (await getChatMessages(threadId)) ?? { messages: [] };

    const existingIndex = stored.messages.findIndex((m) => m.id === messageId);

    const entry: MessageStorageEntry<T> = {
      id: messageId,
      parent_id: parentId,
      format,
      content,
    };

    if (existingIndex >= 0) {
      stored.messages[existingIndex] = entry;
    } else {
      stored.messages.push(entry);
    }

    stored.headId = messageId;

    await setChatMessages(threadId, stored);
    await updateChatMeta(threadId, { messageCount: stored.messages.length });
  }

  async _loadWithFormat<TMessage, TStorageFormat>(
    format: string,
    decoder: (
      message: MessageStorageEntry<TStorageFormat>,
    ) => MessageFormatItem<TMessage>,
  ): Promise<MessageFormatRepository<TMessage>> {
    const threadId = this.currentThreadId;
    if (!threadId) return { messages: [] };

    const stored = await getChatMessages(threadId);
    if (!stored) return { messages: [] };

    return {
      headId: stored.headId ?? null,
      messages: stored.messages
        .filter((m) => m.format === format)
        .map((m) => decoder(m as MessageStorageEntry<TStorageFormat>)),
    };
  }
}

export const useLocalHistoryAdapter = (): ThreadHistoryAdapter => {
  const store = useAssistantApi();
  const [adapter] = useState(() => new LocalHistoryAdapter(store));
  return adapter;
};
