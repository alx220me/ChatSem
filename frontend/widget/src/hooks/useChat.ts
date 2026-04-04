import { useState, useEffect, useCallback } from 'react'
import type { ApiClient } from '../api/client'
import type { Chat, Message } from '../types'

interface UseChatResult {
  chat: Chat | null
  messages: Message[]
  loading: boolean
  error: string | null
  sendMessage: (text: string) => Promise<void>
}

export function useChat(
  api: ApiClient,
  eventId: string,
  roomId?: string,
): UseChatResult {
  const [chat, setChat] = useState<Chat | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function init() {
      try {
        setLoading(true)
        setError(null)

        const chats = await api.listChats(eventId)

        if (cancelled) return

        // Select the chat matching the roomId, or fall back to the parent chat
        let selected: Chat | undefined
        if (roomId) {
          await api.joinRoom(eventId, roomId)
          selected = chats.find(
            (c) => c.externalRoomId === roomId || c.type === 'child',
          )
        }
        if (!selected) {
          selected = chats.find((c) => c.type === 'parent') ?? chats[0]
        }

        if (!selected) {
          setError('No chat found for this event')
          return
        }

        if (cancelled) return
        setChat(selected)

        const msgs = await api.getMessages(selected.id, 50)
        if (cancelled) return
        setMessages(msgs)
      } catch (err) {
        if (cancelled) return
        const msg = err instanceof Error ? err.message : String(err)
        setError(msg)
        console.warn('[useChat] init error', msg)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void init()

    return () => {
      cancelled = true
    }
  }, [api, eventId, roomId])

  const sendMessage = useCallback(
    async (text: string) => {
      if (!chat) return

      // Optimistic message with seq=-1
      const optimisticId = `optimistic-${Date.now()}`
      const optimistic: Message = {
        id: optimisticId,
        chatId: chat.id,
        userId: '',
        text,
        seq: -1,
        createdAt: new Date().toISOString(),
      }

      setMessages((prev) => [...prev, optimistic])

      try {
        const response = await api.sendMessage(chat.id, text)
        setMessages((prev) =>
          prev.map((m) =>
            m.id === optimisticId
              ? { ...m, id: response.id, seq: response.seq, createdAt: response.ts }
              : m,
          ),
        )
      } catch (err) {
        // Remove optimistic message on error
        setMessages((prev) => prev.filter((m) => m.id !== optimisticId))
        const msg = err instanceof Error ? err.message : String(err)
        console.warn('[useChat] sendMessage error', msg)
        throw err
      }
    },
    [api, chat],
  )

  return { chat, messages, loading, error, sendMessage }
}
