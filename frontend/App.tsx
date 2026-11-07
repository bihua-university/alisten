import { mount, Text, View } from 'nsh'
import { hydrateRuntime } from './src/utils/runtime'
import { hydrateStorage } from './src/utils/storage'

async function start() {
  await Promise.all([hydrateRuntime(), hydrateStorage()])
  const { appearanceStore } = await import('./src/stores/appearance-store')
  appearanceStore.applySettings()
  const { App } = await import('./src/app')
  mount(App)
}

start().catch((error) => {
  console.error('应用初始化失败:', error)
  mount(() => (
    <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center', padding: 32, backgroundColor: '#121212' }}>
      <Text style={{ color: '#f4f4f5', fontSize: 18 }}>{`应用初始化失败：${error instanceof Error ? error.message : String(error)}`}</Text>
    </View>
  ))
})
