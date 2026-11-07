import { useCallback, useEffect, useState } from 'react'
import { listVideos } from '../api'
import { errorMessage } from '../format'
import type { Video } from '../types'

const POLL_INTERVAL_MS = 3000

export function useVideos() {
  const [videos, setVideos] = useState<Video[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(
    () =>
      listVideos()
        .then((list) => {
          setVideos(list)
          setError(null)
        })
        .catch((err: unknown) => setError(errorMessage(err)))
        .finally(() => setLoading(false)),
    [],
  )

  // アップロード直後の動画を一覧へ即時反映する
  const addVideo = useCallback((video: Video) => {
    setVideos((prev) => [video, ...prev.filter((v) => v.id !== video.id)])
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // 変換待ち・変換中の動画がある間だけポーリングする
  const hasInFlight = videos.some((v) => v.status === 'pending' || v.status === 'processing')
  useEffect(() => {
    if (!hasInFlight) return
    const timer = setInterval(() => void refresh(), POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [hasInFlight, refresh])

  return { videos, loading, error, refresh, addVideo }
}
