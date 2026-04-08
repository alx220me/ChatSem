import { useState, useEffect, useCallback } from 'react'
import type { ApiClient } from '../api/client'
import { HttpError } from '../api/client'
import type { Chat, Message, SendResponse } from '../types'

interface UseChatResult {
  chat: Chat | null
  messages: Message[]
  initialHasMore: boolean
  loading: boolean
  error: string | null
  sendMessage: (text: string, replyToId?: string) => Promise<SendResponse>
}

export function useChat(
  api: ApiClient,
  eventId: string,
  roomId?: string,
  roomName?: string,
): UseChatResult {
  const [chat, setChat] = useState<Chat | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [initialHasMore, setInitialHasMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function init() {
      try {
        setLoading(true)
        setError(null)

        let selected: Chat | undefined

        if (roomId) {
          // joinRoom creates/returns the child chat directly — no need to listChats
          selected = await api.joinRoom(eventId, roomId, roomName)
          if (cancelled) return
        } else {
          // No roomId — list chats and pick the parent
          const chats = await api.listChats(eventId)
          if (cancelled) return
          selected = chats.find((c) => c.type === 'parent') ?? chats[0]
        }

        if (!selected) {
          setError('No chat found for this event')
          return
        }

        if (cancelled) return
        setChat(selected)

        const result = await api.getMessages(selected.id, 50)
        if (cancelled) return
        setMessages(result.messages)
        setInitialHasMore(result.has_more)
      } catch (err) {
        if (cancelled) return
        if (err instanceof HttpError && err.code === 'banned') {
          setError('banned')
        } else {
          const msg = err instanceof Error ? err.message : String(err)
          setError(msg)
        }
        console.warn('[useChat] init error', err instanceof Error ? err.message : String(err))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void init()

    return () => {
      cancelled = true
    }
  }, [api, eventId, roomId, roomName])

  const sendMessage = useCallback(
    async (text: string, replyToId?: string): Promise<SendResponse> => {
      if (!chat) throw new Error('no active chat')
      try {
        return await api.sendMessage(chat.id, text, replyToId)
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err)
        console.warn('[useChat] sendMessage error', msg)
        throw err
      }
    },
    [api, chat],
  )

  return { chat, messages, initialHasMore, loading, error, sendMessage }
}
