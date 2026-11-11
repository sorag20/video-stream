import { useState } from 'react'
import { Link, NavLink } from 'react-router'
import { GridIcon, PlayIcon, SidebarIcon, SlidersIcon, UploadIcon } from './icons'

// 注意: /videos と /stream は API に振り分けられるため、ページのパスには使えない
const NAV_ITEMS = [
  { to: '/', label: '動画一覧', icon: GridIcon },
  { to: '/upload', label: 'アップロード', icon: UploadIcon },
  { to: '/settings', label: '設定', icon: SlidersIcon },
]

const COLLAPSED_KEY = 'video-stream:sidebar-collapsed'

function loadCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

export function Sidebar() {
  // 最小化の状態はリロード後も保つ
  const [collapsed, setCollapsed] = useState(loadCollapsed)

  function toggle() {
    const next = !collapsed
    setCollapsed(next)
    try {
      localStorage.setItem(COLLAPSED_KEY, next ? '1' : '0')
    } catch {
      // 保存できなくても、このセッション中は有効
    }
  }

  return (
    <aside className={`sidebar${collapsed ? ' sidebar--collapsed' : ''}`}>
      <div className="sidebar__header">
        <Link to="/" className="brand" aria-label="video-stream">
          <span className="brand__logo">
            <PlayIcon size={14} />
          </span>
          <span className="brand__name">video-stream</span>
        </Link>
        <button
          type="button"
          className="icon-btn sidebar__toggle"
          onClick={toggle}
          aria-expanded={!collapsed}
          aria-controls="main-nav"
          aria-label={collapsed ? 'メニューを開く' : 'メニューを最小化'}
          title={collapsed ? 'メニューを開く' : 'メニューを最小化'}
        >
          <SidebarIcon size={18} />
        </button>
      </div>

      <nav id="main-nav" className="nav" aria-label="メインメニュー">
        {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
          // 最小化中はラベルが見えないため、aria-label と title で項目名を補う
          <NavLink key={to} to={to} end className="nav__link" aria-label={label} title={collapsed ? label : undefined}>
            <Icon size={18} />
            <span className="nav__label">{label}</span>
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
