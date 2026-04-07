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
  replyToCreatedAt?: string
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
  /** Static JWT token. Use either this or tokenProvider, not both. */
  token?: string
  /**
   * Called on init and on every 401 to obtain a fresh JWT.
   * The widget handles the initial fetch and all subsequent refreshes automatically.
   * Replaces the token + onTokenExpired pair.
   */
  tokenProvider?: () => Promise<string>
  /** @deprecated Use tokenProvider instead. Kept for backward compatibility. */
  onTokenExpired?: () => Promise<string>
  roomId?: string
}
