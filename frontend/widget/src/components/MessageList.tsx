import React, { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { Message } from '../types'
import { UserAvatar } from './UserAvatar'

/** Distance from bottom (px) within which auto-scroll activates on new messages. */
const NEAR_BOTTOM_THRESHOLD = 150

interface MessageListProps {
  messages: Message[]
  loading: boolean
  currentUserId?: string
  currentUserRole?: string
  onDelete?: (msgId: string) => void
  onBan?: (userId: string, reason: string) => void
  onMute?: (userId: string, reason: string) => void
  onReply?: (msg: Message) => void
  onEdit?: (msgId: string, newText: string) => Promise<void>
  onLoadMore?: () => void
  loadingMore?: boolean
  /** Increment to force-scroll to bottom (e.g. when user sends a message). */
  scrollToBottomTrigger?: number
}

interface ModTarget {
  userId: string
  userName: string
  action: 'ban' | 'mute'
}

function formatMessageTime(isoString: string): string {
  const date = new Date(isoString)
  const now = new Date()
  const isToday =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()

  const time = date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  if (isToday) return time

  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  const isYesterday =
    date.getFullYear() === yesterday.getFullYear() &&
    date.getMonth() === yesterday.getMonth() &&
    date.getDate() === yesterday.getDate()

  if (isYesterday) return `вчера, ${time}`

  const dateStr = date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
  return `${dateStr}, ${time}`
}

function SkeletonRow(): React.ReactElement {
  return (
    <div
      style={{
        display: 'flex',
        gap: 8,
        padding: '8px 12px',
        alignItems: 'flex-start',
      }}
    >
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: '50%',
          backgroundColor: '#e0e0e0',
          flexShrink: 0,
          animation: 'chatsem-pulse 1.4s ease-in-out infinite',
        }}
      />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
        <div
          style={{
            height: 12,
            width: '30%',
            backgroundColor: '#e0e0e0',
            borderRadius: 4,
            animation: 'chatsem-pulse 1.4s ease-in-out infinite',
          }}
        />
        <div
          style={{
            height: 12,
            width: '70%',
            backgroundColor: '#e0e0e0',
            borderRadius: 4,
            animation: 'chatsem-pulse 1.4s ease-in-out infinite',
          }}
        />
      </div>
    </div>
  )
}

