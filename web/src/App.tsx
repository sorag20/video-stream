import { Navigate, Route, Routes } from 'react-router'
import { Sidebar } from './components/Sidebar'
import { useSettings } from './hooks/useSettings'
import { useVideos } from './hooks/useVideos'
import { SettingsPage } from './pages/SettingsPage'
import { UploadPage } from './pages/UploadPage'
import { VideosPage } from './pages/VideosPage'

export default function App() {
  // 動画一覧はページをまたいで保持する（ページを移動しても変換状況のポーリングが続き、戻ったときに即表示できる）
  const { videos, loading, error, refresh, addVideo } = useVideos()
  const { settings, updateSettings } = useSettings()

  return (
    <div className="app">
      <Sidebar />
      <Routes>
        <Route
          index
          element={
            <VideosPage videos={videos} loading={loading} error={error} refresh={refresh} autoplay={settings.autoplay} />
          }
        />
        <Route path="upload" element={<UploadPage onUploaded={addVideo} />} />
        <Route path="settings" element={<SettingsPage settings={settings} onChange={updateSettings} />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  )
}
