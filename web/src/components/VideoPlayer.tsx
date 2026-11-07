// 単一レンディション・音声多重化済みの HLS しか扱わないため light ビルドで十分
import Hls from 'hls.js/light'
import { useEffect, useRef, useState } from 'react'
import { AlertIcon } from './icons'

const supportsNativeHls = document.createElement('video').canPlayType('application/vnd.apple.mpegurl') !== ''

// src が変わったら親側で key を変えて再マウントすること（エラー状態をリセットするため）
export default function VideoPlayer({ src }: { src: string }) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    if (Hls.isSupported()) {
      const hls = new Hls()
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
        if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
          hls.recoverMediaError()
          return
        }
        setError(`再生に失敗しました (${data.details})`)
      })
      hls.loadSource(src)
      hls.attachMedia(video)
      return () => hls.destroy()
    }

    // Safari など Media Source Extensions 非対応でネイティブ HLS 再生できるブラウザ
    video.src = src
    return () => {
      video.removeAttribute('src')
      video.load()
    }
  }, [src])

  if (!Hls.isSupported() && !supportsNativeHls) {
    return (
      <div className="screen screen--placeholder">
        <div className="screen__message">
          <AlertIcon size={28} />
          <p>このブラウザは HLS 再生に対応していません</p>
        </div>
      </div>
    )
  }

  return (
    <div className="screen">
      <video ref={videoRef} controls playsInline preload="metadata" />
      {error && (
        <div className="screen__error" role="alert">
          <AlertIcon size={16} />
          {error}
        </div>
      )}
    </div>
  )
}
