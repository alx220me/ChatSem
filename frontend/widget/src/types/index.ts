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
  /**
   * ID of the host-page element to mount into.
   * In floating mode this field is optional — if omitted or the element is not found,
   * the widget creates its own <div id="chatsem-widget"> and appends it to <body>.
   */
  containerId?: string
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
  /**
   * When true, renders the widget as a fixed-position floating window.
   * Supports drag-to-move (by header) and collapse to FAB icon.
   */
  floating?: boolean
  /**
   * Initial collapsed state when floating=true.
   * true  → starts as FAB icon with online count badge.
   * false → starts expanded (default).
   */
  defaultCollapsed?: boolean
  /**
   * Initial position for the floating window.
   * When omitted, the widget snaps to the bottom-right corner (right: 20px, bottom: 20px).
   * After the user drags the widget, position is stored in React state.
   */
  defaultPosition?: { x: number; y: number }
  /**
   * Initial size of the floating window in pixels.
   * Defaults to 360 × 520. The user can resize by dragging the edges/corners.
   * Min: 240 × 280. Max: 900 × 900.
   */
  defaultSize?: { w: number; h: number }
  /**
   * Accent color for the widget UI (CSS color string, e.g. "#2563eb" or "rgb(37,99,235)").
   * Applied to the chat header background and the collapsed FAB button.
   * Defaults to #2563eb (blue).
   */
  accentColor?: string
  /**
   * Text/icon color used inside elements painted with accentColor (header, FAB).
   * Defaults to #ffffff (white). Use a dark value (e.g. "#111111") when accentColor is light.
   */
  accentTextColor?: string
}
