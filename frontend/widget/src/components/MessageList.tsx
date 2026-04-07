import React, { useEffect, useRef, useState } from 'react'
import type { Message } from '../types'
import { UserAvatar } from './UserAvatar'

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
}

interface ModTarget {
  userId: string
  userName: string
  action: 'ban' | 'mute'
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
}: MessageListProps): React.ReactElement {
  const bottomRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const [hoveredId, setHoveredId] = useState<string | null>(null)
  const [modTarget, setModTarget] = useState<ModTarget | null>(null)
  const [modReason, setModReason] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editText, setEditText] = useState('')

  const isMod = currentUserRole === 'moderator' || currentUserRole === 'admin'

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  function handleModConfirm() {
    if (!modTarget) return
    const reason = modReason.trim() || 'Нарушение правил'
    if (modTarget.action === 'ban') onBan?.(modTarget.userId, reason)
    else onMute?.(modTarget.userId, reason)
    setModTarget(null)
    setModReason('')
  }

  async function handleEditSave(msgId: string) {
    const trimmed = editText.trim()
    if (!trimmed || !onEdit) {
      setEditingId(null)
      return
    }
    if (import.meta.env.DEV) {
      console.debug('[MessageList] edit save', msgId, trimmed)
    }
    await onEdit(msgId, trimmed)
    setEditingId(null)
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
    <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: '8px 0', position: 'relative' }}>
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
                <span>{new Date(msg.createdAt).toLocaleTimeString()}</span>
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
                  <span style={{ fontWeight: 600, color: '#2563eb', marginRight: 4 }}>
                    {msg.replyToUserName || 'User'}
                  </span>
                  <span style={{ color: '#6b7280' }}>
                    {msg.replyToText
                      ? (msg.replyToText.length > 80 ? msg.replyToText.slice(0, 80) + '…' : msg.replyToText)
                      : '…'}
                  </span>
                </div>
              )}

              {isEditing ? (
                <textarea
                  autoFocus
                  value={editText}
                  onChange={(e) => setEditText(e.target.value)}
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
