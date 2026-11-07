import type { CSSProperties } from 'react'
import { formatDate } from '../format'
import type { Video } from '../types'
import { AlertIcon, ClockIcon, PlayIcon, UploadIcon } from './icons'
import { StatusBadge } from './StatusBadge'

type Props = {
  videos: Video[]
  selectedId: string | null
  onSelect: (id: string) => void
}

export function VideoList({ videos, selectedId, onSelect }: Props) {
  if (videos.length === 0) {
    return (
      <div className="empty">
        <UploadIcon size={28} />
        <p>まだ動画がありません。最初の動画をアップロードしてみましょう。</p>
      </div>
    )
  }

  return (
    <ul className="video-grid">
      {videos.map((video) => (
        <li key={video.id}>
          <VideoCard video={video} selected={video.id === selectedId} onSelect={() => onSelect(video.id)} />
        </li>
      ))}
    </ul>
  )
}

function VideoCard({ video, selected, onSelect }: { video: Video; selected: boolean; onSelect: () => void }) {
  // サムネイルが無いため、ID から決まる色のグラデーションで見分けやすくする
  const thumbStyle = { '--hue': hueFromId(video.id) } as CSSProperties

  return (
    <button type="button" className="video-card" aria-current={selected} onClick={onSelect}>
      <div className="video-card__thumb" style={thumbStyle}>
        <span className="video-card__thumb-icon">
          <ThumbIcon status={video.status} />
        </span>
      </div>
      <div className="video-card__body">
        <span className="video-card__title" title={video.title}>
          {video.title}
        </span>
        <span className="video-card__meta">
          <StatusBadge status={video.status} />
          <time dateTime={video.created_at}>{formatDate(video.created_at)}</time>
        </span>
      </div>
    </button>
  )
}

function ThumbIcon({ status }: { status: Video['status'] }) {
  switch (status) {
    case 'ready':
      return <PlayIcon size={20} />
    case 'processing':
      return <span className="spinner" />
    case 'pending':
      return <ClockIcon size={20} />
    case 'failed':
      return <AlertIcon size={20} />
  }
}

function hueFromId(id: string): number {
  let hash = 0
  for (const ch of id) {
    hash = (hash * 31 + ch.charCodeAt(0)) % 360
  }
  return hash
}
