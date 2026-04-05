import React, { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { ConfirmDialog } from '../components/ConfirmDialog'
import type { Ban, Chat, Mute } from '../types'

type Tab = 'bans' | 'mutes'

// ─── Bans tab ────────────────────────────────────────────────────────────────

function BansTab({ eventId }: { eventId: string }): React.ReactElement {
  const { api } = useAuth()
  const [bans, setBans] = useState<Ban[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [userId, setUserId] = useState('')
  const [reason, setReason] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const [confirmBanId, setConfirmBanId] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!api) return
    setLoading(true)
    try {
      const data = await api.listBans(eventId)
      setBans(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load bans')
    } finally {
      setLoading(false)
    }
  }, [api, eventId])

  useEffect(() => {
    void load()
  }, [load])

  async function handleBan(e: React.FormEvent) {
    e.preventDefault()
    if (!api) return
    setFormError(null)
    setSubmitting(true)
    try {
      if (import.meta.env.DEV) {
        console.debug('[ModerationPage] ban', userId)
      }
      const ban = await api.createBan(userId, eventId, reason, expiresAt ? new Date(expiresAt).toISOString() : undefined)
      console.info('[ModerationPage] banned', ban.id)
      setBans((prev) => [ban, ...prev])
      setUserId('')
      setReason('')
      setExpiresAt('')
    } catch (err) {
      console.warn('[ModerationPage] action failed', err)
      setFormError(err instanceof Error ? err.message : 'Ban failed')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleUnban(banId: string) {
    if (!api) return
    try {
      await api.deleteBan(banId)
      setBans((prev) => prev.filter((b) => b.id !== banId))
    } catch (err) {
      console.warn('[ModerationPage] unban failed', err)
    }
    setConfirmBanId(null)
  }

  return (
    <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
      {/* Ban form */}
      <div style={{ minWidth: 280, flex: '0 0 auto' }}>
        <h3 style={{ fontSize: 15, margin: '0 0 12px' }}>Ban User</h3>
        <form onSubmit={handleBan} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {formError && <div style={{ color: '#b91c1c', fontSize: 13 }}>{formError}</div>}
          <label style={labelStyle}>
            User ID
            <input value={userId} onChange={(e) => setUserId(e.target.value)} required style={inputStyle} />
          </label>
          <label style={labelStyle}>
            Reason
            <input value={reason} onChange={(e) => setReason(e.target.value)} required style={inputStyle} />
          </label>
          <label style={labelStyle}>
            Expires At (optional)
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              style={inputStyle}
            />
          </label>
          <button type="submit" disabled={submitting} style={btnDangerStyle}>
            {submitting ? 'Banning...' : 'Ban'}
          </button>
        </form>
      </div>

      {/* Bans table */}
      <div style={{ flex: 1, minWidth: 300 }}>
        <h3 style={{ fontSize: 15, margin: '0 0 12px' }}>Active Bans</h3>
        {error && <div style={{ color: '#b91c1c', fontSize: 13, marginBottom: 8 }}>{error}</div>}
        {loading ? (
          <div style={{ color: '#9ca3af', fontSize: 14 }}>Loading...</div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ textAlign: 'left', borderBottom: '2px solid #e5e7eb' }}>
                <th style={thStyle}>User ID</th>
                <th style={thStyle}>Reason</th>
                <th style={thStyle}>Expires</th>
                <th style={thStyle}></th>
              </tr>
            </thead>
            <tbody>
              {bans.map((ban) => (
                <tr key={ban.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={tdStyle}>{ban.userId}</td>
                  <td style={tdStyle}>{ban.reason}</td>
                  <td style={tdStyle}>{ban.expiresAt ? new Date(ban.expiresAt).toLocaleDateString() : '—'}</td>
                  <td style={tdStyle}>
                    <button onClick={() => setConfirmBanId(ban.id)} style={btnSmallDangerStyle}>
                      Unban
                    </button>
                  </td>
                </tr>
              ))}
              {bans.length === 0 && (
                <tr>
                  <td colSpan={4} style={{ ...tdStyle, color: '#9ca3af', textAlign: 'center', padding: 24 }}>
                    No active bans
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>

      <ConfirmDialog
        open={confirmBanId !== null}
        title="Remove ban?"
        description="This will allow the user to access the chat again."
        confirmLabel="Unban"
        onConfirm={() => confirmBanId && void handleUnban(confirmBanId)}
        onCancel={() => setConfirmBanId(null)}
      />
    </div>
  )
}

// ─── Mutes tab ────────────────────────────────────────────────────────────────

function MutesTab({ eventId }: { eventId: string }): React.ReactElement {
  const { api } = useAuth()
  const [chats, setChats] = useState<Chat[]>([])
  const [selectedChatId, setSelectedChatId] = useState('')
  const [mutes, setMutes] = useState<Mute[]>([])
  const [loadingMutes, setLoadingMutes] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [userId, setUserId] = useState('')
  const [reason, setReason] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [confirmMuteId, setConfirmMuteId] = useState<string | null>(null)

  useEffect(() => {
    if (!api) return
    void api.listChats(eventId).then(setChats).catch(() => {})
  }, [api, eventId])

  const loadMutes = useCallback(
    async (chatId: string) => {
      if (!api) return
      setLoadingMutes(true)
      setError(null)
      try {
        const data = await api.listMutes(chatId)
        setMutes(data ?? [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load mutes')
      } finally {
        setLoadingMutes(false)
      }
    },
    [api],
  )

  function handleChatSelect(chatId: string) {
    setSelectedChatId(chatId)
    if (chatId) void loadMutes(chatId)
    else setMutes([])
  }

  async function handleMute(e: React.FormEvent) {
    e.preventDefault()
    if (!api || !selectedChatId) return
    setFormError(null)
    setSubmitting(true)
    try {
      if (import.meta.env.DEV) {
        console.debug('[ModerationPage] mute', userId)
      }
      const mute = await api.createMute(selectedChatId, userId, reason, expiresAt ? new Date(expiresAt).toISOString() : undefined)
      setMutes((prev) => [mute, ...prev])
      setUserId('')
      setReason('')
      setExpiresAt('')
    } catch (err) {
      console.warn('[ModerationPage] action failed', err)
      setFormError(err instanceof Error ? err.message : 'Mute failed')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleUnmute(muteId: string) {
    if (!api) return
    try {
      await api.deleteMute(muteId)
      setMutes((prev) => prev.filter((m) => m.id !== muteId))
    } catch (err) {
      console.warn('[ModerationPage] unmute failed', err)
    }
    setConfirmMuteId(null)
  }

  return (
    <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
      {/* Mute form */}
      <div style={{ minWidth: 280, flex: '0 0 auto' }}>
        <h3 style={{ fontSize: 15, margin: '0 0 12px' }}>Mute User</h3>
        <form onSubmit={handleMute} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {formError && <div style={{ color: '#b91c1c', fontSize: 13 }}>{formError}</div>}
          <label style={labelStyle}>
            Chat
            <select
              value={selectedChatId}
              onChange={(e) => handleChatSelect(e.target.value)}
              required
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
          <label style={labelStyle}>
            User ID
            <input value={userId} onChange={(e) => setUserId(e.target.value)} required style={inputStyle} />
          </label>
          <label style={labelStyle}>
            Reason
            <input value={reason} onChange={(e) => setReason(e.target.value)} required style={inputStyle} />
          </label>
          <label style={labelStyle}>
            Expires At (optional)
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              style={inputStyle}
            />
          </label>
          <button type="submit" disabled={submitting || !selectedChatId} style={{ ...btnDangerStyle, opacity: !selectedChatId ? 0.5 : 1 }}>
            {submitting ? 'Muting...' : 'Mute'}
          </button>
        </form>
      </div>

      {/* Mutes table */}
      <div style={{ flex: 1, minWidth: 300 }}>
        <h3 style={{ fontSize: 15, margin: '0 0 12px' }}>Active Mutes</h3>
        {error && <div style={{ color: '#b91c1c', fontSize: 13, marginBottom: 8 }}>{error}</div>}
        {!selectedChatId ? (
          <div style={{ color: '#9ca3af', fontSize: 14 }}>Select a chat to view mutes</div>
        ) : loadingMutes ? (
          <div style={{ color: '#9ca3af', fontSize: 14 }}>Loading...</div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ textAlign: 'left', borderBottom: '2px solid #e5e7eb' }}>
                <th style={thStyle}>User ID</th>
                <th style={thStyle}>Reason</th>
                <th style={thStyle}>Expires</th>
                <th style={thStyle}></th>
              </tr>
            </thead>
            <tbody>
              {mutes.map((mute) => (
                <tr key={mute.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={tdStyle}>{mute.userId}</td>
                  <td style={tdStyle}>{mute.reason}</td>
                  <td style={tdStyle}>{mute.expiresAt ? new Date(mute.expiresAt).toLocaleDateString() : '—'}</td>
                  <td style={tdStyle}>
                    <button onClick={() => setConfirmMuteId(mute.id)} style={btnSmallDangerStyle}>
                      Unmute
                    </button>
                  </td>
                </tr>
              ))}
              {mutes.length === 0 && (
                <tr>
                  <td colSpan={4} style={{ ...tdStyle, color: '#9ca3af', textAlign: 'center', padding: 24 }}>
                    No active mutes
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>

      <ConfirmDialog
        open={confirmMuteId !== null}
        title="Remove mute?"
        description="This will allow the user to send messages again."
        confirmLabel="Unmute"
        onConfirm={() => confirmMuteId && void handleUnmute(confirmMuteId)}
        onCancel={() => setConfirmMuteId(null)}
      />
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function ModerationPage(): React.ReactElement {
  const { eventId } = useParams<{ eventId: string }>()
  const [tab, setTab] = useState<Tab>('bans')

  if (!eventId) return <div>No event selected</div>

  return (
    <div>
      <h1 style={{ margin: '0 0 16px', fontSize: 20 }}>Moderation</h1>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 0, borderBottom: '2px solid #e5e7eb', marginBottom: 24 }}>
        {(['bans', 'mutes'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              padding: '8px 20px',
              border: 'none',
              borderBottom: tab === t ? '2px solid #2563eb' : '2px solid transparent',
              backgroundColor: 'transparent',
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: tab === t ? 600 : 400,
              color: tab === t ? '#2563eb' : '#6b7280',
              marginBottom: -2,
            }}
          >
            {t === 'bans' ? 'Bans' : 'Mutes'}
          </button>
        ))}
      </div>

      {tab === 'bans' ? <BansTab eventId={eventId} /> : <MutesTab eventId={eventId} />}
    </div>
  )
}

const labelStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }
const inputStyle: React.CSSProperties = { padding: '7px 10px', border: '1px solid #d1d5db', borderRadius: 4, fontSize: 13 }
const btnDangerStyle: React.CSSProperties = { padding: '8px 16px', backgroundColor: '#dc2626', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 13 }
const btnSmallDangerStyle: React.CSSProperties = { padding: '4px 10px', backgroundColor: '#dc2626', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
const thStyle: React.CSSProperties = { padding: '8px 10px', fontWeight: 600, color: '#374151' }
const tdStyle: React.CSSProperties = { padding: '8px 10px', color: '#4b5563' }
