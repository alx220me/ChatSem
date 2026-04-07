import React, { useCallback, useEffect, useState } from 'react'

interface EmbedCodeModalProps {
  eventId: string
  onClose: () => void
}

export function EmbedCodeModal({ eventId, onClose }: EmbedCodeModalProps): React.ReactElement {
  const [copied, setCopied] = useState(false)

  const baseUrl = import.meta.env.VITE_WIDGET_BASE_URL ?? 'http://localhost:5173'

  const snippet = `<!-- Option A: floating widget (recommended) -->
<!-- No placeholder div needed — widget creates its own container -->
<script src="${baseUrl}/widget.js" defer></script>
<script defer>
  window.ChatSem.init({
    eventId: '${eventId}',
    floating: true,
    defaultCollapsed: true,    // start as FAB icon; false = start expanded
    // roomName: 'Main Stage',    // optional: room display name shown in the header
    // accentColor: '#2563eb',    // header and FAB button color (default: blue)
    // accentTextColor: '#ffffff', // text/icon color in header and FAB (default: white)
    // roomId: 'my-room',         // optional: embed a specific room
    tokenProvider: async function() {
      // Call YOUR backend — it holds the API secret and calls ChatSem auth:
      //   POST /api/auth/token  Authorization: Bearer <api_secret>
      const res = await fetch('/your-server/chat-token');
      const data = await res.json();
      return data.token;
    },
  });
</script>

<!-- Option B: embedded in your layout -->
<div id="chat-widget" style="height:500px"></div>
<script src="${baseUrl}/widget.js" defer></script>
<script defer>
  window.ChatSem.init({
    containerId: 'chat-widget',
    eventId: '${eventId}',
    // roomName: 'Main Stage',    // optional: room display name shown in the header
    // accentColor: '#2563eb',    // header color (default: blue)
    // accentTextColor: '#ffffff', // text/icon color in header (default: white)
    // roomId: 'my-room',
    tokenProvider: async function() {
      const res = await fetch('/your-server/chat-token');
      const data = await res.json();
      return data.token;
    },
  });
</script>`

  const handleCopy = useCallback(() => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(snippet).then(() => {
        if (import.meta.env.DEV) {
          console.info('[EmbedCodeModal] snippet copied', 'event_id', eventId)
        }
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }).catch(() => fallbackCopy(snippet))
    } else {
      fallbackCopy(snippet)
    }
  }, [snippet, eventId])

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

  useEffect(() => {
    if (import.meta.env.DEV) {
      console.debug('[EmbedCodeModal] opened', 'event_id', eventId)
    }

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [eventId, onClose])

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
          maxWidth: 680,
          width: '100%',
          boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ margin: '0 0 12px', fontSize: 18 }}>Widget Embed Code</h2>
        <p style={{ margin: '0 0 12px', fontSize: 14, color: '#6b7280' }}>
          Choose <strong>Option A</strong> (floating FAB, no placeholder div needed) or{' '}
          <strong>Option B</strong> (embedded in your layout).
          Implement{' '}
          <code style={{ background: '#f3f4f6', padding: '1px 4px', borderRadius: 3 }}>
            /your-server/chat-token
          </code>{' '}
          on your backend — it calls the ChatSem auth API with your API secret and returns{' '}
          <code style={{ background: '#f3f4f6', padding: '1px 4px', borderRadius: 3 }}>
            {'{'}token{'}'}
          </code>.
          The widget refreshes the token automatically on expiry.
        </p>
        <pre
          style={{
            background: '#f3f4f6',
            padding: 12,
            borderRadius: 4,
            fontFamily: 'monospace',
            fontSize: 12,
            overflowX: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            marginBottom: 16,
            color: '#111827',
          }}
        >
          {snippet}
        </pre>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <button
            onClick={onClose}
            style={{
              padding: '8px 16px',
              border: '1px solid #d1d5db',
              borderRadius: 4,
              cursor: 'pointer',
              fontSize: 14,
              backgroundColor: '#fff',
            }}
          >
            Close
          </button>
          <button
            onClick={handleCopy}
            style={{
              padding: '8px 16px',
              border: 'none',
              borderRadius: 4,
              cursor: 'pointer',
              fontSize: 14,
              backgroundColor: copied ? '#16a34a' : '#2563eb',
              color: '#fff',
              transition: 'background-color 0.15s',
            }}
          >
            {copied ? 'Copied ✓' : 'Copy'}
          </button>
        </div>
      </div>
    </div>
  )
}
