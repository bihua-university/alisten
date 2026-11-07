import type { JSX, ViewStyle } from 'nsh'
import type { IconName } from './icon'
import { Image, keyboardFrame, platform, Scroll, signal, Text, View, windowSize } from 'nsh'
import { Button, Dialog } from 'nsh/base'
import { brand, compact, controlBase, focusRing, font, foreground, muted, palette, subtle, surface, surfaceHover } from '@/theme'
import { normalizeArtworkUrl } from '@/utils/artwork-url'
import { Icon } from './icon'

export function IconButton(props: {
  icon: IconName
  label: string
  onPress?: () => void
  disabled?: boolean
  selected?: boolean
  color?: string
  size?: number
  style?: ViewStyle | ViewStyle[]
}) {
  return (
    <Button
      accessibilityLabel={props.label}
      disabled={props.disabled}
      selected={props.selected}
      hitSlop={compact() ? 4 : 0}
      onPress={props.onPress}
      focusStyle={focusRing}
      disabledStyle={{ opacity: 0.45 }}
      style={[
        {
          width: compact() ? 44 : 36,
          height: compact() ? 44 : 36,
          borderRadius: 8,
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          cursor: 'pointer',
          transitionProperty: 'colors, opacity, transform',
          transitionDuration: 150,
        },
        props.style,
      ]}
    >
      <Icon name={props.icon} size={props.size ?? 20} color={props.color ?? (props.selected ? brand : muted)} />
    </Button>
  )
}

export function AsyncIconButton(props: Omit<Parameters<typeof IconButton>[0], 'onPress'> & { onAction: () => Promise<void> }) {
  const pending = signal(false)
  return (
    <IconButton
      {...props}
      icon={pending() ? 'Ellipsis' : props.icon}
      disabled={props.disabled || pending()}
      onPress={async () => {
        if (props.disabled || pending())
          return
        pending.set(true)
        try {
          await props.onAction()
        } finally {
          pending.set(false)
        }
      }}
    />
  )
}

export function TextButton(props: {
  title: string
  onPress?: () => void
  disabled?: boolean
  selected?: boolean
  variant?: 'primary' | 'secondary' | 'ghost' | 'destructive'
  style?: ViewStyle | ViewStyle[]
}) {
  const variant = props.variant ?? 'primary'
  const colors = palette()
  return (
    <Button
      onPress={props.onPress}
      disabled={props.disabled}
      selected={props.selected}
      disabledStyle={{ opacity: 0.45 }}
      focusStyle={focusRing}
      style={[
        controlBase,
        {
          minHeight: compact() ? 44 : 36,
          minWidth: 0,
          flexShrink: 0,
          backgroundColor: variant === 'primary' ? brand : variant === 'secondary' ? colors.surface : variant === 'destructive' ? colors.destructive : 'transparent',
          borderWidth: variant === 'secondary' ? 1 : 0,
          borderColor: colors.border,
        },
        props.style,
      ]}
    >
      <Text style={[font(), { color: variant === 'primary' ? '#122412' : variant === 'destructive' ? '#34151b' : colors.foreground, fontWeight: '500', textAlign: 'center' }]}>{props.title}</Text>
    </Button>
  )
}

export function Artwork(props: {
  src?: string
  title: string
  size?: number
  radius?: number
  fill?: boolean
  style?: ViewStyle | ViewStyle[]
}) {
  const failedSrc = signal<string>()
  const loadedSrc = signal<string>()
  const src = () => normalizeArtworkUrl(props.src)
  const size = () => props.size ?? 42
  const dimensions = () => props.fill ? { width: '100%' as const, height: '100%' as const } : { width: size(), height: size() }
  return (
    <View style={[{ ...dimensions(), borderRadius: props.radius ?? 8, overflow: 'hidden', alignItems: 'center', justifyContent: 'center', backgroundColor: surface, flexShrink: 0 }, props.style]}>
      <If when={!src() || src() === failedSrc() || src() !== loadedSrc()}>
        <Icon name="Music2" size={Math.round(size() * 0.55)} color={subtle} />
      </If>
      <Each of={src() && src() !== failedSrc() ? [src()!] : []} key={url => url}>
        {(url) => {
          const requestedSrc = url()
          return (
            <Image
              src={requestedSrc}
              accessibilityLabel={props.title}
              onLoadStart={() => {
                if (src() === requestedSrc)
                  loadedSrc.set(undefined)
              }}
              onLoad={() => {
                if (src() === requestedSrc)
                  loadedSrc.set(requestedSrc)
              }}
              onError={() => {
                if (src() === requestedSrc)
                  failedSrc.set(requestedSrc)
              }}
              style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', objectFit: 'cover', opacity: requestedSrc === loadedSrc() ? 1 : 0 }}
            />
          )
        }}
      </Each>
    </View>
  )
}

