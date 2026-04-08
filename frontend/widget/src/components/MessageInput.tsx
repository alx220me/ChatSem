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
  const [emojiAnchorRect, setEmojiAnchorRect] = useState<DOMRect | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const emojiButtonRef = useRef<HTMLButtonElement>(null)

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
    if (!emojiPickerOpen && emojiButtonRef.current) {
      setEmojiAnchorRect(emojiButtonRef.current.getBoundingClientRect())
    }
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
      <div style={{ padding: '8px 12px' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            border: '1px solid #d0d0d0',
            borderRadius: 8,
            backgroundColor: isDisabled ? '#f9fafb' : '#fff',
            opacity: isDisabled ? 0.7 : 1,
            overflow: 'hidden',
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
              border: 'none',
              outline: 'none',
              background: 'transparent',
              padding: '8px 10px',
              fontSize: 14,
              fontFamily: 'inherit',
              lineHeight: 1.4,
              maxHeight: 120,
              overflowY: 'auto',
            }}
          />
          <div style={{ display: 'flex', alignItems: 'flex-end', flexShrink: 0, padding: '4px 4px' }}>
            <button
              ref={emojiButtonRef}
              onClick={toggleEmojiPicker}
              onMouseDown={(e) => e.stopPropagation()}
              disabled={isDisabled}
              title="Смайлики"
              style={{
                width: 30,
                height: 30,
                border: 'none',
                background: 'none',
                cursor: isDisabled ? 'not-allowed' : 'pointer',
                fontSize: 17,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: 0,
                borderRadius: 6,
                opacity: isDisabled ? 0.5 : 1,
                backgroundColor: emojiPickerOpen ? '#f0f0f0' : 'transparent',
              }}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true" stroke="currentColor" strokeWidth="2" color={isDisabled ? '#d1d5db' : emojiPickerOpen ? '#374151' : '#9ca3af'}>
                <circle cx="12" cy="12" r="10" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M8 14s1.5 2 4 2 4-2 4-2" strokeLinecap="round"/>
                <circle cx="9" cy="10" r="1" fill="currentColor" stroke="none"/>
                <circle cx="15" cy="10" r="1" fill="currentColor" stroke="none"/>
              </svg>
            </button>
            {emojiPickerOpen && emojiAnchorRect && (
              <EmojiPicker onSelect={handleEmojiSelect} onClose={closeEmojiPicker} anchorRect={emojiAnchorRect} />
            )}
            <button
              onClick={() => void handleSend()}
              disabled={isDisabled || !text.trim()}
              aria-label="Отправить"
              title="Отправить"
              style={{
                width: 30,
                height: 30,
                border: 'none',
                background: 'none',
                color: isDisabled || !text.trim() ? '#d1d5db' : '#374151',
                cursor: isDisabled || !text.trim() ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: 0,
                borderRadius: 6,
                opacity: sending ? 0.5 : 1,
                transition: 'color 0.15s',
              }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <g transform="rotate(45, 12, 12)">
                  <path d="M22 2L11 13" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  <path d="M22 2L15 22L11 13L2 9L22 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </g>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
