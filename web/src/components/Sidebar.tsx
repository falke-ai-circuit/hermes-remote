import { NavLink } from 'react-router-dom'
import { clearToken } from '../api/client'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '◉' },
  { to: '/agents', label: 'Agents', icon: '🖥' },
  { to: '/builds', label: 'Builder', icon: '🔧' },
  { to: '/profiles', label: 'Profiles', icon: '📋' },
  { to: '/tasks', label: 'Tasks', icon: '⏰' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
]

export default function Sidebar() {
  const handleLogout = () => {
    clearToken()
    window.location.reload()
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-logo">PROBE</div>
      <nav className="sidebar-nav">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) => isActive ? 'active' : ''}
          >
            <span className="nav-icon">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
      </nav>
      <div className="sidebar-footer">
        <button className="btn btn-sm logout-btn" onClick={handleLogout}>
          ⏻ Logout
        </button>
      </div>
    </aside>
  )
}