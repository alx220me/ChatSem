import type { Chat, Message, SendResponse, PollResponse } from '../types'

export class HttpError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    /** Seconds to wait before retrying (from Retry-After header). */
    public readonly retryAfter: number = 0,
  ) {
    super(message)
    this.name = 'HttpError'
  }
}

export class ApiClient {
  private baseUrl: string
  private getToken: () => string
  private onTokenExpired?: () => Promise<string>
  /** Deduplicates concurrent token refresh calls — only one in-flight at a time. */
  private refreshPromise: Promise<string> | null = null

  constructor(
    baseUrl: string,
    getToken: () => string,
    onTokenExpired?: () => Promise<string>,
  ) {
    this.baseUrl = baseUrl
    this.getToken = getToken
    this.onTokenExpired = onTokenExpired
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    signal?: AbortSignal,
    retry = true,
  ): Promise<T> {
    if (import.meta.env.DEV) {
      console.debug('[ApiClient] request', method, `${this.baseUrl}${path}`)
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${this.getToken()}`,
    }

    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body != null ? JSON.stringify(body) : undefined,
      signal,
    })

    if (res.status === 401 && retry && this.onTokenExpired) {
      if (import.meta.env.DEV) {
        console.warn('[ApiClient] token expired, refreshing')
      }
      // Deduplicate: if a refresh is already in-flight (e.g. concurrent poll + heartbeat
      // both got 401), all callers await the same promise instead of triggering N refreshes.
      if (!this.refreshPromise) {
        this.refreshPromise = this.onTokenExpired().finally(() => {
          this.refreshPromise = null
        })
      }
      const newToken = await this.refreshPromise
      // Permanently update getToken so subsequent requests use the new token.
      // Not restoring the old getter was the root cause of the infinite 401→refresh loop.
      this.getToken = () => newToken
      return await this.request<T>(method, path, body, signal, false)
    }

    if (!res.ok) {
      const retryAfter = parseInt(res.headers.get('Retry-After') ?? '0', 10) || 0
      throw new HttpError(res.status, `HTTP ${res.status}: ${res.statusText}`, retryAfter)
    }

    // 204 No Content — return empty object cast to T
    if (res.status === 204) {
      return {} as T
    }

    return res.json() as Promise<T>
  }

  async listChats(eventId: string): Promise<Chat[]> {
    const res = await this.request<{ parent: Chat; children: Chat[] }>(
      'GET',
      `/chat/events/${eventId}/chats`,
    )
    return [res.parent, ...(res.children ?? [])]
  }

  async joinRoom(eventId: string, roomId: string): Promise<void> {
    await this.request<unknown>('POST', `/chat/join`, { event_id: eventId, room_id: roomId })
  }

  async getMessages(
    chatId: string,
    limit: number,
    before?: number,
  ): Promise<{ messages: Message[]; has_more: boolean }> {
    const url = `/chat/${chatId}/messages?limit=${limit}${before != null ? `&before=${before}` : ''}`
    const res = await this.request<{ messages: Message[]; has_more: boolean }>('GET', url)
    return { messages: res.messages ?? [], has_more: res.has_more ?? false }
  }

  async sendMessage(chatId: string, text: string, replyToId?: string): Promise<SendResponse> {
    const body: Record<string, unknown> = { text }
    if (replyToId) body.reply_to_id = replyToId
    return this.request<SendResponse>('POST', `/chat/${chatId}/messages`, body)
  }

  async poll(
    chatId: string,
    afterSeq: number,
    afterDeleteSeq: number,
    afterEditSeq: number,
    signal: AbortSignal,
  ): Promise<PollResponse> {
    return this.request<PollResponse>(
      'GET',
      `/chat/${chatId}/poll?after=${afterSeq}&after_delete_seq=${afterDeleteSeq}&after_edit_seq=${afterEditSeq}`,
      undefined,
      signal,
    )
  }

  async editMessage(
    msgId: string,
    text: string,
  ): Promise<{ id: string; text: string; edited_at: string }> {
    return this.request('PATCH', `/chat/messages/${msgId}`, { text })
  }

  /** Decode JWT payload (no signature verification — server already did that). */
  private decodeJwtPayload(): Record<string, unknown> {
    try {
      const parts = this.getToken().split('.')
      if (parts.length !== 3) return {}
      const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
      const binary = atob(base64)
      const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
      return JSON.parse(new TextDecoder().decode(bytes)) as Record<string, unknown>
    } catch {
      return {}
    }
  }

  getCurrentUserName(): string {
    return (this.decodeJwtPayload().name as string) || ''
  }

  getCurrentUserId(): string {
    return (this.decodeJwtPayload().user_id as string) || ''
  }

  getCurrentUserRole(): string {
    return (this.decodeJwtPayload().role as string) || ''
  }

  async deleteMessage(msgId: string): Promise<void> {
    await this.request<unknown>('DELETE', `/chat/messages/${msgId}`)
  }

  async banUser(userId: string, eventId: string, reason: string): Promise<void> {
    await this.request<unknown>('POST', `/admin/bans`, { user_id: userId, event_id: eventId, reason })
  }

  async muteUser(userId: string, chatId: string, reason: string): Promise<void> {
    await this.request<unknown>('POST', `/admin/mutes`, { user_id: userId, chat_id: chatId, reason })
  }

  async heartbeat(chatId: string): Promise<void> {
    await this.request<unknown>('POST', `/chat/${chatId}/heartbeat`)
  }

  async leave(chatId: string): Promise<void> {
    await this.request<unknown>('DELETE', `/chat/${chatId}/heartbeat`)
  }

  /** Fire-and-forget leave with keepalive=true — survives page close. */
  leaveBeacon(chatId: string): void {
    fetch(`${this.baseUrl}/chat/${chatId}/heartbeat`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${this.getToken()}` },
      keepalive: true,
    }).catch(() => {})
  }

  async getOnlineCount(chatId: string): Promise<number> {
    const res = await this.request<{ count: number }>('GET', `/chat/${chatId}/online`)
    return res.count ?? 0
  }
}
