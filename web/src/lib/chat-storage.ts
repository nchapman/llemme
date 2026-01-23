import { get, set, del } from "idb-keyval";

export interface ChatMeta {
  id: string;
  title?: string;
  model?: string;
  createdAt: number;
  updatedAt: number;
  messageCount: number;
  isArchived?: boolean;
}

export interface MessageStorageEntry<TPayload = unknown> {
  id: string;
  parent_id: string | null;
  format: string;
  content: TPayload;
}

export interface StoredChat<TPayload = unknown> {
  headId?: string | null;
  messages: MessageStorageEntry<TPayload>[];
}

const CHATS_KEY = "chats";

function chatMetaKey(id: string): string {
  return `chat:${id}:meta`;
}

function chatMessagesKey(id: string): string {
  return `chat:${id}:messages`;
}

export async function getChatIds(): Promise<string[]> {
  return (await get<string[]>(CHATS_KEY)) ?? [];
}

export async function setChatIds(ids: string[]): Promise<void> {
  await set(CHATS_KEY, ids);
}

export async function getChatMeta(id: string): Promise<ChatMeta | undefined> {
  return get<ChatMeta>(chatMetaKey(id));
}

export async function setChatMeta(meta: ChatMeta): Promise<void> {
  await set(chatMetaKey(meta.id), meta);
}

export async function getChatMessages(
  id: string,
): Promise<StoredChat | undefined> {
  return get<StoredChat>(chatMessagesKey(id));
}

export async function setChatMessages(
  id: string,
  chat: StoredChat,
): Promise<void> {
  await set(chatMessagesKey(id), chat);
}

export async function deleteChat(id: string): Promise<void> {
  const ids = await getChatIds();
  await setChatIds(ids.filter((i) => i !== id));
  await del(chatMetaKey(id));
  await del(chatMessagesKey(id));
}

export async function getAllChatMetas(): Promise<ChatMeta[]> {
  const ids = await getChatIds();
  const metas = await Promise.all(ids.map((id) => getChatMeta(id)));
  return metas.filter((m): m is ChatMeta => m !== undefined);
}

export async function createChat(
  id: string,
  meta?: Partial<Omit<ChatMeta, "id">>,
): Promise<ChatMeta> {
  const now = Date.now();
  const chatMeta: ChatMeta = {
    id,
    createdAt: now,
    updatedAt: now,
    messageCount: 0,
    ...meta,
  };

  const ids = await getChatIds();
  if (!ids.includes(id)) {
    await setChatIds([id, ...ids]);
  }
  await setChatMeta(chatMeta);
  await setChatMessages(id, { messages: [] });

  return chatMeta;
}

export async function updateChatMeta(
  id: string,
  updates: Partial<Omit<ChatMeta, "id">>,
): Promise<ChatMeta | undefined> {
  const meta = await getChatMeta(id);
  if (!meta) return undefined;

  const updated: ChatMeta = {
    ...meta,
    ...updates,
    updatedAt: Date.now(),
  };
  await setChatMeta(updated);
  return updated;
}

export async function saveChat(
  id: string,
  chat: StoredChat,
  metaUpdates?: Partial<Omit<ChatMeta, "id">>,
): Promise<void> {
  await setChatMessages(id, chat);
  await updateChatMeta(id, {
    messageCount: chat.messages.length,
    ...metaUpdates,
  });
}
