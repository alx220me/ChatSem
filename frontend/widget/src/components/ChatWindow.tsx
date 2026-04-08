import React, { useCallback, useEffect, useRef, useState } from 'react'
import { HttpError } from '../api/client'
import type { ApiClient } from '../api/client'
import type { Message, EditedMessage, WidgetConfig } from '../types'
import { useChat } from '../hooks/useChat'
import { useLongPoll } from '../hooks/useLongPoll'
import { useOnline } from '../hooks/useOnline'
import { useDrag } from '../hooks/useDrag'
import { useResize } from '../hooks/useResize'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'
import { Toast } from './Toast'
import type { ToastState } from './Toast'

interface ChatWindowProps {
  config: WidgetConfig
  api: ApiClient
}

export function ChatWindow({ config, api }: ChatWindowProps): React.ReactElement {
  const { chat, messages, initialHasMore, loading, error, sendMessage } = useChat(
    api,
    config.eventId,
    config.roomId,
    config.roomName,
  )

  const [isBanned, setIsBanned] = useState(false)
  const [pollError, setPollError] = useState<string | null>(null)
  const [allMessages, setAllMessages] = useState<Message[]>([])
  const [initialized, setInitialized] = useState(false)
  const [replyingTo, setReplyingTo] = useState<Message | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [toast, setToast] = useState<ToastState | null>(null)
  const [scrollToBottomTrigger, setScrollToBottomTrigger] = useState(0)
  const allMessagesRef = useRef(allMessages)
  useEffect(() => { allMessagesRef.current = allMessages }, [allMessages])

  // Floating widget state — only active when config.floating === true
  const [collapsed, setCollapsed] = useState(config.defaultCollapsed ?? false)
  const { pos, isDragging, dragHandlers } = useDrag(config.defaultPosition)
  const { size, isResizing, getHandlers: getResizeHandlers } = useResize(
    config.defaultSize ?? { w: 360, h: 520 },
  )

  const handleExpand = useCallback(() => {
    if (!isDragging) {
      setCollapsed(false)
      if (import.meta.env.DEV) {
        console.debug('[ChatWindow] floating expanded')
      }
    }
  }, [isDragging])

  const handleCollapse = useCallback(() => {
    setCollapsed(true)
    if (import.meta.env.DEV) {
      console.debug('[ChatWindow] floating collapsed')
    }
  }, [])

  // Sync banned state from useChat's initial load error
  React.useEffect(() => {
    if (error === 'banned') setIsBanned(true)
  }, [error])

  const handleBanned = useCallback(() => {
    setIsBanned(true)
    setToast({ message: 'Вы заблокированы в этом мероприятии', variant: 'error' })
  }, [])

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
    const oldestSeq = allMessagesRef.current[0]?.seq ?? 0
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
  }, [api, chat, hasMore, loadingMore])

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

  // Passed to useLongPoll — only active while not banned
  const activeChatId = initialized && !isBanned ? (chat?.id ?? null) : null

  // Start polling from the highest seq already loaded to avoid receiving stale messages out of order.
  const initialSeq = messages.reduce((max, m) => Math.max(max, m.seq), 0)
  useLongPoll(api, activeChatId, initialSeq, handlePollMessages, handlePollError, handleBanned)
  const onlineCount = useOnline(api, chat?.id ?? null, !isBanned)

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
        setToast({ message: 'Пользователь заблокирован', variant: 'success' })
      } catch (err) {
        console.warn('[ChatWindow] ban failed', err)
        setToast({ message: 'Не удалось заблокировать', variant: 'error' })
      }
    },
    [api, config.eventId],
  )

  const handleMute = useCallback(
    async (userId: string, reason: string) => {
      if (!chat) return
      try {
        await api.muteUser(userId, chat.id, reason)
        setToast({ message: 'Пользователь замьючен', variant: 'success' })
      } catch (err) {
        console.warn('[ChatWindow] mute failed', err)
        setToast({ message: 'Не удалось замьютить', variant: 'error' })
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
      setScrollToBottomTrigger((n) => n + 1)
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
        if (err instanceof HttpError && err.code === 'muted') {
          setToast({ message: 'Вы замьючены в этом чате', variant: 'error' })
          return
        }
        if (err instanceof HttpError && (err.status === 403 || err.code === 'banned')) {
          handleBanned()
          return
        }
        throw err
      }
    },
    [chat, sendMessage, replyingTo],
  )

  const accent = config.accentColor ?? '#2563eb'

  // Room title priority: server-stored name > config.roomName > externalRoomId > 'Chat'
  const roomTitle = chat?.externalRoomName
    ?? config.roomName
    ?? (chat?.externalRoomId ? `#${chat.externalRoomId}` : null)
    ?? (chat ? 'Chat' : (loading ? '' : 'Chat'))

  if (import.meta.env.DEV && chat) {
    console.debug('[ChatWindow] room title resolved:', roomTitle,
      'from server:', chat.externalRoomName, 'from config:', config.roomName)
  }
  const accentText = config.accentTextColor ?? '#ffffff'
  // Derive a semi-transparent version of accentText for secondary labels and borders.
  const accentTextMuted = accentText === '#ffffff' || accentText === 'white'
    ? 'rgba(255,255,255,0.85)'
    : 'rgba(0,0,0,0.55)'
  const accentTextSubtle = accentText === '#ffffff' || accentText === 'white'
    ? 'rgba(255,255,255,0.4)'
    : 'rgba(0,0,0,0.2)'
  const accentBorder = accentText === '#ffffff' || accentText === 'white'
    ? 'rgba(255,255,255,0.35)'
    : 'rgba(0,0,0,0.2)'
  const accentBtnBg = accentText === '#ffffff' || accentText === 'white'
    ? 'rgba(255,255,255,0.15)'
    : 'rgba(0,0,0,0.08)'

  // ─── Floating: collapsed FAB ─────────────────────────────────────────────
  // FAB is always anchored to bottom-right — no drag, no dynamic position.
  if (config.floating && collapsed) {
    return (
      <div
        role="button"
        aria-label="Открыть чат"
        data-testid="chat-fab"
        onClick={handleExpand}
        style={{
          position: 'fixed',
          right: 20,
          bottom: 20,
          height: 36,
          borderRadius: 18,
          backgroundColor: accent,
          boxShadow: '0 4px 12px rgba(0,0,0,0.25)',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '0 12px 0 10px',
          zIndex: 9999,
          userSelect: 'none',
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
        }}
      >
        {/* Chat bubble icon */}
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true"
          style={{ flexShrink: 0 }}>
          <path
            d="M20 2H4C2.9 2 2 2.9 2 4V22L6 18H20C21.1 18 22 17.1 22 16V4C22 2.9 21.1 2 20 2Z"
            fill={accentText}
          />
        </svg>

        {/* Label */}
        <span style={{ color: accentText, fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap' }}>
          Чат
        </span>

        {/* Online indicator */}
        {onlineCount > 0 && (
          <span style={{
            display: 'flex', alignItems: 'center', gap: 3,
            color: accentTextMuted, fontSize: 11, whiteSpace: 'nowrap',
          }}>
            <span style={{
              width: 6, height: 6, borderRadius: '50%',
              backgroundColor: '#4ade80', flexShrink: 0,
            }} />
            {onlineCount > 99 ? '99+' : onlineCount}
          </span>
        )}
      </div>
    )
  }

  // ─── Shared inner content (used in both floating-expanded and embedded) ──
  const innerContent = (
    <>
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

      {(isBanned || (error && error !== 'banned' && !loading)) && (
        <div
          style={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
            padding: 24,
            textAlign: 'center',
          }}
        >
          {isBanned ? (
            <>
              <span style={{ fontSize: 32 }}>🚫</span>
              <span style={{ fontWeight: 600, fontSize: 15, color: '#111827' }}>
                Доступ ограничен
              </span>
              <span style={{ fontSize: 13, color: '#6b7280' }}>
                Вы заблокированы в этом мероприятии
              </span>
            </>
          ) : (
            <span style={{ color: '#b91c1c', fontSize: 14 }}>{error}</span>
          )}
        </div>
      )}

      {!isBanned && !error && (
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
            scrollToBottomTrigger={scrollToBottomTrigger}
          />
          <MessageInput
            onSend={handleSend}
            disabled={loading || !chat}
            replyingTo={replyingTo ?? undefined}
            onCancelReply={() => setReplyingTo(null)}
          />
        </>
      )}
    </>
  )

  // ─── Floating: expanded window ───────────────────────────────────────────
  if (config.floating) {
    // Default: open above the FAB (right-aligned, bottom = FAB height + gap)
    const windowPos: React.CSSProperties = pos
      ? { left: pos.x, top: pos.y }
      : { right: 20, bottom: 72 }

    return (
      <div
        data-testid="chat-window-floating"
        style={{
          position: 'fixed',
          ...windowPos,
          width: size.w,
          height: size.h,
          display: 'flex',
          flexDirection: 'column',
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
          backgroundColor: '#fff',
          border: '1px solid #e5e5e5',
          borderRadius: 12,
          overflow: 'hidden',
          boxShadow: '0 8px 24px rgba(0,0,0,0.15)',
          zIndex: 9999,
          userSelect: isDragging || isResizing ? 'none' : undefined,
        }}
      >
        {/* Drag handle header */}
        <div
          {...dragHandlers}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 14px',
            borderBottom: '1px solid rgba(0,0,0,0.1)',
            backgroundColor: accent,
            minHeight: 44,
            cursor: isDragging ? 'grabbing' : 'grab',
            touchAction: 'none',
          }}
        >
          <span
            style={{
              fontWeight: 600,
              fontSize: 14,
              color: accentText,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {roomTitle}
          </span>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
            {chat && (
              <span
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 5,
                  fontSize: 12,
                  color: accentTextMuted,
                }}
              >
                <span
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: '50%',
                    backgroundColor: onlineCount > 0 ? '#4ade80' : accentTextSubtle,
                    display: 'inline-block',
                  }}
                />
                {onlineCount} online
              </span>
            )}

            {/* Collapse button — stopPropagation prevents header drag from swallowing the click */}
            <button
              aria-label="Свернуть чат"
              title="Свернуть"
              data-testid="chat-collapse-btn"
              onPointerDown={(e) => e.stopPropagation()}
              onClick={handleCollapse}
              style={{
                width: 28,
                height: 28,
                borderRadius: 6,
                border: `1px solid ${accentBorder}`,
                backgroundColor: accentBtnBg,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: accentText,
                flexShrink: 0,
                padding: 0,
              }}
            >
              {/* Chevron-down icon */}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M6 9L12 15L18 9" stroke="currentColor" strokeWidth="2.5"
                  strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          </div>
        </div>

        {innerContent}

        {/* ── Resize handles ─────────────────────────────────────────── */}
        {/* Right edge */}
        <div
          {...getResizeHandlers('e')}
          style={{
            position: 'absolute', top: 12, right: 0, bottom: 12, width: 6,
            cursor: 'e-resize', zIndex: 1,
          }}
        />
        {/* Bottom edge */}
        <div
          {...getResizeHandlers('s')}
          style={{
            position: 'absolute', left: 12, right: 12, bottom: 0, height: 6,
            cursor: 's-resize', zIndex: 1,
          }}
        />
        {/* Bottom-right corner */}
        <div
          {...getResizeHandlers('se')}
          style={{
            position: 'absolute', right: 0, bottom: 0, width: 16, height: 16,
            cursor: 'se-resize', zIndex: 2,
          }}
        />
        {/* Bottom-left corner */}
        <div
          {...getResizeHandlers('sw')}
          style={{
            position: 'absolute', left: 0, bottom: 0, width: 16, height: 16,
            cursor: 'sw-resize', zIndex: 2,
          }}
        />
        {/* Left edge */}
        <div
          {...getResizeHandlers('w')}
          style={{
            position: 'absolute', top: 12, left: 0, bottom: 12, width: 6,
            cursor: 'w-resize', zIndex: 1,
          }}
        />

        <Toast state={toast} onDismiss={() => setToast(null)} />
      </div>
    )
  }

  // ─── Embedded (default): fills the host-page container ───────────────────
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
          borderBottom: '1px solid rgba(0,0,0,0.1)',
          backgroundColor: accent,
          minHeight: 44,
        }}
      >
        <span
          style={{
            fontWeight: 600,
            fontSize: 14,
            color: accentText,
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
              color: accentTextMuted,
              flexShrink: 0,
            }}
          >
            <span
              style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                backgroundColor: onlineCount > 0 ? '#4ade80' : accentTextSubtle,
                display: 'inline-block',
              }}
            />
            {onlineCount} online
          </span>
        )}
      </div>

      {innerContent}
      <Toast state={toast} onDismiss={() => setToast(null)} />
    </div>
  )
}
