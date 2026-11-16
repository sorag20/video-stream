import { lazy, Suspense, useState } from 'react'
import { deleteVideo, streamUrl } from '../api'
import { errorMessage, formatDate } from '../format'
import type { Video } from '../types'
import { Comments } from './Comments'
import { AlertIcon, CheckIcon, ClockIcon, CopyIcon, TrashIcon } from './icons'
import { StatusBadge } from './StatusBadge'

// hls.js はサイズが大きいため、再生可能な動画を選択したときに初めて読み込む
const VideoPlayer = lazy(() => import('./VideoPlayer'))

type Props = {
  video: Video
  autoplay: boolean
  onDeleted: (id: string) => void
}

export function PlayerPanel({ video, autoplay, onDeleted }: Props) {
  return (
    <>
      <Screen video={video} autoplay={autoplay} />
      <div className="stage__meta">
        <h2 className="stage__title">{video.title}</h2>
        <div className="stage__sub">
          <StatusBadge status={video.status} />
          <span>{formatDate(video.created_at)}</span>
          <span className="mono">{video.id}</span>
        </div>
        {/* key で動画ごとに再マウントし、削除中・エラーの表示を持ち越さない */}
        <DeleteButton key={video.id} video={video} onDeleted={onDeleted} />
        {video.status === 'ready' && <UrlField url={new URL(streamUrl(video.id), window.location.origin).href} />}
      </div>
      {/* コメントは再生できる動画にだけ付けられる。key で動画ごとに再マウントし、一覧と入力中の本文をリセットする */}
      {video.status === 'ready' && <Comments key={video.id} videoId={video.id} />}
    </>
  )
}

function Screen({ video, autoplay }: { video: Video; autoplay: boolean }) {
  switch (video.status) {
    case 'ready':
      return (
        <Suspense fallback={<div className="screen" />}>
          <VideoPlayer key={video.id} src={streamUrl(video.id)} autoplay={autoplay} />
        </Suspense>
      )
    case 'failed':
      return (
        <div className="screen screen--placeholder">
          <div className="screen__message screen__message--failed">
            <AlertIcon size={32} />
            <p className="screen__headline">変換に失敗しました</p>
            {video.error_msg && <p className="screen__detail">{video.error_msg}</p>}
          </div>
        </div>
      )
    default:
      return (
        <div className="screen screen--placeholder">
          <div className="screen__message">
            {video.status === 'pending' ? <ClockIcon size={32} /> : <span className="spinner spinner--lg" />}
            <p className="screen__headline">
              {video.status === 'pending' ? 'トランスコードの順番待ちです' : 'HLS に変換しています'}
            </p>
            <p className="screen__detail">変換が完了すると、自動で再生できるようになります</p>
          </div>
        </div>
      )
  }
}

// 変換待ち・変換中・失敗した動画も削除できる（変換中の場合は Transcoder 側で後片付けされる）
function DeleteButton({ video, onDeleted }: { video: Video; onDeleted: (id: string) => void }) {
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleDelete() {
    if (!window.confirm(`「${video.title}」を削除しますか？\n動画とコメントが完全に削除され、元に戻せません。`)) return
    setDeleting(true)
    setError(null)
    try {
      await deleteVideo(video.id)
      onDeleted(video.id)
    } catch (err) {
      setError(errorMessage(err))
      setDeleting(false)
    }
  }

  return (
    <div className="stage__actions">
      <button type="button" className="btn btn--danger btn--sm" onClick={handleDelete} disabled={deleting}>
        <TrashIcon size={14} />
        {deleting ? '削除中…' : '削除'}
      </button>
      {error && (
        <p className="form-error" role="alert">
          動画を削除できませんでした: {error}
        </p>
      )}
    </div>
  )
}

function UrlField({ url }: { url: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // クリップボードへのアクセスが拒否された場合は何もしない
    }
  }

  return (
    <div className="url-field">
      <span className="url-field__label">HLS</span>
      <code title={url}>{url}</code>
      {/* Clipboard API は secure context (https / localhost) でのみ利用できる */}
      {window.isSecureContext && (
        <button type="button" className="btn btn--ghost btn--sm" onClick={copy}>
          {copied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
          {copied ? 'コピーしました' : 'コピー'}
        </button>
      )}
    </div>
  )
}
