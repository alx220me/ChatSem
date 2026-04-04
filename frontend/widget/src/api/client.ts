import type { Chat, Message, SendResponse, PollResponse } from '../types'

export class ApiClient {
  private baseUrl: string
  private getToken: () => string
  private onTokenExpired?: () => Promise<string>

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
      const newToken = await this.onTokenExpired()
      // Update token via closure — caller must re-wire getToken if needed.
      // We store refreshed token temporarily for the retry call.
      const savedGetToken = this.getToken
      this.getToken = () => newToken
      try {
        return await this.request<T>(method, path, body, signal, false)
      } finally {
        this.getToken = savedGetToken
      }
    }

    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`)
    }

    // 204 No Content — return empty object cast to T
    if (res.status === 204) {
      return {} as T
    }

    return res.json() as Promise<T>
  }

  async listChats(eventId: string): Promise<Chat[]> {
    return this.request<Chat[]>('GET', `/chat/events/${eventId}/chats`)
  }

  async joinRoom(eventId: string, roomId: string): Promise<void> {
    await this.request<unknown>('POST', `/chat/join`, { eventId, roomId })
  }

  async getMessages(chatId: string, limit: number): Promise<Message[]> {
    return this.request<Message[]>('GET', `/chat/chats/${chatId}/messages?limit=${limit}`)
  }

  async sendMessage(chatId: string, text: string): Promise<SendResponse> {
    return this.request<SendResponse>('POST', `/chat/${chatId}/messages`, { text })
  }

  async poll(chatId: string, afterSeq: number, signal: AbortSignal): Promise<PollResponse> {
    return this.request<PollResponse>(
      'GET',
      `/chat/${chatId}/poll?after=${afterSeq}`,
      undefined,
      signal,
    )
  }
}
