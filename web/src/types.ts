// pkg/models/video.go と対応する型定義

export type VideoStatus = 'pending' | 'processing' | 'ready' | 'failed'

export interface Video {
  id: string
  title: string
  original_key: string
  hls_key: string
  status: VideoStatus
  error_msg?: string
  created_at: string
  updated_at: string
}
