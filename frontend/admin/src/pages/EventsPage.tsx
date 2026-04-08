import React, { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { EmbedCodeModal } from '../components/EmbedCodeModal'
import type { Event } from '../types'

interface CreateEventModalProps {
  onClose: () => void
  onCreated: (event: Event) => void
}

function CreateEventModal({ onClose, onCreated }: CreateEventModalProps): React.ReactElement {
  const { api } = useAuth()
  const [name, setName] = useState('')
  const [allowedOrigin, setAllowedOrigin] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  function copySecret() {
    if (!createdSecret) return
    if (navigator.clipboard) {
      navigator.clipboard.writeText(createdSecret).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }).catch(() => fallbackCopy(createdSecret))
    } else {
      fallbackCopy(createdSecret)
    }
  }

  function fallbackCopy(text: string) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    try {
      document.execCommand('copy')
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } finally {
      document.body.removeChild(ta)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!api) return
    setError(null)
    setLoading(true)
    try {
      if (import.meta.env.DEV) {
        console.debug('[EventsPage] createEvent', name)
      }
      const event = await api.createEvent(name, allowedOrigin)
      console.info('[EventsPage] event created', event.id)
      setCreatedSecret(event.api_secret)
      onCreated(event)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create event')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: '#fff',
          borderRadius: 8,
          padding: 24,
          maxWidth: 440,
          width: '100%',
          boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ margin: '0 0 16px', fontSize: 18 }}>Create Event</h2>

        {createdSecret ? (
          <div>
            <p style={{ fontSize: 14, color: '#374151', marginBottom: 8 }}>
              Event created! Save the API secret — it will only be shown once:
            </p>
            <div
              style={{
                background: '#f3f4f6',
                padding: 12,
                borderRadius: 4,
                fontFamily: 'monospace',
                fontSize: 13,
                wordBreak: 'break-all',
                marginBottom: 16,
              }}
            >
              {createdSecret}
            </div>
            <button
              onClick={copySecret}
              style={btnSecondaryStyle}
            >
              {copied ? 'Copied!' : 'Copy to clipboard'}
            </button>
            <button onClick={onClose} style={{ ...btnPrimaryStyle, marginLeft: 8 }}>
              Done
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {error && (
              <div style={{ color: '#b91c1c', fontSize: 13, padding: '6px 0' }}>{error}</div>
            )}
            <label style={labelStyle}>
              Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                style={inputStyle}
              />
            </label>
            <label style={labelStyle}>
              Allowed Origin
              <input
                value={allowedOrigin}
                onChange={(e) => setAllowedOrigin(e.target.value)}
                placeholder="https://example.com"
                required
                style={inputStyle}
              />
            </label>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
              <button type="button" onClick={onClose} style={btnSecondaryStyle}>
                Cancel
              </button>
              <button type="submit" disabled={loading} style={btnPrimaryStyle}>
                {loading ? 'Creating...' : 'Create'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

interface RotateSecretModalProps {
  event: Event
  onClose: () => void
}

function RotateSecretModal({ event, onClose }: RotateSecretModalProps): React.ReactElement {
  const { api } = useAuth()
  const [newSecret, setNewSecret] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  async function handleConfirm() {
    if (!api) return
    setLoading(true)
    setError(null)
    try {
      if (import.meta.env.DEV) {
        console.debug('[EventsPage] rotateAPISecret', event.id)
      }
      const res = await api.rotateAPISecret(event.id)
      console.info('[EventsPage] secret rotated', event.id)
      setNewSecret(res.api_secret)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rotate secret')
    } finally {
      setLoading(false)
    }
  }

  function copySecret() {
    if (!newSecret) return
    if (navigator.clipboard) {
      navigator.clipboard.writeText(newSecret).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }).catch(() => fallbackCopy(newSecret))
    } else {
      fallbackCopy(newSecret)
    }
  }

  function fallbackCopy(text: string) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    try {
      document.execCommand('copy')
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } finally {
      document.body.removeChild(ta)
    }
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: '#fff',
          borderRadius: 8,
          padding: 24,
          maxWidth: 440,
          width: '100%',
          boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ margin: '0 0 16px', fontSize: 18 }}>Rotate API Secret</h2>

        {newSecret ? (
          <div>
            <p style={{ fontSize: 14, color: '#374151', marginBottom: 8 }}>
              New API secret for <strong>{event.name}</strong>. Save it — it will only be shown once:
            </p>
            <div
              style={{
                background: '#f3f4f6',
                padding: 12,
                borderRadius: 4,
                fontFamily: 'monospace',
                fontSize: 13,
                wordBreak: 'break-all',
                marginBottom: 16,
              }}
            >
              {newSecret}
            </div>
            <button onClick={copySecret} style={btnSecondaryStyle}>
              {copied ? 'Copied!' : 'Copy to clipboard'}
            </button>
            <button onClick={onClose} style={{ ...btnPrimaryStyle, marginLeft: 8 }}>
              Done
            </button>
          </div>
        ) : (
          <div>
            <p style={{ fontSize: 14, color: '#374151', marginBottom: 16 }}>
              Generate a new API secret for <strong>{event.name}</strong>?
              The old secret will stop working immediately.
            </p>
            {error && (
              <div style={{ color: '#b91c1c', fontSize: 13, marginBottom: 12 }}>{error}</div>
            )}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
              <button onClick={onClose} style={btnSecondaryStyle}>
                Cancel
              </button>
              <button
                onClick={() => void handleConfirm()}
                disabled={loading}
                style={{ ...btnPrimaryStyle, backgroundColor: '#dc2626' }}
              >
                {loading ? 'Rotating...' : 'Rotate Secret'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export function EventsPage(): React.ReactElement {
  const { api } = useAuth()
  const navigate = useNavigate()
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showModal, setShowModal] = useState(false)
  const [embedEventId, setEmbedEventId] = useState<string | null>(null)
  const [rotateTarget, setRotateTarget] = useState<Event | null>(null)

  const load = useCallback(async () => {
    if (!api) return
    setLoading(true)
    setError(null)
    try {
      const data = await api.listEvents()
      setEvents(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load events')
    } finally {
      setLoading(false)
    }
  }, [api])

  useEffect(() => {
    void load()
  }, [load])

  function handleCreated(event: Event) {
    setEvents((prev) => [...prev, event])
    // Modal stays open to show the api_secret; user closes it via "Done"
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1 style={{ margin: 0, fontSize: 20 }}>Events</h1>
        <button onClick={() => setShowModal(true)} style={btnPrimaryStyle}>
          + Create Event
        </button>
      </div>

      {error && <div style={{ color: '#b91c1c', marginBottom: 12, fontSize: 14 }}>{error}</div>}

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: 14 }}>Loading...</div>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '2px solid #e5e7eb' }}>
              <th style={thStyle}>Name</th>
              <th style={thStyle}>ID</th>
              <th style={thStyle}>Allowed Origin</th>
              <th style={thStyle}>Created At</th>
              <th style={thStyle}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {events.map((ev) => (
              <tr
                key={ev.id}
                onClick={() => navigate(`/events/${ev.id}/chats`)}
                style={{ borderBottom: '1px solid #f3f4f6', cursor: 'pointer' }}
                onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = '#f9fafb')}
                onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = '')}
              >
                <td style={tdStyle}>{ev.name}</td>
                <td style={{ ...tdStyle, fontFamily: 'monospace', fontSize: 12, color: '#6b7280' }}>
                  <span
                    title="Click to copy"
                    onClick={(e) => { e.stopPropagation(); void navigator.clipboard.writeText(ev.id) }}
                    style={{ cursor: 'copy' }}
                  >
                    {ev.id}
                  </span>
                </td>
                <td style={tdStyle}>{ev.allowedOrigin}</td>
                <td style={tdStyle}>{new Date(ev.createdAt).toLocaleString()}</td>
                <td style={{ ...tdStyle, display: 'flex', gap: 6 }} onClick={(e) => e.stopPropagation()}>
                  <button
                    onClick={() => setEmbedEventId(ev.id)}
                    style={btnSecondaryStyle}
                  >
                    Code
                  </button>
                  <button
                    onClick={() => setRotateTarget(ev)}
                    style={{ ...btnSecondaryStyle, color: '#dc2626', borderColor: '#fca5a5' }}
                  >
                    Rotate Secret
                  </button>
                </td>
              </tr>
            ))}
            {events.length === 0 && (
              <tr>
                <td colSpan={5} style={{ ...tdStyle, color: '#9ca3af', textAlign: 'center', padding: 32 }}>
                  No events yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}

      {showModal && (
        <CreateEventModal onClose={() => setShowModal(false)} onCreated={handleCreated} />
      )}

      {embedEventId && (
        <EmbedCodeModal eventId={embedEventId} onClose={() => setEmbedEventId(null)} />
      )}

      {rotateTarget && (
        <RotateSecretModal event={rotateTarget} onClose={() => setRotateTarget(null)} />
      )}
    </div>
  )
}

const labelStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: 14 }
const inputStyle: React.CSSProperties = { padding: '8px 10px', border: '1px solid #d1d5db', borderRadius: 4, fontSize: 14 }
const btnPrimaryStyle: React.CSSProperties = { padding: '8px 16px', backgroundColor: '#2563eb', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 14 }
const btnSecondaryStyle: React.CSSProperties = { padding: '8px 16px', border: '1px solid #d1d5db', borderRadius: 4, cursor: 'pointer', fontSize: 14, backgroundColor: '#fff' }
const thStyle: React.CSSProperties = { padding: '8px 12px', fontWeight: 600, color: '#374151' }
const tdStyle: React.CSSProperties = { padding: '10px 12px', color: '#4b5563' }
