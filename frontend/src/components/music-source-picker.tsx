import type { MusicSearchSelection } from '@/types'
import { View } from 'nsh'
import { SelectField } from './select-field'

type MusicSource = MusicSearchSelection['source']
const sources = [
  { value: 'netease', title: '网易云音乐' },
  { value: 'qq', title: 'QQ音乐' },
] as const

export function MusicSourcePicker(props: { value: MusicSource, onChange: (source: MusicSource) => void }) {
  return (
    <View style={{ alignSelf: 'flex-end', maxWidth: '100%' }}>
      <SelectField label="选择音乐源" value={props.value} items={sources} onChange={value => props.onChange(value as MusicSource)} style={{ borderRadius: 999, backgroundColor: 'transparent' }} />
    </View>
  )
}
