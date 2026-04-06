import React, { useState, useRef } from 'react'
import type { Message } from '../types'
import { EmojiPicker } from './EmojiPicker'

interface MessageInputProps {
  onSend: (text: string, replyToId?: string) => void
  disabled: boolean
  replyingTo?: Message
  onCancelReply: () => void
}

export function MessageInput({
  onSend,
  disabled,
  replyingTo,
  onCancelReply,
}: MessageInputProps): React.ReactElement {
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [emojiPickerOpen, setEmojiPickerOpen] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  async function handleSend() {
    const trimmed = text.trim()
    if (!trimmed || sending || disabled) return

    setSending(true)
    try {
      await onSend(trimmed, replyingTo?.id)
      setText('')
      onCancelReply()
      textareaRef.current?.focus()
    } finally {
      setSending(false)
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void handleSend()
    }
  }

  function handleEmojiSelect(emoji: string) {
    const textarea = textareaRef.current
    if (!textarea) {
      setText((prev) => prev + emoji)
      return
    }

    const start = textarea.selectionStart ?? text.length
    const end = textarea.selectionEnd ?? text.length
    const newText = text.slice(0, start) + emoji + text.slice(end)
    setText(newText)

    // Restore cursor position after the inserted emoji
    requestAnimationFrame(() => {
      textarea.focus()
      const newPos = start + emoji.length
      textarea.setSelectionRange(newPos, newPos)
    })
  }

  function toggleEmojiPicker() {
    setEmojiPickerOpen((prev) => {
      console.debug('[MessageInput] emoji picker open/close:', !prev)
      return !prev
    })
  }

  function closeEmojiPicker() {
    console.debug('[MessageInput] emoji picker open/close:', false)
    setEmojiPickerOpen(false)
  }

  const isDisabled = disabled || sending
  const replyPreview = replyingTo
    ? (replyingTo.text.length > 80 ? replyingTo.text.slice(0, 80) + '…' : replyingTo.text)
    : ''

  return (
    <div style={{ borderTop: '1px solid #e5e5e5' }}>
      {replyingTo && (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            padding: '6px 12px 4px',
            background: '#f5f5f5',
            borderLeft: '3px solid #2563eb',
            fontSize: 13,
          }}
        >
          <div style={{ overflow: 'hidden' }}>
            <span style={{ fontWeight: 600, color: '#2563eb', marginRight: 4 }}>
              ↩ {replyingTo.userName ?? 'User'}
            </span>
            <span style={{ color: '#6b7280', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', display: 'block' }}>
              {replyPreview}
            </span>
          </div>
          <button
            onClick={onCancelReply}
            title="Отменить ответ"
            style={{
              marginLeft: 8,
              flexShrink: 0,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: 16,
              color: '#9ca3af',
              padding: '0 2px',
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </div>
      )}
      <div
        style={{
          display: 'flex',
          gap: 8,
          padding: '8px 12px',
          alignItems: 'flex-end',
        }}
      >
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={isDisabled}
          placeholder="Введите сообщение..."
          rows={1}
          style={{
            flex: 1,
            resize: 'none',
            border: '1px solid #d0d0d0',
            borderRadius: 8,
            padding: '8px 10px',
            fontSize: 14,
            fontFamily: 'inherit',
            lineHeight: 1.4,
            outline: 'none',
            maxHeight: 120,
            overflowY: 'auto',
            opacity: isDisabled ? 0.6 : 1,
          }}
        />
        <div style={{ position: 'relative', flexShrink: 0 }}>
          <button
            onClick={toggleEmojiPicker}
            onMouseDown={(e) => e.stopPropagation()}
            disabled={isDisabled}
            title="Смайлики"
            style={{
              padding: '8px 10px',
              borderRadius: 8,
              border: '1px solid #d0d0d0',
              backgroundColor: emojiPickerOpen ? '#f0f0f0' : '#fff',
              cursor: isDisabled ? 'not-allowed' : 'pointer',
              fontSize: 18,
              height: 38,
              opacity: isDisabled ? 0.6 : 1,
              lineHeight: 1,
            }}
          >
            😊
          </button>
          {emojiPickerOpen && (
            <EmojiPicker onSelect={handleEmojiSelect} onClose={closeEmojiPicker} />
          )}
        </div>
        <button
          onClick={() => void handleSend()}
          disabled={isDisabled || !text.trim()}
          style={{
            padding: '8px 16px',
            borderRadius: 8,
            border: 'none',
            backgroundColor: isDisabled || !text.trim() ? '#ccc' : '#2563eb',
            color: '#fff',
            cursor: isDisabled || !text.trim() ? 'not-allowed' : 'pointer',
            fontSize: 14,
            fontWeight: 600,
            flexShrink: 0,
            height: 38,
          }}
        >
          {sending ? '...' : 'Send'}
        </button>
      </div>
    </div>
  )
}
