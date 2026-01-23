import {
  type ThreadHistoryAdapter,
  type ExportedMessageRepository,
  type ExportedMessageRepositoryItem,
  type MessageFormatAdapter,
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
  // Thread ID will be set by the runtime
  private currentThreadId: string | null = null;

  setThreadId(threadId: string | null) {
    this.currentThreadId = threadId;
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
    if (!this.currentThreadId) return { messages: [] };

    const stored = await getChatMessages(this.currentThreadId);
    if (!stored) return { messages: [] };

    return { messages: [], headId: stored.headId };
  }

  async _appendWithFormat<T>(
    parentId: string | null,
    messageId: string,
    format: string,
    content: T,
  ): Promise<void> {
    if (!this.currentThreadId) {
      // Generate a thread ID if we don't have one
      this.currentThreadId = crypto.randomUUID();
    }

    const threadId = this.currentThreadId;
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
    if (!this.currentThreadId) return { messages: [] };

    const stored = await getChatMessages(this.currentThreadId);
    if (!stored) return { messages: [] };

    return {
      headId: stored.headId,
      messages: stored.messages
        .filter((m) => m.format === format)
        .map((m) => decoder(m as MessageStorageEntry<TStorageFormat>)),
    };
  }
}
