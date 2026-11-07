import { useRef, useState, type DragEvent, type FormEvent } from 'react'
import { MAX_UPLOAD_BYTES, uploadVideo } from '../api'
import { errorMessage, formatBytes } from '../format'
import type { Video } from '../types'
import { FilmIcon, UploadIcon, XIcon } from './icons'

export function UploadForm({ onUploaded }: { onUploaded: (video: Video) => void }) {
  const [title, setTitle] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [dragging, setDragging] = useState(false)
  const [progress, setProgress] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  const uploading = progress !== null

  function selectFile(next: File | undefined) {
    if (!next) return
    // 拡張子によっては MIME タイプが空になるため、空の場合はサーバー側 (FFmpeg) の判定に任せる
    if (next.type && !next.type.startsWith('video/')) {
      setError('動画ファイルを選択してください')
      return
    }
    if (next.size > MAX_UPLOAD_BYTES) {
      setError(`ファイルサイズが上限 (${formatBytes(MAX_UPLOAD_BYTES)}) を超えています`)
      return
    }
    setError(null)
    setFile(next)
    // タイトル未入力ならファイル名（拡張子なし）で補完する
    setTitle((current) => current || next.name.replace(/\.[^.]+$/, ''))
  }

  function clearFile() {
    setFile(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  function handleDrop(e: DragEvent<HTMLLabelElement>) {
    e.preventDefault()
    setDragging(false)
    if (!uploading) selectFile(e.dataTransfer.files[0])
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!file || !title.trim()) return

    const controller = new AbortController()
    abortRef.current = controller
    setError(null)
    setProgress(0)
    try {
      const video = await uploadVideo(title.trim(), file, {
        onProgress: setProgress,
        signal: controller.signal,
      })
      setTitle('')
      clearFile()
      onUploaded(video)
    } catch (err) {
      if (!controller.signal.aborted) setError(errorMessage(err))
    } finally {
      abortRef.current = null
      setProgress(null)
    }
  }

  return (
    <form className="upload-form" onSubmit={handleSubmit}>
      <label className="field">
        <span className="field__label">タイトル</span>
        <input
          className="input"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="My Video"
          disabled={uploading}
          required
        />
      </label>

      {file ? (
        <div className="file-chip">
          <FilmIcon size={18} />
          <span className="file-chip__name" title={file.name}>
            {file.name}
          </span>
          <span className="file-chip__size">{formatBytes(file.size)}</span>
          {!uploading && (
            <button type="button" className="icon-btn" onClick={clearFile} aria-label="ファイルの選択を解除">
              <XIcon size={16} />
            </button>
          )}
        </div>
      ) : (
        <label
          className={`dropzone${dragging ? ' dropzone--active' : ''}`}
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={handleDrop}
        >
          <input
            ref={fileInputRef}
            className="visually-hidden"
            type="file"
            accept="video/*"
            onChange={(e) => selectFile(e.target.files?.[0])}
          />
          <UploadIcon size={28} />
          <span className="dropzone__primary">動画ファイルをドロップ</span>
          <span>またはクリックして選択（最大 {formatBytes(MAX_UPLOAD_BYTES)}）</span>
        </label>
      )}

      {uploading && (
        <div className="progress-block" aria-live="polite">
          <div className="progress-row">
            <span>{progress < 1 ? 'アップロード中…' : 'サーバーで保存中…'}</span>
            <span>{Math.round(progress * 100)}%</span>
          </div>
          <div className="progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress * 100)}>
            <div className="progress__bar" style={{ width: `${progress * 100}%` }} />
          </div>
        </div>
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <div className="upload-actions">
        <button type="submit" className="btn btn--primary" disabled={!file || !title.trim() || uploading}>
          <UploadIcon size={16} />
          アップロード
        </button>
        {uploading && (
          <button type="button" className="btn btn--ghost" onClick={() => abortRef.current?.abort()}>
            キャンセル
          </button>
        )}
      </div>
    </form>
  )
}
