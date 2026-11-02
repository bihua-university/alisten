import type { LucideIcon } from 'lucide-react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Ellipsis, Music2, X } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { registerModalHistory } from '@/utils/modal-history'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: LucideIcon
  label: string
}

export function IconButton({
  icon: Icon,
  label,
  className = '',
  ...props
}: IconButtonProps) {
  return (
    <button
      type="button"
      className={`icon-button ${className}`}
      aria-label={label}
      title={label}
      {...props}
    >
      <Icon size={20} strokeWidth={1.7} aria-hidden="true" />
    </button>
  )
}

export function AsyncIconButton({
  onAction,
  disabled,
  icon,
  ...props
}: Omit<IconButtonProps, 'onClick'> & { onAction: () => Promise<void> }) {
  const [pending, setPending] = useState(false)
  const pendingRef = useRef(false)
  return (
    <IconButton
      {...props}
      icon={pending ? Ellipsis : icon}
      disabled={disabled || pending}
      aria-busy={pending}
      onClick={async () => {
        if (disabled || pendingRef.current)
          return
        pendingRef.current = true
        setPending(true)
        try {
          await onAction()
        } finally {
          pendingRef.current = false
          setPending(false)
        }
      }}
    />
  )
}

export function Artwork({
  src,
  title,
  className = '',
}: {
  src?: string
  title: string
  className?: string
}) {
  const [failedSrc, setFailedSrc] = useState<string>()
  return (
    <div className={`artwork ${className}`}>
      {src && src !== failedSrc
        ? (
            <img
              src={src}
              alt={title}
              loading="lazy"
              decoding="async"
              onError={() => setFailedSrc(src)}
            />
          )
        : (
            <Music2 size={40} strokeWidth={1.2} aria-hidden="true" />
          )}
    </div>
  )
}

export function EmptyState({
  icon: Icon = Music2,
  title,
  children,
  action,
}: {
  icon?: LucideIcon
  title: string
  children?: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="empty-state">
      <Icon size={36} strokeWidth={1.2} aria-hidden="true" />
      <h3 className="text-ui-lg">{title}</h3>
      <div className="text-ui-caption muted">{children}</div>
      {action}
    </div>
  )
}

export function Skeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="skeleton-list" role="status" aria-label="正在加载">
      {Array.from({ length: rows }, (_, i) => (
        <div className="skeleton-row" key={i}>
          <span />
          <div>
            <span />
            <span />
          </div>
        </div>
      ))}
    </div>
  )
}

/** Native dialog supplies focus containment, Escape, and an inert background. */
export function Modal({
  title,
  children,
  onClose,
  wide = false,
  className = '',
}: {
  title: string
  children: ReactNode
  onClose: () => void
  wide?: boolean
  className?: string
}) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  useEffect(() => {
    const dialog = ref.current!
    const previousFocus = document.activeElement as HTMLElement | null
    dialog.showModal()
    const unregister = registerModalHistory(() => closeRef.current())
    return () => {
      unregister()
      dialog.close()
      previousFocus?.focus()
    }
  }, [])
  return (
    <dialog
      ref={ref}
      className={`modal ${wide ? 'modal-wide' : ''} ${className}`}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget)
          onClose()
      }}
    >
      <div className="modal-inner">
        <header className="modal-header">
          <h2 id={titleId} className="text-ui-lg">
            {title}
          </h2>
          <IconButton icon={X} label="关闭" onClick={onClose} />
        </header>
        <div className="modal-body">{children}</div>
      </div>
    </dialog>
  )
}
