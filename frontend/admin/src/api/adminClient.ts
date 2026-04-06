import type { Ban, Chat, Event, Mute, User } from '../types'

/** Returned once at event creation — contains the plaintext API secret. */
export interface CreateEventResponse extends Event {
  api_secret: string
}

export class AdminApiClient {
  private baseUrl: string
  private getToken: () => string

  constructor(baseUrl: string, getToken: () => string) {
    this.baseUrl = baseUrl
    this.getToken = getToken
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const url = `${this.baseUrl}${path}`
    if (import.meta.env.DEV) {
      console.debug('[AdminClient] request', method, url)
    }

    const res = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.getToken()}`,
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    if (!res.ok) {
      if (res.status === 401) {
        console.warn('[AdminClient] auth error', res.status)
      }
      const text = await res.text().catch(() => '')
      throw new Error(`${method} ${url} → ${res.status}: ${text}`)
    }

    if (res.status === 204) return undefined as T
    return res.json() as Promise<T>
  }

  // Events
  async listEvents(): Promise<Event[]> {
    return this.request<Event[]>('GET', '/api/admin/events')
  }

  async createEvent(name: string, allowedOrigin: string): Promise<CreateEventResponse> {
    return this.request<CreateEventResponse>('POST', '/api/admin/events', {
      name,
      allowed_origin: allowedOrigin,
    })
  }

  // Chats
  async listChats(eventId: string): Promise<Chat[]> {
    return this.request<Chat[]>('GET', `/api/admin/events/${eventId}/chats`)
  }

  async updateChatSettings(chatId: string, settings: Record<string, unknown>): Promise<void> {
    return this.request<void>('PATCH', `/api/admin/chats/${chatId}/settings`, { settings })
  }

  // Users
  async listUsers(eventId: string, limit: number, offset: number): Promise<User[]> {
    return this.request<User[]>(
      'GET',
      `/api/admin/events/${eventId}/users?limit=${limit}&offset=${offset}`,
    )
  }

  async updateUserRole(userId: string, role: User['role']): Promise<void> {
    return this.request<void>('PATCH', `/api/admin/users/${userId}/role`, { role })
  }

  // Bans
  async listBans(eventId: string): Promise<Ban[]> {
    return this.request<Ban[]>('GET', `/api/admin/events/${eventId}/bans`)
  }

  async createBan(
    userId: string,
    eventId: string,
    reason: string,
    expiresAt?: string,
  ): Promise<Ban> {
    return this.request<Ban>('POST', '/api/admin/bans', { userId, eventId, reason, expiresAt })
  }

  async deleteBan(banId: string): Promise<void> {
    return this.request<void>('DELETE', `/api/admin/bans/${banId}`)
  }

  // Mutes
  async listMutes(chatId: string): Promise<Mute[]> {
    return this.request<Mute[]>('GET', `/api/admin/chats/${chatId}/mutes`)
  }

  async createMute(
    chatId: string,
    userId: string,
    reason: string,
    expiresAt?: string,
  ): Promise<Mute> {
    return this.request<Mute>('POST', '/api/admin/mutes', { chatId, userId, reason, expiresAt })
  }

  async deleteMute(muteId: string): Promise<void> {
    return this.request<void>('DELETE', `/api/admin/mutes/${muteId}`)
  }

  // Export
  exportUrl(chatId: string, format: 'csv' | 'json'): string {
    return `${this.baseUrl}/api/admin/chats/${chatId}/export?format=${format}&token=${this.getToken()}`
  }
}