export function EmptyState(props: {
  icon?: IconName
  title: string
  children?: JSX.Element | string
  action?: JSX.Element
}) {
  return (
    <View style={{ minWidth: 0, maxWidth: '100%', minHeight: 230, paddingHorizontal: 20, paddingVertical: 32, gap: 12, alignItems: 'center', justifyContent: 'center' }}>
      <Icon name={props.icon ?? 'Music2'} size={36} color={subtle} />
      <Text style={[font(4), { maxWidth: '100%', color: foreground, fontWeight: '600', textAlign: 'center' }]}>{props.title}</Text>
      <Text style={[font(-1), { color: muted, textAlign: 'center', maxWidth: 380 }]}>{props.children}</Text>
      {props.action}
    </View>
  )
}

export function Skeleton(props: { rows?: number }) {
  return (
    <View accessibilityLabel="正在加载" style={{ paddingVertical: 8 }}>
      <Each of={Array.from({ length: props.rows ?? 4 }, (_, index) => index)}>
        {() => (
          <View style={{ flexDirection: 'row', gap: 12, alignItems: 'center', paddingVertical: 12 }}>
            <View style={{ width: 48, height: 48, borderRadius: 8, backgroundColor: surfaceHover }} />
            <View style={{ flex: 1, gap: 10 }}>
              <View style={{ width: '55%', height: 12, borderRadius: 4, backgroundColor: surfaceHover }} />
              <View style={{ width: '35%', height: 10, borderRadius: 4, backgroundColor: surface }} />
            </View>
          </View>
        )}
      </Each>
    </View>
  )
}

export function Modal(props: {
  title: string
  children: JSX.Element
  onClose: () => void
  open?: boolean
  wide?: boolean
  footer?: JSX.Element
  onOpen?: () => void
  onClosed?: () => void
}) {
  const mobile = compact
  const colors = palette()
  const width = () => mobile() ? windowSize().width : Math.min(props.wide ? 760 : 480, windowSize().width - (props.wide ? 48 : 32))
  const keyboardInset = () => platform.os === 'android' ? keyboardFrame().height : 0
  const headerHeight = signal(0)
  const footerHeight = signal(0)
  const availableHeight = () => Math.max(0, windowSize().height - keyboardInset() - (mobile() ? 16 : 48))
  return (
    <Dialog
      open={props.open !== false}
      animation={mobile() ? 'slide' : 'fade'}
      onOpen={props.onOpen}
      onClose={props.onClosed}
      onOpenChange={({ open }) => {
        if (!open)
          props.onClose()
      }}
      backdropStyle={{ backgroundColor: 'rgba(0,0,0,0.55)' }}
      style={{
        width: width(),
        minHeight: 0,
        flexShrink: 1,
        maxHeight: availableHeight(),
        marginHorizontal: mobile() ? 0 : 'auto',
        marginTop: mobile() ? 'auto' : 'auto',
        marginBottom: mobile() ? keyboardInset() : 'auto',
        borderWidth: mobile() ? 0 : 1,
        borderColor: colors.border,
        borderTopLeftRadius: 16,
        borderTopRightRadius: 16,
        borderBottomLeftRadius: mobile() ? 0 : 16,
        borderBottomRightRadius: mobile() ? 0 : 16,
        backgroundColor: colors.popover,
        overflow: 'hidden',
      }}
    >
      <View onLayout={({ bounds }) => headerHeight.set(bounds.size.height)} style={{ flexShrink: 0, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 16, paddingHorizontal: mobile() ? 20 : 24, paddingVertical: mobile() ? 12 : 16, borderBottomWidth: 1, borderBottomColor: colors.border }}>
        <Text style={[font(4), { color: colors.foreground, fontWeight: '600' }]}>{props.title}</Text>
        <IconButton icon="X" label="关闭" color={colors.muted} onPress={props.onClose} />
      </View>
      <Scroll
        keyboardShouldPersistTaps="handled"
        style={{ minHeight: 0, flexShrink: 1, maxHeight: Math.max(0, availableHeight() - headerHeight() - (props.footer ? footerHeight() : 0)) }}
        contentStyle={{ padding: mobile() ? 20 : 24, paddingBottom: props.footer || keyboardInset() > 0 ? mobile() ? 20 : 24 : 'calc(env(safe-area-inset-bottom) + 24px)' }}
      >
        {props.children}
      </Scroll>
      <If when={props.footer}>
        <View
          onLayout={({ bounds }) => footerHeight.set(bounds.size.height)}
          style={{ flexShrink: 0, paddingHorizontal: mobile() ? 20 : 24, paddingTop: 16, paddingBottom: keyboardInset() > 0 ? 20 : 'calc(env(safe-area-inset-bottom) + 24px)', borderTopWidth: 1, borderTopColor: colors.border }}
        >
          {props.footer}
        </View>
      </If>
    </Dialog>
  )
}
