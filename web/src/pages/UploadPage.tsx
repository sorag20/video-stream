import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router'
import { UploadForm } from '../components/UploadForm'
import type { Video } from '../types'

export function UploadPage({ onUploaded }: { onUploaded: (video: Video) => void }) {
  const navigate = useNavigate()
  const mountedRef = useRef(true)
  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  function handleUploaded(video: Video) {
    onUploaded(video)
    // アップロードした動画を選択した状態で一覧へ移ると、変換完了と同時にそのまま再生できる。
    // アップロード中に別ページへ移動していた場合は、いま見ているページから勝手に遷移させない
    if (mountedRef.current) {
      void navigate(`/?v=${encodeURIComponent(video.id)}`)
    }
  }

  return (
    <main className="page page--narrow">
      <div>
        <h1 className="page-title">アップロード</h1>
      </div>

      <section className="panel">
        <UploadForm onUploaded={handleUploaded} />
      </section>
    </main>
  )
}
