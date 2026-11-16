import { useSearchParams } from 'react-router'
import { AlertIcon } from '../components/icons'
import { PlayerPanel } from '../components/PlayerPanel'
import { VideoList } from '../components/VideoList'
import type { Video } from '../types'

type Props = {
  videos: Video[]
  loading: boolean
  error: string | null
  autoplay: boolean
  onDeleted: (id: string) => void
}

export function VideosPage({ videos, loading, error, autoplay, onDeleted }: Props) {
  // 選択中の動画は URL (?v=<id>) で持つ。アップロード完了後の遷移やリロードでも選択が保たれる
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedId = searchParams.get('v')
  const selected = videos.find((v) => v.id === selectedId)

  function handleSelect(id: string) {
    setSearchParams({ v: id })
    // プレイヤーは一覧の上に出るため、スクロール位置を先頭へ戻す
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function handleDeleted(id: string) {
    onDeleted(id)
    // 削除した動画の選択を外す（履歴には残さず、「戻る」で消えた動画の URL に戻らないようにする）
    setSearchParams({}, { replace: true })
  }

  return (
    <main className="page">
      <div className="page-header">
        <h1 className="page-title">
          動画一覧
        </h1>
      </div>

      {selected && (
        <section className="stage" aria-label="プレイヤー">
          <PlayerPanel video={selected} autoplay={autoplay} onDeleted={handleDeleted} />
        </section>
      )}

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
          <VideoList videos={videos} selectedId={selectedId} onSelect={handleSelect} />
        )
      )}
    </main>
  )
}
