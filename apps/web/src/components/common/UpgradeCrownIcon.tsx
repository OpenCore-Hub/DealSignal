export function UpgradeCrownIcon({ size = 16 }: { size?: number }) {
  return (
    <svg viewBox="0 0 20 20" width={size} height={size} aria-hidden className="overflow-visible">
      <path
        fill="#F4C430"
        d="M10 2.6 12.15 8.1 18 6.2 15.7 13.2H4.3L2 6.2l5.85 1.9L10 2.6Z"
      />
      <path fill="#E0B01E" d="M4.15 13.05h11.7v2.35c0 .7-2.6 1.4-5.85 1.4s-5.85-.7-5.85-1.4v-2.35Z" />
      <circle cx="10" cy="2.85" r="1.25" fill="#E11D48" />
      <circle cx="3.55" cy="6.15" r="1.15" fill="#22C55E" />
      <circle cx="16.45" cy="6.15" r="1.15" fill="#3B82F6" />
      <circle cx="7.15" cy="8.15" r="0.7" fill="#F8FAFC" />
      <circle cx="12.85" cy="8.15" r="0.7" fill="#F8FAFC" />
    </svg>
  );
}
