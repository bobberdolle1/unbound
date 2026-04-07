import { type ConnectionStatus } from '../types';

interface ConnectToggleProps {
  status: ConnectionStatus;
  onToggle: (connected: boolean) => void;
}

export function ConnectToggle({ status, onToggle }: ConnectToggleProps) {
  const isConnected = status === 'connected' || status === 'connecting';
  const isConnecting = status === 'connecting';

  return (
    <div className="flex flex-col items-center py-4">
      <button
        onClick={() => !isConnecting && onToggle(!isConnected)}
        disabled={isConnecting}
        className={`
          relative w-32 h-32 rounded-full transition-all duration-300 ease-in-out
          flex items-center justify-center
          focus:outline-none focus:ring-4 focus:ring-[var(--color-accent)] focus:ring-opacity-50
          ${isConnected 
            ? 'bg-gradient-to-br from-green-400 to-green-600 shadow-lg shadow-green-500/50' 
            : 'bg-gradient-to-br from-gray-400 to-gray-600 dark:from-gray-600 dark:to-gray-800 shadow-lg shadow-gray-500/30'
          }
          ${isConnecting ? 'animate-pulse-slow cursor-wait' : 'cursor-pointer hover:scale-105 active:scale-95'}
          ${isConnected ? 'hover:shadow-green-500/70' : 'hover:shadow-gray-500/50'}
        `}
        aria-label={isConnected ? 'Disconnect' : 'Connect'}
      >
        {/* Inner circle */}
        <div className={`
          w-24 h-24 rounded-full border-4 border-white/30 
          flex items-center justify-center
          transition-all duration-300
          ${isConnected ? 'bg-white/20' : 'bg-black/10'}
        `}>
          {/* Power icon */}
          <svg 
            className={`w-12 h-12 text-white transition-transform duration-300 ${isConnected ? 'rotate-0' : '-rotate-12'}`}
            fill="none" 
            stroke="currentColor" 
            viewBox="0 0 24 24"
          >
            {isConnected ? (
              // Disconnect icon (square/stop)
              <path 
                strokeLinecap="round" 
                strokeLinejoin="round" 
                strokeWidth={2} 
                d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" 
              />
            ) : (
              // Connect icon (power)
              <>
                <path 
                  strokeLinecap="round" 
                  strokeLinejoin="round" 
                  strokeWidth={2} 
                  d="M13 10V3L4 14h7v7l9-11h-7z" 
                />
              </>
            )}
          </svg>
        </div>

        {/* Status ring animation */}
        {isConnecting && (
          <div className="absolute inset-0 rounded-full">
            <div className="absolute inset-0 rounded-full border-4 border-[var(--color-accent)] animate-ping opacity-75"></div>
          </div>
        )}
      </button>

      {/* Status text */}
      <p className={`
        mt-4 text-sm font-semibold transition-colors duration-300
        ${isConnected ? 'text-green-500' : 'text-[var(--text-muted)]'}
      `}>
        {isConnected ? 'CONNECTED' : isConnecting ? 'CONNECTING...' : 'DISCONNECTED'}
      </p>
    </div>
  );
}
