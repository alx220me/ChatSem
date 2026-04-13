import React from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ToastProvider } from './context/ToastContext'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { EventsPage } from './pages/EventsPage'
import { ChatsPage } from './pages/ChatsPage'
import { UsersPage } from './pages/UsersPage'
import { ModerationPage } from './pages/ModerationPage'
import { ExportPage } from './pages/ExportPage'

function PrivateRoute({ children }: { children: React.ReactElement }): React.ReactElement {
  const { token } = useAuth()
  return token ? children : <Navigate to="/login" replace />
}

function AppRoutes(): React.ReactElement {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <PrivateRoute>
            <Layout />
          </PrivateRoute>
        }
      >
        <Route path="/events" element={<EventsPage />} />
        <Route path="/events/:eventId/chats" element={<ChatsPage />} />
        <Route path="/events/:eventId/users" element={<UsersPage />} />
        <Route path="/events/:eventId/moderation" element={<ModerationPage />} />
        <Route path="/events/:eventId/export" element={<ExportPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/events" replace />} />
    </Routes>
  )
}

export function App(): React.ReactElement {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </ToastProvider>
    </BrowserRouter>
  )
}
