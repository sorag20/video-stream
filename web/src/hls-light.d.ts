// hls.js の light ビルド（字幕・代替音声・DRM 非対応）には型定義が同梱されていないため、フルビルドの型を流用する
declare module 'hls.js/light' {
  export * from 'hls.js'
  export { default } from 'hls.js'
}
