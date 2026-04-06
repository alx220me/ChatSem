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
  userName?: string
  text: string
  seq: number
  createdAt: string
  // Reply fields — populated by the server when the message is a reply.
  replyToId?: string
  replyToSeq?: number
  replyToText?: string
  replyToUserName?: string
}

export interface SendResponse {
  id: string
  seq: number
  ts: string
}

export interface PollResponse {
  messages: Message[]
  deleted_ids?: string[]
  last_delete_seq?: number
}

export interface WidgetConfig {
  containerId: string
  eventId: string
  token: string
  roomId?: string
  onTokenExpired?: () => Promise<string>
}
