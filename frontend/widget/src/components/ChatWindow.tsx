import React, { useCallback, useState } from 'react'
import type { ApiClient } from '../api/client'
import type { Message, WidgetConfig } from '../types'
import { useChat } from '../hooks/useChat'
import { useLongPoll } from '../hooks/useLongPoll'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'

interface ChatWindowProps {
  config: WidgetConfig
  api: ApiClient
}

export function ChatWindow({ config, api }: ChatWindowProps): React.ReactElement {
  const { chat, messages, loading, error, sendMessage } = useChat(
    api,
    config.eventId,
    config.roomId,
  )

  const [pollError, setPollError] = useState<string | null>(null)
  const [allMessages, setAllMessages] = useState<Message[]>([])
  const [initialized, setInitialized] = useState(false)

  // Merge initial messages from useChat into allMessages once loaded
  React.useEffect(() => {
    if (!loading && !initialized) {
      setAllMessages(messages)
      setInitialized(true)

      if (import.meta.env.DEV && chat) {
        console.debug('[ChatWindow] mounted', 'chat_id', chat.id)
      }
    }
  }, [loading, initialized, messages, chat])

  const handlePollMessages = useCallback((incoming: Message[]) => {
    setPollError(null)
    setAllMessages((prev) => {
      const existingIds = new Set(prev.map((m) => m.id))
      const fresh = incoming.filter((m) => !existingIds.has(m.id))
      return fresh.length > 0 ? [...prev, ...fresh] : prev
    })
  }, [])

  const handlePollError = useCallback((err: unknown) => {
    const msg = err instanceof Error ? err.message : String(err)
    console.warn('[ChatWindow] poll error', msg)
    setPollError(msg)
  }, [])

  useLongPoll(api, chat?.id ?? null, handlePollMessages, handlePollError)

  const handleSend = useCallback(
    async (text: string) => {
      await sendMessage(text)
    },
    [sendMessage],
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
          <MessageList messages={initialized ? allMessages : messages} loading={loading} />
          <MessageInput onSend={handleSend} disabled={loading || !chat} />
        </>
      )}
    </div>
  )
}
