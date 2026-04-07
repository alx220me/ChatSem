/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_WIDGET_BASE_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
