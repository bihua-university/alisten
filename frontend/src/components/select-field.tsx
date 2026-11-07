import type { ViewRef, ViewStyle } from 'nsh'
import { effect, Keyboard, Overlay, Scroll, signal, Text, View, windowSize } from 'nsh'
import { Button } from 'nsh/base'
import { brand, compact, focusRing, font, palette } from '@/theme'
import { Icon } from './icon'

export function SelectField(props: {
  label: string
  value: string
  items: readonly { value: string, title: string }[]
  onChange: (value: string) => void
  disabled?: boolean
  fullWidth?: boolean
  style?: ViewStyle
}) {
  const open = signal(false)
  const anchorWidth = signal(0)
  const colors = palette('dark')
  let anchor: ViewRef | undefined
  effect(() => {
    if (props.disabled)
      open.set(false)
  })
  return (
    <View ref={anchor} onLayout={({ bounds }) => anchorWidth.set(bounds.size.width)} style={{ width: props.fullWidth ? '100%' : undefined, minWidth: 0, maxWidth: '100%' }}>
      <Button
        role="combobox"
        accessibilityLabel={`${props.label}，${props.items.find(item => item.value === props.value)?.title || ''}${open() ? '，已展开' : ''}`}
        selected={open()}
        disabled={props.disabled}
        disabledStyle={{ opacity: 0.45 }}
        focusStyle={focusRing}
        onPress={() => {
          if (compact())
            Keyboard.dismiss()
          open.update(value => !value)
        }}
        style={[{ minHeight: compact() ? 48 : 40, minWidth: 128, maxWidth: '100%', flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, paddingHorizontal: 14, paddingVertical: 10, borderWidth: 1, borderColor: open() ? brand : colors.inputBorder, borderRadius: 8, backgroundColor: colors.input, cursor: 'pointer' }, props.style]}
      >
        <Text style={[font(compact() ? 2 : 0), { flex: 1, minWidth: 0, color: colors.foreground, numberOfLines: 1 }]}>{props.items.find(item => item.value === props.value)?.title}</Text>
        <Icon name="ChevronDown" size={18} color={colors.muted} />
      </Button>
      <If when={open() && !props.disabled}>
        <Overlay
          anchor={anchor}
          placement="bottom"
          align="end"
          offset={8}
          margin={16}
          onPressOutside={() => open.set(false)}
          onCancel={() => open.set(false)}
          style={{ width: Math.min(Math.max(anchorWidth(), 240), Math.max(0, windowSize().width - 32)), maxHeight: Math.min(360, windowSize().height - 32), padding: 8, borderWidth: 1, borderColor: colors.inputBorder, borderRadius: 12, backgroundColor: colors.popover }}
        >
          <Scroll keyboardShouldPersistTaps="handled" style={{ minHeight: 0, flexShrink: 1, maxHeight: Math.min(344, Math.max(80, windowSize().height * 0.5)) }} contentStyle={{ gap: 4 }}>
            <View role="listbox" accessibilityLabel={props.label} style={{ gap: 4 }}>
              <Each of={props.items} key={item => item.value}>
                {item => (
                  <Button
                    role="option"
                    accessibilityLabel={item().title}
                    selected={item().value === props.value}
                    focusStyle={focusRing}
                    activeStyle={{ opacity: 0.75 }}
                    onPress={() => {
                      open.set(false)
                      if (item().value !== props.value)
                        props.onChange(item().value)
                    }}
                    style={{ minHeight: compact() ? 60 : 44, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 16, paddingHorizontal: 16, paddingVertical: compact() ? 16 : 12, borderRadius: 8, backgroundColor: item().value === props.value ? colors.accent : 'transparent', cursor: 'pointer' }}
                  >
                    <Text style={[font(compact() ? 4 : 0), { flex: 1, minWidth: 0, color: item().value === props.value ? brand : colors.foreground, fontWeight: '500' }]}>{item().title}</Text>
                    <If when={item().value === props.value}><Icon name="Check" size={20} color={brand} /></If>
                  </Button>
                )}
              </Each>
            </View>
          </Scroll>
        </Overlay>
      </If>
    </View>
  )
}
