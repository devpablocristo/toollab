export default function Spinner({ className = '' }: { className?: string }) {
  return (
    <div className={`relative inline-flex items-center justify-center ${className}`}>
      <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="2" className="text-edge" />
        <path d="M12 2a10 10 0 019.8 8" stroke="url(#spinner-grad)" strokeWidth="2" strokeLinecap="round" />
        <defs>
          <linearGradient id="spinner-grad" x1="12" y1="2" x2="22" y2="10">
            <stop stopColor="#00ffaa" />
            <stop offset="1" stopColor="#00cc88" stopOpacity="0.3" />
          </linearGradient>
        </defs>
      </svg>
    </div>
  )
}
