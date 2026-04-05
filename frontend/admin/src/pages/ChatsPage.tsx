import React, { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import type { Chat } from '../types'

export function ChatsPage(): React.ReactElement {
  const { eventId } = useParams<{ eventId: string }>()
  const { api } = useAuth()
  const [chats, setChats] = useState<Chat[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedChat, setSelectedChat] = useState<Chat | null>(null)
  const [settingsJson, setSettingsJson] = useState('')
  const [settingsError, setSettingsError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!api || !eventId) return
    setLoading(true)
    setError(null)
    try {
      const data = await api.listChats(eventId)
      setChats(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load chats')
    } finally {
      setLoading(false)
    }
  }, [api, eventId])

  useEffect(() => {
    void load()
  }, [load])

  function openSettings(chat: Chat) {
    setSelectedChat(chat)
    setSettingsJson(JSON.stringify(chat.settings, null, 2))
    setSettingsError(null)
  }

  async function handleSaveSettings() {
    if (!api || !selectedChat) return
    setSettingsError(null)

    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(settingsJson) as Record<string, unknown>
    } catch {
      setSettingsError('Invalid JSON')
      return
    }

    setSaving(true)
    try {
      if (import.meta.env.DEV) {
        console.debug('[ChatsPage] updateSettings', selectedChat.id)
      }
      await api.updateChatSettings(selectedChat.id, parsed)
      setChats((prev) =>
        prev.map((c) => (c.id === selectedChat.id ? { ...c, settings: parsed } : c)),
      )
      setSelectedChat(null)
      setToast('Settings saved')
      setTimeout(() => setToast(null), 3000)
    } catch (err) {
      console.warn('[ChatsPage] settings update failed', err)
      setSettingsError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const parentChats = chats.filter((c) => c.type === 'parent')
  const childChats = chats.filter((c) => c.type === 'child')

  return (
    <div style={{ position: 'relative' }}>
      <h1 style={{ margin: '0 0 16px', fontSize: 20 }}>Chats</h1>

      {toast && (
        <div
          style={{
            position: 'fixed',
            bottom: 24,
            right: 24,
            backgroundColor: '#16a34a',
            color: '#fff',
            padding: '10px 16px',
            borderRadius: 6,
            fontSize: 14,
            zIndex: 200,
          }}
        >
          {toast}
        </div>
      )}

      {error && <div style={{ color: '#b91c1c', marginBottom: 12, fontSize: 14 }}>{error}</div>}

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: 14 }}>Loading...</div>
      ) : (
        <div>
          {parentChats.map((parent) => (
            <div
              key={parent.id}
              style={{
                border: '1px solid #e5e7eb',
                borderRadius: 6,
                marginBottom: 16,
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '12px 16px',
                  backgroundColor: '#f9fafb',
                  borderBottom: '1px solid #e5e7eb',
                }}
              >
                <div>
                  <span style={{ fontWeight: 600, fontSize: 14 }}>Parent chat</span>
                  <span style={{ color: '#9ca3af', fontSize: 12, marginLeft: 8 }}>{parent.id}</span>
                </div>
                <button
                  onClick={() => openSettings(parent)}
                  style={btnSecondaryStyle}
                >
                  Settings
                </button>
              </div>
              {childChats.length > 0 && (
                <div style={{ padding: '8px 16px' }}>
                  {childChats.map((child) => (
                    <div
                      key={child.id}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        padding: '8px 0',
                        borderBottom: '1px solid #f3f4f6',
                        fontSize: 14,
                        gap: 12,
                      }}
                    >
                      <span style={{ color: '#9ca3af', fontSize: 12 }}>└</span>
                      <span style={{ color: '#374151' }}>
                        {child.externalRoomId ?? child.id}
                      </span>
                      <span style={{ color: '#9ca3af', fontSize: 12 }}>child</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
          {chats.length === 0 && (
            <div style={{ color: '#9ca3af', fontSize: 14 }}>No chats found</div>
          )}
        </div>
      )}

      {/* Settings sidebar */}
      {selectedChat && (
        <div
          style={{
            position: 'fixed',
            top: 0,
            right: 0,
            bottom: 0,
            width: 400,
            backgroundColor: '#fff',
            boxShadow: '-4px 0 16px rgba(0,0,0,0.1)',
            display: 'flex',
            flexDirection: 'column',
            zIndex: 50,
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '16px 20px',
              borderBottom: '1px solid #e5e7eb',
            }}
          >
            <h3 style={{ margin: 0, fontSize: 16 }}>Chat Settings</h3>
            <button
              onClick={() => setSelectedChat(null)}
              style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: 18 }}
            >
              ×
            </button>
          </div>
          <div style={{ flex: 1, padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ fontSize: 12, color: '#9ca3af' }}>Chat ID: {selectedChat.id}</div>
            {settingsError && (
              <div style={{ color: '#b91c1c', fontSize: 13 }}>{settingsError}</div>
            )}
            <textarea
              value={settingsJson}
              onChange={(e) => setSettingsJson(e.target.value)}
              rows={16}
              style={{
                fontFamily: 'monospace',
                fontSize: 13,
                padding: 10,
                border: '1px solid #d1d5db',
                borderRadius: 4,
                resize: 'vertical',
              }}
            />
            <button
              onClick={() => void handleSaveSettings()}
              disabled={saving}
              style={{ ...btnPrimaryStyle, opacity: saving ? 0.7 : 1 }}
            >
              {saving ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const btnPrimaryStyle: React.CSSProperties = { padding: '7px 14px', backgroundColor: '#2563eb', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 13 }
const btnSecondaryStyle: React.CSSProperties = { padding: '6px 12px', border: '1px solid #d1d5db', borderRadius: 4, cursor: 'pointer', fontSize: 13, backgroundColor: '#fff' }