export function MessageList({
  messages,
  loading,
  currentUserId,
  currentUserRole,
  onDelete,
  onBan,
  onMute,
  onReply,
  onEdit,
  onLoadMore,
  loadingMore,
  scrollToBottomTrigger,
}: MessageListProps): React.ReactElement {
  const bottomRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const topSentinelRef = useRef<HTMLDivElement>(null)
  const loadingTriggeredRef = useRef(false)
  const prevFirstMsgIdRef = useRef<string | undefined>(undefined)
  const savedScrollHeightRef = useRef(0)
  const isNearBottomRef = useRef(true)
  const [hoveredId, setHoveredId] = useState<string | null>(null)
  const [modTarget, setModTarget] = useState<ModTarget | null>(null)
  const [modReason, setModReason] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editText, setEditText] = useState('')

  const isMod = currentUserRole === 'moderator' || currentUserRole === 'admin'

  // Scroll management: preserve position on prepend, auto-scroll on append only when near bottom
  useLayoutEffect(() => {
    const list = listRef.current
    if (!list) return

    const firstMsgId = messages[0]?.id

    if (prevFirstMsgIdRef.current !== undefined && firstMsgId !== prevFirstMsgIdRef.current) {
      // First message changed — prepend happened, preserve scroll position
      const delta = list.scrollHeight - savedScrollHeightRef.current
      if (delta > 0) {
        list.scrollTop += delta
      }
    } else if (isNearBottomRef.current) {
      // New message appended or initial load — scroll to bottom synchronously (before paint)
      // so the user never sees the list at scrollTop=0. Smooth animation is avoided here
      // because it defers the actual scroll to animation frames, causing a visible flash.
      list.scrollTop = list.scrollHeight
    }

    prevFirstMsgIdRef.current = firstMsgId
    savedScrollHeightRef.current = list.scrollHeight
    // NOTE: isNearBottomRef is kept up-to-date by the onScroll handler below,
    // not here — smooth scrollIntoView doesn't update scrollTop synchronously,
    // so computing it here would give a stale position and break auto-scroll.
  }, [messages])

  // Track whether user is near the bottom — updated on every manual scroll
  function handleListScroll() {
    const list = listRef.current
    if (!list) return
    isNearBottomRef.current = list.scrollHeight - list.scrollTop - list.clientHeight < NEAR_BOTTOM_THRESHOLD
  }

  // IntersectionObserver on top sentinel to trigger loadOlderMessages
  useEffect(() => {
    const sentinel = topSentinelRef.current
    if (!sentinel || !onLoadMore) return

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0]
        if (entry.isIntersecting && !loadingTriggeredRef.current) {
          loadingTriggeredRef.current = true
          console.debug('[MessageList] top sentinel visible — load older')
          onLoadMore()
        }
      },
      { threshold: 0.1 },
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [onLoadMore])

  // Reset trigger when loading finishes so next scroll-to-top triggers again
  useEffect(() => {
    if (!loadingMore) {
      loadingTriggeredRef.current = false
    }
  }, [loadingMore])

  // Force-scroll to bottom when user sends a message (regardless of current scroll position)
  useEffect(() => {
    if (!scrollToBottomTrigger) return
    const list = listRef.current
    if (!list) return
    isNearBottomRef.current = true
    list.scrollTop = list.scrollHeight
  }, [scrollToBottomTrigger])

  function handleModConfirm() {
    if (!modTarget) return
    const reason = modReason.trim() || 'Нарушение правил'
    if (modTarget.action === 'ban') onBan?.(modTarget.userId, reason)
    else onMute?.(modTarget.userId, reason)
    setModTarget(null)
    setModReason('')
  }

  const editSavingRef = React.useRef(false)

  async function handleEditSave(msgId: string) {
    const trimmed = editText.trim()
    if (!trimmed || !onEdit) {
      setEditingId(null)
      return
    }
    if (import.meta.env.DEV) {
      console.debug('[MessageList] edit save', msgId, trimmed)
    }
    editSavingRef.current = true
    try {
      await onEdit(msgId, trimmed)
    } finally {
      editSavingRef.current = false
      setEditingId(null)
    }
  }

  function scrollToSeq(seq: number) {
    const el = listRef.current?.querySelector(`[data-seq="${seq}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }

  if (loading) {
    return (
      <div style={{ flex: 1, overflowY: 'auto' }}>
        <style>{`
          @keyframes chatsem-pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.4; }
          }
        `}</style>
        {Array.from({ length: 5 }).map((_, i) => (
          <SkeletonRow key={i} />
        ))}
      </div>
    )
  }

  return (
    <div ref={listRef} onScroll={handleListScroll} style={{ flex: 1, overflowY: 'auto', padding: '8px 0', position: 'relative' }}>
      {/* Top sentinel for IntersectionObserver — triggers loadOlderMessages when visible */}
      <div ref={topSentinelRef} style={{ height: 1 }} />

      {/* Spinner shown while older messages are loading */}
      {loadingMore && (
        <div
          style={{
            textAlign: 'center',
            padding: '6px 0',
            fontSize: 12,
            color: '#9ca3af',
          }}
        >
          Загрузка...
        </div>
      )}

      {messages.map((msg) => {
        const isOwn = msg.userId === currentUserId
        const canDelete = (isOwn || isMod) && msg.seq !== -1
        const canEdit = isOwn && msg.seq !== -1 && !!onEdit
        const canBan = isMod && !isOwn && msg.seq !== -1
        const showActions = hoveredId === msg.id && (canDelete || canEdit || canBan || !!onReply)
        const isEditing = editingId === msg.id

        return (
          <div
            key={msg.id}
            data-seq={msg.seq}
            style={{
              display: 'flex',
              gap: 8,
              padding: '6px 12px',
              alignItems: 'flex-start',
              opacity: msg.seq === -1 ? 0.6 : 1,
              position: 'relative',
            }}
            onMouseEnter={() => setHoveredId(msg.id)}
            onMouseLeave={() => setHoveredId(null)}
          >
            <UserAvatar name={msg.userName ?? msg.userId} size="md" />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  fontSize: 12,
                  color: '#888',
                  marginBottom: 2,
                  display: 'flex',
                  gap: 8,
                }}
              >
                <span style={{ fontWeight: 600, color: '#333' }}>{msg.userName ?? msg.userId}</span>
                <span>{formatMessageTime(msg.createdAt)}</span>
              </div>

              {/* Reply quote block */}
              {msg.replyToId && (
                <div
                  onClick={() => msg.replyToSeq != null && scrollToSeq(msg.replyToSeq)}
                  title="Перейти к оригинальному сообщению"
                  style={{
                    cursor: msg.replyToSeq != null ? 'pointer' : 'default',
                    marginBottom: 4,
                    padding: '4px 8px',
                    background: '#f0f4ff',
                    borderLeft: '3px solid #2563eb',
                    borderRadius: '0 4px 4px 0',
                    fontSize: 12,
                    color: '#4b5563',
                    maxWidth: '100%',
                    overflow: 'hidden',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
                    <span style={{ fontWeight: 600, color: '#2563eb' }}>
                      {msg.replyToUserName || 'User'}
                    </span>
                    {msg.replyToCreatedAt && (
                      <span style={{ fontSize: 10, color: '#9ca3af' }}>
                        {formatMessageTime(msg.replyToCreatedAt)}
                      </span>
                    )}
                  </div>
                  <span style={{ color: '#6b7280' }}>
                    {msg.replyToText
                      ? (msg.replyToText.length > 80 ? msg.replyToText.slice(0, 80) + '…' : msg.replyToText)
                      : '…'}
                  </span>
                </div>
              )}

              {isEditing ? (
                <div>
                  <textarea
                    autoFocus
                    value={editText}
                    onChange={(e) => setEditText(e.target.value)}
                    onBlur={() => { if (!editSavingRef.current) setEditingId(null) }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault()
                        void handleEditSave(msg.id)
                      }
                      if (e.key === 'Escape') {
                        setEditingId(null)
                      }
                    }}
                    style={{
                      width: '100%',
                      fontSize: 14,
                      padding: '4px 6px',
                      border: '1px solid #2563eb',
                      borderRadius: 4,
                      resize: 'vertical',
                      fontFamily: 'inherit',
                      boxSizing: 'border-box',
                      minHeight: 60,
                    }}
                  />
                  <div style={{ fontSize: 11, color: '#9ca3af', marginTop: 2 }}>
                    Enter — сохранить · Esc — отмена
                  </div>
                </div>
              ) : (
                <div
                  style={{
                    fontSize: 14,
                    color: '#1a1a1a',
                    wordBreak: 'break-word',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {msg.text}
                  {msg.editedAt && (
                    <span style={{ fontSize: 11, color: '#9ca3af', marginLeft: 6 }}>(изм.)</span>
                  )}
                </div>
              )}
            </div>

            {/* Action buttons on hover */}
            {showActions && (
              <div
                style={{
                  position: 'absolute',
                  top: 4,
                  right: 12,
                  display: 'flex',
                  gap: 4,
                  backgroundColor: '#fff',
                  border: '1px solid #e5e5e5',
                  borderRadius: 6,
                  padding: '2px 4px',
                  boxShadow: '0 1px 4px rgba(0,0,0,0.12)',
                }}
              >
                {onReply && msg.seq !== -1 && (
                  <button
                    title="Ответить"
                    onClick={() => onReply(msg)}
                    style={actionBtnStyle}
                  >
                    ↩
                  </button>
                )}
                {canEdit && (
                  <button
                    title="Редактировать сообщение"
                    onClick={() => {
                      setEditingId(msg.id)
                      setEditText(msg.text)
                    }}
                    style={actionBtnStyle}
                  >
                    ✏️
                  </button>
                )}
                {canDelete && (
                  <button
                    title="Удалить сообщение"
                    onClick={() => onDelete?.(msg.id)}
                    style={actionBtnStyle}
                  >
                    🗑
                  </button>
                )}
                {canBan && (
                  <button
                    title="Замутить в чате"
                    onClick={() =>
                      setModTarget({ userId: msg.userId, userName: msg.userName ?? msg.userId, action: 'mute' })
                    }
                    style={{ ...actionBtnStyle, color: '#d97706' }}
                  >
                    🔇
                  </button>
                )}
                {canBan && (
                  <button
                    title="Забанить в событии"
                    onClick={() =>
                      setModTarget({ userId: msg.userId, userName: msg.userName ?? msg.userId, action: 'ban' })
                    }
                    style={{ ...actionBtnStyle, color: '#dc2626' }}
                  >
                    🚫
                  </button>
                )}
              </div>
            )}
          </div>
        )
      })}
      <div ref={bottomRef} />

      {/* Ban / Mute confirmation dialog */}
      {modTarget && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.35)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 10,
          }}
          onClick={() => { setModTarget(null); setModReason('') }}
        >
          <div
            style={{
              backgroundColor: '#fff',
              borderRadius: 8,
              padding: 16,
              width: 260,
              boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 8 }}>
              {modTarget.action === 'ban' ? '🚫 Забанить' : '🔇 Замутить'} {modTarget.userName}?
            </div>
            <div style={{ fontSize: 12, color: '#6b7280', marginBottom: 8 }}>
              {modTarget.action === 'ban' ? 'Запрет на всё событие' : 'Запрет в этом чате'}
            </div>
            <input
              autoFocus
              placeholder="Причина (необязательно)"
              value={modReason}
              onChange={(e) => setModReason(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleModConfirm()
                if (e.key === 'Escape') { setModTarget(null); setModReason('') }
              }}
              style={{
                width: '100%',
                padding: '6px 8px',
                border: '1px solid #d1d5db',
                borderRadius: 4,
                fontSize: 13,
                boxSizing: 'border-box',
                marginBottom: 10,
              }}
            />
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button
                onClick={() => { setModTarget(null); setModReason('') }}
                style={cancelBtnStyle}
              >
                Отмена
              </button>
              <button onClick={handleModConfirm} style={confirmBtnStyle}>
                {modTarget.action === 'ban' ? 'Забанить' : 'Замутить'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

const actionBtnStyle: React.CSSProperties = {
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  fontSize: 14,
  padding: '2px 3px',
  lineHeight: 1,
  borderRadius: 3,
  color: '#555',
}

const cancelBtnStyle: React.CSSProperties = {
  padding: '5px 12px',
  border: '1px solid #d1d5db',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  backgroundColor: '#fff',
}

const confirmBtnStyle: React.CSSProperties = {
  padding: '5px 12px',
  border: 'none',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 13,
  backgroundColor: '#dc2626',
  color: '#fff',
}
