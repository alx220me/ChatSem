import React, { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import type { User } from '../types'

const PAGE_SIZE = 50

export function UsersPage(): React.ReactElement {
  const { eventId } = useParams<{ eventId: string }>()
  const { api } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)

  const load = useCallback(
    async (off: number) => {
      if (!api || !eventId) return
      setLoading(true)
      setError(null)
      try {
        const data = await api.listUsers(eventId, PAGE_SIZE, off)
        setUsers(data)
        setHasMore(data.length === PAGE_SIZE)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load users')
      } finally {
        setLoading(false)
      }
    },
    [api, eventId],
  )

  useEffect(() => {
    void load(offset)
  }, [load, offset])

  async function handleRoleChange(user: User, role: User['role']) {
    if (!api) return
    try {
      if (import.meta.env.DEV) {
        console.debug('[UsersPage] updateRole', user.id, role)
      }
      await api.updateUserRole(user.id, role)
      setUsers((prev) => prev.map((u) => (u.id === user.id ? { ...u, role } : u)))
    } catch (err) {
      console.warn('[UsersPage] updateRole failed', err)
    }
  }

  return (
    <div>
      <h1 style={{ margin: '0 0 16px', fontSize: 20 }}>Users</h1>

      {error && <div style={{ color: '#b91c1c', marginBottom: 12, fontSize: 14 }}>{error}</div>}

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: 14 }}>Loading...</div>
      ) : (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ textAlign: 'left', borderBottom: '2px solid #e5e7eb' }}>
                <th style={thStyle}>External ID</th>
                <th style={thStyle}>Name</th>
                <th style={thStyle}>Role</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                  <td style={tdStyle}>{user.externalId}</td>
                  <td style={tdStyle}>{user.name}</td>
                  <td style={tdStyle}>
                    <select
                      value={user.role}
                      onChange={(e) =>
                        void handleRoleChange(user, e.target.value as User['role'])
                      }
                      style={{
                        border: '1px solid #d1d5db',
                        borderRadius: 4,
                        padding: '4px 8px',
                        fontSize: 13,
                        cursor: 'pointer',
                      }}
                    >
                      <option value="user">user</option>
                      <option value="moderator">moderator</option>
                      <option value="admin">admin</option>
                    </select>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td
                    colSpan={3}
                    style={{ ...tdStyle, color: '#9ca3af', textAlign: 'center', padding: 32 }}
                  >
                    No users found
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
            <button
              onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
              disabled={offset === 0}
              style={{ ...btnStyle, opacity: offset === 0 ? 0.4 : 1 }}
            >
              ← Prev
            </button>
            <button
              onClick={() => setOffset((o) => o + PAGE_SIZE)}
              disabled={!hasMore}
              style={{ ...btnStyle, opacity: !hasMore ? 0.4 : 1 }}
            >
              Next →
            </button>
          </div>
        </>
      )}
    </div>
  )
}

const thStyle: React.CSSProperties = { padding: '8px 12px', fontWeight: 600, color: '#374151' }
const tdStyle: React.CSSProperties = { padding: '10px 12px', color: '#4b5563' }
const btnStyle: React.CSSProperties = { padding: '7px 14px', border: '1px solid #d1d5db', borderRadius: 4, cursor: 'pointer', fontSize: 13, backgroundColor: '#fff' }
