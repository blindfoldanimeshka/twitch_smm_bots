import { LayoutDashboard, Bot, Settings } from "lucide-react";

interface DockProps {
  currentPage: string;
  onNavigate: (page: string) => void;
}

const navItems = [
  { label: "dashboard", page: "dashboard", icon: LayoutDashboard },
  { label: "bots", page: "bots", icon: Bot },
  { label: "settings", page: "settings", icon: Settings }
];

export function Dock({ currentPage, onNavigate }: DockProps) {
  return (
    <div className="fixed bottom-8 left-1/2 transform -translate-x-1/2 z-50">
      <div className="relative flex items-center gap-2 sm:gap-4 scale-90 sm:scale-95">
        <div className="flex items-center gap-3 bg-neutral-900/80 px-3 py-2 shadow-2xl ring-1 ring-white/10 backdrop-blur-lg sm:gap-5 sm:px-6 sm:py-3">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = currentPage === item.page;
            return (
              <button
                key={item.page}
                className={`cursor-target hover-halo group relative grid h-12 w-12 place-items-center ring-1 backdrop-blur-xl shadow-lg transition-all duration-200 hover:-translate-y-1 hover:scale-[1.05] sm:h-14 sm:w-14 ${
                  isActive
                    ? 'ring-white/30 bg-gradient-to-b from-neutral-700/80 to-neutral-800/90'
                    : 'ring-white/10 bg-gradient-to-b from-neutral-800/60 to-neutral-900/70'
                }`}
                aria-label={item.label}
                onClick={() => onNavigate(item.page)}
              >
                <Icon
                  className={`transition-colors duration-200 ${
                    isActive ? 'text-white' : 'text-neutral-400'
                  }`}
                  size={20}
                />
              </button>
            );
          })}
        </div>
      </div>

      <style>{`
        .hover-halo{position:relative}
        .hover-halo::after{content:"";position:absolute;inset:-2px;border-radius:inherit;opacity:0;transition:opacity .25s, transform .25s;box-shadow:0 0 0 0 rgba(255,255,255,.18),0 12px 30px -10px rgba(0,0,0,.7)}
        .hover-halo:hover::after{opacity:1;}
      `}</style>
    </div>
  );
}
