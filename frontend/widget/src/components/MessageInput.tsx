import React, { useState, useRef } from 'react'

interface MessageInputProps {
  onSend: (text: string) => void
  disabled: boolean
}

export function MessageInput({ onSend, disabled }: MessageInputProps): React.ReactElement {
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  async function handleSend() {
    const trimmed = text.trim()
    if (!trimmed || sending || disabled) return

    setSending(true)
    try {
      await onSend(trimmed)
      setText('')
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

  const isDisabled = disabled || sending

  return (
    <div
      style={{
        display: 'flex',
        gap: 8,
        padding: '8px 12px',
        borderTop: '1px solid #e5e5e5',
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
  )
}
