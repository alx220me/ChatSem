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
  editedAt?: string  // ISO timestamp — present when the message was edited
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

export interface EditedMessage {
  id: string
  text: string
  edited_at: string
}

export interface PollResponse {
  messages: Message[]
  deleted_ids?: string[]
  last_delete_seq?: number
  edited_messages?: EditedMessage[]
  last_edit_seq?: number
}

export interface WidgetConfig {
  containerId: string
  eventId: string
  token: string
  roomId?: string
  onTokenExpired?: () => Promise<string>
}
