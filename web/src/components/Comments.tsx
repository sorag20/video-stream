import { useEffect, useState, type FormEvent, type KeyboardEvent } from 'react'
import { listComments, MAX_COMMENT_AUTHOR_LENGTH, MAX_COMMENT_BODY_LENGTH, postComment } from '../api'
import { errorMessage, formatDate } from '../format'
import type { Comment } from '../types'
import { AlertIcon } from './icons'

const AUTHOR_STORAGE_KEY = 'video-stream:comment-author'

// 名前はブラウザ (localStorage) に覚えておき、次回の投稿時に入力済みにする
function loadAuthor(): string {
  try {
    return localStorage.getItem(AUTHOR_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveAuthor(author: string) {
  try {
    localStorage.setItem(AUTHOR_STORAGE_KEY, author)
  } catch {
    // 保存できなくても投稿自体には影響しない
  }
}

// videoId が変わったら親側で key を変えて再マウントすること（一覧と入力内容をリセットするため）
export function Comments({ videoId }: { videoId: string }) {
  const [comments, setComments] = useState<Comment[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  const [author, setAuthor] = useState(loadAuthor)
  const [body, setBody] = useState('')
  const [posting, setPosting] = useState(false)
  const [postError, setPostError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    listComments(videoId, controller.signal)
      .then((list) => {
        // 取得中に投稿されたコメントが消えないよう、取得結果に含まれないものは残す
        setComments((prev) => {
          const ids = new Set(list.map((c) => c.id))
          return [...(prev ?? []).filter((c) => !ids.has(c.id)), ...list]
        })
      })
      .catch((err: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(err))
      })
    return () => controller.abort()
  }, [videoId])

  const trimmedBody = body.trim()
  const canSubmit = trimmedBody !== '' && !posting

  async function submit() {
    if (!canSubmit) return
    setPosting(true)
    setPostError(null)
    try {
      const comment = await postComment(videoId, author.trim(), trimmedBody)
      setComments((prev) => [comment, ...(prev ?? [])])
      setBody('')
      saveAuthor(author.trim())
    } catch (err) {
      setPostError(errorMessage(err))
    } finally {
      setPosting(false)
    }
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    void submit()
  }

  // Ctrl / ⌘ + Enter で投稿する（日本語入力の変換確定の Enter では投稿しない）
  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !e.nativeEvent.isComposing) {
      e.preventDefault()
      void submit()
    }
  }

  return (
    <section className="comments" aria-label="コメント">
      <h3 className="comments__title">
        コメント
        {comments && <span className="comments__count">{comments.length} 件</span>}
      </h3>

      <form className="comment-form" onSubmit={handleSubmit}>
        <input
          className="input comment-form__author"
          value={author}
          onChange={(e) => setAuthor(e.target.value)}
          placeholder="名前（省略すると「匿名」）"
          aria-label="名前"
          maxLength={MAX_COMMENT_AUTHOR_LENGTH}
          autoComplete="nickname"
          disabled={posting}
        />
        <textarea
          className="input textarea"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="コメントを書く…"
          aria-label="コメント"
          maxLength={MAX_COMMENT_BODY_LENGTH}
          disabled={posting}
        />
        <div className="comment-form__footer">
          {postError && (
            <p className="form-error" role="alert">
              コメントを投稿できませんでした: {postError}
            </p>
          )}
          <span className="comment-form__counter">
            {body.length} / {MAX_COMMENT_BODY_LENGTH}
          </span>
          <button type="submit" className="btn btn--primary btn--sm" disabled={!canSubmit}>
            {posting ? '投稿中…' : '投稿'}
          </button>
        </div>
      </form>

      {loadError && (
        <div className="alert" role="alert">
          <AlertIcon size={16} />
          コメントを取得できませんでした: {loadError}
        </div>
      )}

      {comments === null ? (
        !loadError && <p className="muted">読み込み中…</p>
      ) : comments.length === 0 ? (
        <p className="muted">まだコメントはありません。最初のコメントを書いてみましょう。</p>
      ) : (
        <ul className="comment-list">
          {comments.map((c) => (
            <li key={c.id} className="comment">
              <div className="comment__head">
                <span className="comment__author">{c.author}</span>
                <time className="comment__date" dateTime={c.created_at}>
                  {formatDate(c.created_at)}
                </time>
              </div>
              <p className="comment__body">{c.body}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
