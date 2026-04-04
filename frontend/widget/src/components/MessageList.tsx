import React, { useEffect, useRef } from 'react'
import type { Message } from '../types'
import { UserAvatar } from './UserAvatar'

interface MessageListProps {
  messages: Message[]
  loading: boolean
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

export function MessageList({ messages, loading }: MessageListProps): React.ReactElement {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

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
    <div style={{ flex: 1, overflowY: 'auto', padding: '8px 0' }}>
      {messages.map((msg) => (
        <div
          key={msg.id}
          style={{
            display: 'flex',
            gap: 8,
            padding: '6px 12px',
            alignItems: 'flex-start',
            opacity: msg.seq === -1 ? 0.6 : 1,
          }}
        >
          <UserAvatar name={msg.userId} size="md" />
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
              <span style={{ fontWeight: 600, color: '#333' }}>{msg.userId}</span>
              <span>{new Date(msg.createdAt).toLocaleTimeString()}</span>
            </div>
            <div
              style={{
                fontSize: 14,
                color: '#1a1a1a',
                wordBreak: 'break-word',
                whiteSpace: 'pre-wrap',
              }}
            >
              {msg.text}
            </div>
          </div>
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
