interface NavigationProps {
  currentPage: string
  onNavigate: (page: string) => void
}

export default function Navigation({ currentPage, onNavigate }: NavigationProps) {
  return (
    <header className="site-shell">
      <nav className="site-nav" aria-label="Разделы приложения">
        <a href="#" className="site-nav__brand" onClick={(e) => { e.preventDefault(); onNavigate('dashboard'); }}>
          <span className="site-nav__mark" aria-hidden="true"></span>
          <span className="site-nav__wordmark">Twitch Bot Farm</span>
        </a>
        <ul className="site-nav__list">
          <li>
            <a
              href="#"
              className={`site-nav__link${currentPage === 'dashboard' ? ' is-active' : ''}`}
              onClick={(e) => { e.preventDefault(); onNavigate('dashboard'); }}
            >
              Дашборд
            </a>
          </li>
          <li>
            <a
              href="#"
              className={`site-nav__link site-nav__link--featured${currentPage === 'bots' ? ' is-active' : ''}`}
              onClick={(e) => { e.preventDefault(); onNavigate('bots'); }}
            >
              Боты
            </a>
          </li>
          <li>
            <a
              href="#"
              className={`site-nav__link${currentPage === 'settings' ? ' is-active' : ''}`}
              onClick={(e) => { e.preventDefault(); onNavigate('settings'); }}
            >
              Настройки
            </a>
          </li>
        </ul>
      </nav>
    </header>
  )
}
