import { useState, useEffect, useCallback } from 'react'
import type { ApiClient } from '../api/client'
import type { Chat, Message, SendResponse } from '../types'

interface UseChatResult {
  chat: Chat | null
  messages: Message[]
  loading: boolean
  error: string | null
  sendMessage: (text: string) => Promise<SendResponse>
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

        // If roomId given — join first so the child chat exists, then list
        if (roomId) {
          await api.joinRoom(eventId, roomId)
          if (cancelled) return
        }

        const chats = await api.listChats(eventId)

        if (cancelled) return

        // Select the chat matching the roomId, or fall back to the parent chat
        let selected: Chat | undefined
        if (roomId) {
          selected = chats.find((c) => c.externalRoomId === roomId)
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
    async (text: string): Promise<SendResponse> => {
      if (!chat) throw new Error('no active chat')
      try {
        return await api.sendMessage(chat.id, text)
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        console.warn('[useChat] sendMessage error', msg)
        throw err
      }
    },
    [api, chat],
  )

  return { chat, messages, loading, error, sendMessage }
}
