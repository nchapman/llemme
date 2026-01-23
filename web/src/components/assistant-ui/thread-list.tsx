import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AssistantIf,
  ThreadListItemMorePrimitive,
  ThreadListItemPrimitive,
  ThreadListPrimitive,
  useThreadListItemRuntime,
} from "@assistant-ui/react";
import { MoreHorizontalIcon, SquarePenIcon, TrashIcon } from "lucide-react";
import { type FC, useCallback, useState } from "react";
import { deleteChat } from "@/lib/chat-storage";

export const ThreadList: FC = () => {
  return (
    <ThreadListPrimitive.Root className="aui-root aui-thread-list-root flex flex-col gap-1">
      <ThreadListNew />
      <div className="px-2 pt-4 pb-1">
        <h2 className="text-xs font-medium text-muted-foreground">Your Chats</h2>
      </div>
      <AssistantIf condition={({ threads }) => threads.isLoading}>
        <ThreadListSkeleton />
      </AssistantIf>
      <AssistantIf condition={({ threads }) => !threads.isLoading}>
        <ThreadListPrimitive.Items components={{ ThreadListItem }} />
      </AssistantIf>
    </ThreadListPrimitive.Root>
  );
};

const ThreadListNew: FC = () => {
  return (
    <ThreadListPrimitive.New asChild>
      <button className="flex h-9 items-center gap-2 rounded-lg px-3 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground">
        <SquarePenIcon className="size-4" />
        New Chat
      </button>
    </ThreadListPrimitive.New>
  );
};

const ThreadListSkeleton: FC = () => {
  return (
    <div className="flex flex-col gap-1">
      {Array.from({ length: 5 }, (_, i) => (
        <div
          key={i}
          role="status"
          aria-label="Loading threads"
          className="aui-thread-list-skeleton-wrapper flex h-9 items-center px-3"
        >
          <Skeleton className="aui-thread-list-skeleton h-4 w-full" />
        </div>
      ))}
    </div>
  );
};

const ThreadListItem: FC = () => {
  return (
    <ThreadListItemPrimitive.Root className="aui-thread-list-item group flex h-9 items-center gap-2 rounded-lg transition-colors hover:bg-muted focus-visible:bg-muted focus-visible:outline-none data-active:bg-muted">
      <ThreadListItemPrimitive.Trigger className="aui-thread-list-item-trigger flex h-full min-w-0 flex-1 items-center truncate px-3 text-start text-sm">
        <ThreadListItemPrimitive.Title fallback="New Chat" />
      </ThreadListItemPrimitive.Trigger>
      <ThreadListItemMore />
    </ThreadListItemPrimitive.Root>
  );
};

/**
 * WORKAROUND: Direct storage delete + page reload
 *
 * assistant-ui has a bug where threads created in the current session have
 * __LOCALID_xxx internal IDs that aren't properly tracked in its state lookup
 * table. Any operation that triggers a re-render causes components to try to
 * look up these IDs and crash with "tapLookupResources: Resource not found".
 *
 * The built-in ThreadListItemPrimitive.Delete triggers this bug. Our workaround:
 * 1. Delete directly from IndexedDB (bypasses assistant-ui's broken state)
 * 2. Reload the page to reset assistant-ui's internal state
 *
 * TODO: Remove this workaround when assistant-ui fixes the __LOCALID lookup bug.
 * Consider filing an issue at https://github.com/assistant-ui/assistant-ui/issues
 */
const ThreadListItemMore: FC = () => {
  const threadListItemRuntime = useThreadListItemRuntime();
  const [isDeleting, setIsDeleting] = useState(false);

  const handleDelete = useCallback(async () => {
    if (isDeleting) return;
    setIsDeleting(true);

    const state = threadListItemRuntime.getState();

    // Only delete from storage if thread was initialized (has remoteId)
    if (state.remoteId) {
      await deleteChat(state.remoteId);
    }

    // Reload to reset assistant-ui's broken internal state
    window.location.reload();
  }, [threadListItemRuntime, isDeleting]);

  return (
    <ThreadListItemMorePrimitive.Root>
      <ThreadListItemMorePrimitive.Trigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="aui-thread-list-item-more mr-2 size-7 p-0 opacity-0 transition-opacity group-hover:opacity-100 data-[state=open]:bg-accent data-[state=open]:opacity-100 group-data-active:opacity-100"
        >
          <MoreHorizontalIcon className="size-4" />
          <span className="sr-only">More options</span>
        </Button>
      </ThreadListItemMorePrimitive.Trigger>
      <ThreadListItemMorePrimitive.Content
        side="bottom"
        align="start"
        className="aui-thread-list-item-more-content z-50 min-w-32 overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
      >
        <ThreadListItemMorePrimitive.Item
          onClick={handleDelete}
          disabled={isDeleting}
          className="aui-thread-list-item-more-item flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive outline-none hover:bg-destructive/10 focus:bg-destructive/10 disabled:pointer-events-none disabled:opacity-50"
        >
          <TrashIcon className="size-4" />
          {isDeleting ? "Deleting..." : "Delete"}
        </ThreadListItemMorePrimitive.Item>
      </ThreadListItemMorePrimitive.Content>
    </ThreadListItemMorePrimitive.Root>
  );
};
