import type { Video } from './types'

// API は常に同一オリジンで呼び出す（dev: Vite proxy / docker compose: nginx / k8s: Gateway API）。
// Upload Service は CORS ヘッダを返さないため、別オリジンからは呼び出せない。

// Upload Service の上限 (services/upload/handler: maxUploadSize) と合わせる
export const MAX_UPLOAD_BYTES = 2 * 1024 ** 3

export function streamUrl(id: string): string {
  return `/stream/${encodeURIComponent(id)}/index.m3u8`
}

export async function listVideos(): Promise<Video[]> {
  const res = await fetch('/videos')
  if (!res.ok) {
    throw new Error(await readError(res))
  }
  return res.json()
}

type UploadOptions = {
  onProgress?: (ratio: number) => void
  signal?: AbortSignal
}

// fetch はアップロード進捗を取得できないため XMLHttpRequest を使う
export function uploadVideo(title: string, file: File, { onProgress, signal }: UploadOptions = {}): Promise<Video> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason)
      return
    }

    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/videos')
    xhr.responseType = 'json'

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(e.loaded / e.total)
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(xhr.response as Video)
        return
      }
      const message = (xhr.response as { error?: string } | null)?.error
      reject(new Error(message ?? `アップロードに失敗しました (HTTP ${xhr.status})`))
    }
    xhr.onerror = () => reject(new Error('ネットワークエラーが発生しました'))
    xhr.onabort = () => reject(new DOMException('アップロードを中止しました', 'AbortError'))

    signal?.addEventListener('abort', () => xhr.abort(), { once: true })

    const form = new FormData()
    form.append('title', title)
    form.append('file', file)
    xhr.send(form)
  })
}

async function readError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    // JSON 以外のレスポンス（nginx のエラーページなど）
  }
  return `HTTP ${res.status}`
}
