import React from 'react'
import { NavLink, Outlet, useMatch, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

export function Layout(): React.ReactElement {
  const { userName, logout } = useAuth()
  const navigate = useNavigate()
  // useParams does not receive child-route params — use useMatch instead
  const match = useMatch('/events/:eventId/*')
  const eventId = match?.params.eventId

  function handleLogout() {
    logout()
    navigate('/login')
  }

  const navItems = eventId
    ? [
        { to: '/events', label: 'Events' },
        { to: `/events/${eventId}/chats`, label: 'Chats' },
        { to: `/events/${eventId}/users`, label: 'Users' },
        { to: `/events/${eventId}/moderation`, label: 'Moderation' },
        { to: `/events/${eventId}/export`, label: 'Export' },
      ]
    : [{ to: '/events', label: 'Events' }]

  return (
    <div style={{ display: 'flex', height: '100vh', fontFamily: 'system-ui, sans-serif' }}>
      {/* Sidebar */}
      <nav
        style={{
          width: 220,
          backgroundColor: '#1e293b',
          color: '#fff',
          display: 'flex',
          flexDirection: 'column',
          padding: '16px 0',
          flexShrink: 0,
        }}
      >
        <div style={{ padding: '0 16px 16px', fontWeight: 700, fontSize: 16, color: '#e2e8f0' }}>
          ChatSem Admin
        </div>
        <div style={{ flex: 1 }}>
          {navItems.map(({ to, label }) => (
            <NavLink
              key={to}
              to={to}
              style={({ isActive }) => ({
                display: 'block',
                padding: '10px 16px',
                color: isActive ? '#60a5fa' : '#94a3b8',
                textDecoration: 'none',
                fontSize: 14,
                fontWeight: isActive ? 600 : 400,
                backgroundColor: isActive ? 'rgba(96,165,250,0.1)' : 'transparent',
              })}
            >
              {label}
            </NavLink>
          ))}
        </div>
      </nav>

      {/* Main content */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {/* Header */}
        <header
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            padding: '12px 24px',
            borderBottom: '1px solid #e5e7eb',
            backgroundColor: '#fff',
            gap: 16,
          }}
        >
          <span style={{ fontSize: 14, color: '#374151' }}>{userName}</span>
          <button
            onClick={handleLogout}
            style={{
              padding: '6px 12px',
              border: '1px solid #d1d5db',
              borderRadius: 4,
              cursor: 'pointer',
              fontSize: 13,
              backgroundColor: '#fff',
            }}
          >
            Logout
          </button>
        </header>

        {/* Page content */}
        <main style={{ flex: 1, overflow: 'auto', padding: 24 }}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
