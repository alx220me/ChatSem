import React, { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import type { Chat } from '../types'

type Format = 'csv' | 'json'

export function ExportPage(): React.ReactElement {
  const { eventId } = useParams<{ eventId: string }>()
  const { api } = useAuth()
  const [chats, setChats] = useState<Chat[]>([])
  const [selectedChatId, setSelectedChatId] = useState('')
  const [format, setFormat] = useState<Format>('csv')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!api || !eventId) return
    api
      .listChats(eventId)
      .then(setChats)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : 'Failed to load chats')
      })
  }, [api, eventId])

  function handleDownload() {
    if (!api || !selectedChatId) return
    if (import.meta.env.DEV) {
      console.debug('[ExportPage] export', selectedChatId, format)
    }
    const url = api.exportUrl(selectedChatId, format)
    const a = document.createElement('a')
    a.href = url
    a.download = `chat-${selectedChatId}.${format}`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  return (
    <div>
      <h1 style={{ margin: '0 0 16px', fontSize: 20 }}>History Export</h1>

      {error && <div style={{ color: '#b91c1c', marginBottom: 12, fontSize: 14 }}>{error}</div>}

      <div
        style={{
          maxWidth: 480,
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
        }}
      >
        <label style={labelStyle}>
          Chat
          <select
            value={selectedChatId}
            onChange={(e) => setSelectedChatId(e.target.value)}
            style={inputStyle}
          >
            <option value="">— select chat —</option>
            {chats.map((c) => (
              <option key={c.id} value={c.id}>
                {c.type} {c.externalRoomId ? `(${c.externalRoomId})` : c.id}
              </option>
            ))}
          </select>
        </label>

        <fieldset style={{ border: '1px solid #e5e7eb', borderRadius: 6, padding: '12px 16px' }}>
          <legend style={{ fontSize: 13, color: '#374151', padding: '0 4px' }}>Format</legend>
          <div style={{ display: 'flex', gap: 16 }}>
            {(['csv', 'json'] as Format[]).map((f) => (
              <label key={f} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 14, cursor: 'pointer' }}>
                <input
                  type="radio"
                  name="format"
                  value={f}
                  checked={format === f}
                  onChange={() => setFormat(f)}
                />
                {f.toUpperCase()}
              </label>
            ))}
          </div>
        </fieldset>

        {!selectedChatId && (
          <div
            style={{
              padding: '12px 16px',
              backgroundColor: '#f9fafb',
              border: '1px solid #e5e7eb',
              borderRadius: 6,
              fontSize: 13,
              color: '#6b7280',
            }}
          >
            Select a chat to enable download. The export endpoint will be available once the
            History Export milestone is implemented on the backend.
          </div>
        )}

        <button
          onClick={handleDownload}
          disabled={!selectedChatId}
          style={{
            padding: '10px 20px',
            backgroundColor: '#2563eb',
            color: '#fff',
            border: 'none',
            borderRadius: 6,
            cursor: selectedChatId ? 'pointer' : 'not-allowed',
            fontSize: 14,
            opacity: selectedChatId ? 1 : 0.5,
            alignSelf: 'flex-start',
          }}
        >
          Download
        </button>
      </div>
    </div>
  )
}

const labelStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 6, fontSize: 14, color: '#374151' }
const inputStyle: React.CSSProperties = { padding: '8px 10px', border: '1px solid #d1d5db', borderRadius: 4, fontSize: 14 }
