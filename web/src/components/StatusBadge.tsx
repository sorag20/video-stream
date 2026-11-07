import type { VideoStatus } from '../types'

const LABELS: Record<VideoStatus, string> = {
  pending: '変換待ち',
  processing: '変換中',
  ready: '再生可能',
  failed: '変換失敗',
}

export function StatusBadge({ status }: { status: VideoStatus }) {
  return <span className={`badge badge--${status}`}>{LABELS[status]}</span>
}
