export function AlertBadge({ name, color }: { name: string; color: string }) {
  return (
    <span
      className="alert-badge"
      title={name}
      style={{ color, backgroundColor: `${color}20`, borderColor: `${color}45` }}
    >
      {name}
    </span>
  )
}
