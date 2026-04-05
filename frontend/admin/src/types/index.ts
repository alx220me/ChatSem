export interface Event {
  id: string
  name: string
  allowedOrigin: string
  createdAt: string
}

export interface Chat {
  id: string
  eventId: string
  type: 'parent' | 'child'
  externalRoomId: string | null
  settings: Record<string, unknown>
}

export interface User {
  id: string
  externalId: string
  eventId: string
  name: string
  role: 'user' | 'moderator' | 'admin'
}

export interface Ban {
  id: string
  userId: string
  eventId: string
  reason: string
  createdAt: string
  expiresAt: string | null
}

export interface Mute {
  id: string
  chatId: string
  userId: string
  reason: string
  createdAt: string
  expiresAt: string | null
}
