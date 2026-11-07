// Retire the old React/Workbox worker at its original URL. Without this,
// existing installations can keep serving the cached React entry indefinitely.
globalThis.addEventListener('install', (event) => {
  event.waitUntil(globalThis.skipWaiting())
})

globalThis.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    await caches.delete(`workbox-precache-v2-${globalThis.registration.scope}`)
    await globalThis.clients.claim()
    const windows = await globalThis.clients.matchAll({ type: 'window' })
    await globalThis.registration.unregister()
    await Promise.all(windows.map(client => client.navigate(client.url)))
  })())
})
