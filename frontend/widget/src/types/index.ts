export interface Chat {
  id: string
  eventId: string
  parentId: string | null
  type: 'parent' | 'child'
  externalRoomId: string | null
}

export interface Message {
  id: string
  chatId: string
  userId: string
  text: string
  seq: number
  createdAt: string
}

export interface SendResponse {
  id: string
  seq: number
  ts: string
}

export interface PollResponse {
  messages: Message[]
}

export interface WidgetConfig {
  containerId: string
  eventId: string
  token: string
  roomId?: string
  onTokenExpired?: () => Promise<string>
}
