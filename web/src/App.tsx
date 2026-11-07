import { useState } from 'react'
import { AlertIcon, PlayIcon, RefreshIcon } from './components/icons'
import { PlayerPanel } from './components/PlayerPanel'
import { UploadForm } from './components/UploadForm'
import { VideoList } from './components/VideoList'
import { useVideos } from './hooks/useVideos'
import type { Video } from './types'

export default function App() {
  const { videos, loading, error, refresh, addVideo } = useVideos()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const selected = videos.find((v) => v.id === selectedId)

  // アップロードした動画を選択しておくと、変換完了と同時にそのまま再生できる
  function handleUploaded(video: Video) {
    addVideo(video)
    setSelectedId(video.id)
  }

  return (
    <>
      <header className="app-header">
        <h1 className="brand">
          <span className="brand__logo">
            <PlayIcon size={14} />
          </span>
          video-stream
        </h1>
        <span className="brand__tags">Go · Kubernetes · HLS</span>
      </header>

      <main className="layout">
        <section className="stage" aria-label="プレイヤー">
          <PlayerPanel video={selected} />
        </section>

        <section className="panel upload-panel" aria-labelledby="upload-title">
          <h2 id="upload-title" className="panel__title">
            アップロード
          </h2>
          <UploadForm onUploaded={handleUploaded} />
        </section>

        <section className="library" aria-labelledby="library-title">
          <div className="library__header">
            <h2 id="library-title" className="library__title">
              ライブラリ
              <span className="library__count">{videos.length}</span>
            </h2>
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => void refresh()}>
              <RefreshIcon size={14} />
              更新
            </button>
          </div>

          {error && (
            <div className="alert" role="alert">
              <AlertIcon size={16} />
              動画一覧を取得できませんでした: {error}
            </div>
          )}

          {loading ? (
            <p className="muted">読み込み中…</p>
          ) : (
            // 取得失敗時に「動画がありません」と誤表示しないよう、エラーかつ空なら一覧を出さない
            !(error && videos.length === 0) && (
              <VideoList videos={videos} selectedId={selectedId} onSelect={setSelectedId} />
            )
          )}
        </section>
      </main>
    </>
  )
}
