import React, { useCallback, useState } from 'react'
import type { ApiClient } from '../api/client'
import type { Message, EditedMessage, WidgetConfig } from '../types'
import { useChat } from '../hooks/useChat'
import { useLongPoll } from '../hooks/useLongPoll'
import { useOnline } from '../hooks/useOnline'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'

interface ChatWindowProps {
  config: WidgetConfig
  api: ApiClient
}

export function ChatWindow({ config, api }: ChatWindowProps): React.ReactElement {
  const { chat, messages, initialHasMore, loading, error, sendMessage } = useChat(
    api,
    config.eventId,
    config.roomId,
  )

  const [pollError, setPollError] = useState<string | null>(null)
  const [allMessages, setAllMessages] = useState<Message[]>([])
  const [initialized, setInitialized] = useState(false)
  const [replyingTo, setReplyingTo] = useState<Message | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)

  // Merge initial messages from useChat into allMessages once loaded
  React.useEffect(() => {
    if (!loading && !initialized) {
      setAllMessages(messages)
      setHasMore(initialHasMore)
      setInitialized(true)

      if (import.meta.env.DEV && chat) {
        console.debug('[ChatWindow] mounted', 'chat_id', chat.id, 'has_more', initialHasMore)
      }
    }
  }, [loading, initialized, messages, initialHasMore, chat])

  const loadOlderMessages = useCallback(async () => {
    if (loadingMore || !hasMore || !chat) return
    const oldestSeq = allMessages[0]?.seq ?? 0
    if (oldestSeq <= 0) return
    setLoadingMore(true)
    try {
      const result = await api.getMessages(chat.id, 50, oldestSeq)
      console.debug('[ChatWindow] load older', oldestSeq, 'has_more', result.has_more)
      setAllMessages((prev) => {
        const existingIds = new Set(prev.map((m) => m.id))
        const fresh = result.messages.filter((m) => !existingIds.has(m.id))
        return [...fresh, ...prev]
      })
      setHasMore(result.has_more)
    } catch (err) {
      console.warn('[ChatWindow] loadOlderMessages failed', err)
    } finally {
      setLoadingMore(false)
    }
  }, [api, chat, allMessages, hasMore, loadingMore])

  const handlePollMessages = useCallback((incoming: Message[], deletedIds: string[], editedMessages: EditedMessage[]) => {
    setPollError(null)
    setAllMessages((prev) => {
      let next = prev
      if (deletedIds.length > 0) {
        const deletedSet = new Set(deletedIds)
        next = next.filter((m) => !deletedSet.has(m.id))
      }
      if (editedMessages.length > 0) {
        const editMap = new Map(editedMessages.map((e) => [e.id, e]))
        next = next.map((m) => {
          const edit = editMap.get(m.id)
          if (!edit) return m
          return { ...m, text: edit.text, editedAt: edit.edited_at }
        })
      }
      if (incoming.length > 0) {
        const existingIds = new Set(next.map((m) => m.id))
        const fresh = incoming.filter((m) => !existingIds.has(m.id))
        if (fresh.length > 0) next = [...next, ...fresh]
      }
      return next
    })
  }, [])

  const handlePollError = useCallback((err: unknown) => {
    const msg = err instanceof Error ? err.message : String(err)
    console.warn('[ChatWindow] poll error', msg)
    setPollError(msg)
  }, [])

  // Start polling from the highest seq already loaded to avoid receiving stale messages out of order.
  const initialSeq = messages.reduce((max, m) => Math.max(max, m.seq), 0)
  useLongPoll(api, initialized ? (chat?.id ?? null) : null, initialSeq, handlePollMessages, handlePollError)
  const onlineCount = useOnline(api, chat?.id ?? null)

  const currentUserId = api.getCurrentUserId()
  const currentUserRole = api.getCurrentUserRole()

  const handleEdit = useCallback(
    async (msgId: string, newText: string) => {
      try {
        const updated = await api.editMessage(msgId, newText)
        setAllMessages((prev) =>
          prev.map((m) =>
            m.id === msgId ? { ...m, text: updated.text, editedAt: updated.edited_at } : m,
          ),
        )
      } catch (err) {
        console.warn('[ChatWindow] edit failed', err)
      }
    },
    [api],
  )

  const handleDelete = useCallback(
    async (msgId: string) => {
      try {
        await api.deleteMessage(msgId)
        setAllMessages((prev) => prev.filter((m) => m.id !== msgId))
      } catch (err) {
        console.warn('[ChatWindow] delete failed', err)
      }
    },
    [api],
  )

  const handleBan = useCallback(
    async (userId: string, reason: string) => {
      try {
        await api.banUser(userId, config.eventId, reason)
      } catch (err) {
        console.warn('[ChatWindow] ban failed', err)
      }
    },
    [api, config.eventId],
  )

  const handleMute = useCallback(
    async (userId: string, reason: string) => {
      if (!chat) return
      try {
        await api.muteUser(userId, chat.id, reason)
      } catch (err) {
        console.warn('[ChatWindow] mute failed', err)
      }
    },
    [api, chat],
  )

  const handleSend = useCallback(
    async (text: string, replyToId?: string) => {
      if (!chat) return

      // Optimistic message — show immediately with reply preview if replying
      const optimisticId = `opt-${Date.now()}`
      const optimistic: Message = {
        id: optimisticId,
        chatId: chat.id,
        userId: currentUserId,
        userName: api.getCurrentUserName(),
        text,
        seq: -1,
        createdAt: new Date().toISOString(),
        replyToId: replyingTo?.id,
        replyToSeq: replyingTo?.seq,
        replyToText: replyingTo?.text,
        replyToUserName: replyingTo?.userName,
      }
      setAllMessages((prev) => [...prev, optimistic])
      setReplyingTo(null)

      try {
        const response = await sendMessage(text, replyToId)
        // Replace optimistic entry with confirmed data
        setAllMessages((prev) =>
          prev.map((m) =>
            m.id === optimisticId
              ? { ...m, id: response.id, seq: response.seq, createdAt: response.ts, userId: currentUserId }
              : m,
          ),
        )
      } catch (err) {
        // Remove optimistic message on failure
        setAllMessages((prev) => prev.filter((m) => m.id !== optimisticId))
        throw err
      }
    },
    [chat, sendMessage, replyingTo],
  )

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
        backgroundColor: '#fff',
        border: '1px solid #e5e5e5',
        borderRadius: 12,
        overflow: 'hidden',
      }}
    >
      {/* Header: room name + online count */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          borderBottom: '1px solid #e5e5e5',
          backgroundColor: '#fafafa',
          minHeight: 44,
        }}
      >
        <span
          style={{
            fontWeight: 600,
            fontSize: 14,
            color: '#111',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {chat
            ? (chat.externalRoomId ? `#${chat.externalRoomId}` : 'Chat')
            : (loading ? '' : 'Chat')}
        </span>
        {chat && (
          <span
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 5,
              fontSize: 12,
              color: '#555',
              flexShrink: 0,
            }}
          >
            <span
              style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                backgroundColor: onlineCount > 0 ? '#22c55e' : '#d1d5db',
                display: 'inline-block',
              }}
            />
            {onlineCount} online
          </span>
        )}
      </div>

      {pollError && (
        <div
          style={{
            backgroundColor: '#fef2f2',
            color: '#b91c1c',
            fontSize: 13,
            padding: '6px 12px',
            textAlign: 'center',
            borderBottom: '1px solid #fecaca',
          }}
        >
          Connection error, retrying...
        </div>
      )}

      {error && !loading && (
        <div
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#b91c1c',
            fontSize: 14,
            padding: 20,
          }}
        >
          {error}
        </div>
      )}

      {!error && (
        <>
          <MessageList
            messages={initialized ? allMessages : messages}
            loading={loading}
            currentUserId={currentUserId}
            currentUserRole={currentUserRole}
            onDelete={handleDelete}
            onEdit={handleEdit}
            onBan={handleBan}
            onMute={handleMute}
            onReply={setReplyingTo}
            onLoadMore={hasMore ? loadOlderMessages : undefined}
            loadingMore={loadingMore}
          />
          <MessageInput
            onSend={handleSend}
            disabled={loading || !chat}
            replyingTo={replyingTo ?? undefined}
            onCancelReply={() => setReplyingTo(null)}
          />
        </>
      )}
    </div>
  )
}
