import type { Settings, Theme } from '../hooks/useSettings'

const THEME_OPTIONS: { value: Theme; label: string }[] = [
  { value: 'system', label: 'システム' },
  { value: 'light', label: 'ライト' },
  { value: 'dark', label: 'ダーク' },
]

type Props = {
  settings: Settings
  onChange: (patch: Partial<Settings>) => void
}

export function SettingsPage({ settings, onChange }: Props) {
  return (
    <main className="page page--narrow">
      <div>
        <h1 className="page-title">設定</h1>
      </div>

      <section className="panel" aria-labelledby="settings-appearance">
        <h2 id="settings-appearance" className="panel__title">
          表示
        </h2>
        <div className="settings-row">
          <div>
            <p id="settings-theme" className="settings-row__label">
              テーマ
            </p>
            <p className="settings-row__hint">「システム」は OS の外観設定に合わせます</p>
          </div>
          <div className="segmented" role="radiogroup" aria-labelledby="settings-theme">
            {THEME_OPTIONS.map(({ value, label }) => (
              <label key={value}>
                <input
                  className="visually-hidden"
                  type="radio"
                  name="theme"
                  value={value}
                  checked={settings.theme === value}
                  onChange={() => onChange({ theme: value })}
                />
                {label}
              </label>
            ))}
          </div>
        </div>
      </section>

      <section className="panel" aria-labelledby="settings-playback">
        <h2 id="settings-playback" className="panel__title">
          再生
        </h2>
        <label className="settings-row">
          <span>
            <span className="settings-row__label">自動再生</span>
            <br />
            <span className="settings-row__hint">動画を選択すると自動で再生を始めます（ブラウザによってはブロックされます）</span>
          </span>
          <input
            className="switch"
            type="checkbox"
            role="switch"
            checked={settings.autoplay}
            onChange={(e) => onChange({ autoplay: e.target.checked })}
          />
        </label>
      </section>
    </main>
  )
}
