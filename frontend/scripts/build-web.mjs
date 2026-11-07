import { createHash } from 'node:crypto'
import { cp, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'
import { loadEnv } from 'vite'
import { ensureNsh, run } from './nsh-toolchain.mjs'

const root = fileURLToPath(new URL('../', import.meta.url))
const app = root
const output = join(root, 'dist')
const staging = join(app, '.nsh/pages-build')
const modeIndex = process.argv.indexOf('--mode')
const mode = modeIndex >= 0 ? process.argv[modeIndex + 1] : 'production'

function escapeHtml(text) {
  return text.replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', '\'': '&#39;' })[character])
}

async function build() {
  if (!mode)
    throw new Error('--mode requires a value')
  const environment = loadEnv(mode, root, 'VITE_')
  // Only these public values enter the site; never serialize process.env.
  const config = Object.fromEntries(['VITE_WS_URL', 'VITE_API_URL', 'VITE_APP_NAME']
    .filter(key => environment[key])
    .map(key => [key, environment[key]]))
  for (const [key, protocols] of [['VITE_WS_URL', ['ws:', 'wss:']], ['VITE_API_URL', ['http:', 'https:']]]) {
    if (config[key] && !protocols.includes(new URL(config[key]).protocol))
      throw new Error(`${key} must use ${protocols.join(' or ')}`)
  }
  const toolchain = await ensureNsh()
  const metadata = JSON.parse(await readFile(join(app, 'nsh.json'), 'utf8'))
  run(toolchain.command, [...toolchain.args, 'package', 'App.tsx', '--target', 'web', '--out', staging], { cwd: app, env: toolchain.env })
  const site = join(staging, metadata.name)
  const configSource = `globalThis.__ALISTEN_CONFIG__ = ${JSON.stringify(config)};\n`
  const digest = createHash('sha256').update(configSource).digest('hex').slice(0, 10)
  const configFile = `alisten-config.${digest}.js`
  await writeFile(join(site, configFile), configSource)
  let html = await readFile(join(site, 'index.html'), 'utf8')
  html = html.replace('<head>', `<head>\n<script src="./${configFile}"></script>`)
    .replace('<html lang="en">', '<html lang="zh-CN">')
  const manifestPath = join(site, 'manifest.webmanifest')
  const manifest = JSON.parse(await readFile(manifestPath, 'utf8'))
  // The React manifest had no explicit id: its start_url ('/') was its id.
  // Keep that identity when replacing an already-installed Pages PWA.
  manifest.id = '/'
  if (config.VITE_APP_NAME) {
    html = html.replace(/<title>[^<]*<\/title>/, () => `<title>${escapeHtml(config.VITE_APP_NAME)}</title>`)
    manifest.name = config.VITE_APP_NAME
    manifest.short_name = config.VITE_APP_NAME
  }
  await writeFile(manifestPath, JSON.stringify(manifest))
  await writeFile(join(site, 'index.html'), html)
  // NSH precompresses its output. Pages performs its own compression; remove
  // sidecars so modified HTML/manifest never have stale compressed versions.
  async function removeCompressed(directory) {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name)
      if (entry.isDirectory())
        await removeCompressed(path)
      else if (/\.(?:br|gz)$/.test(entry.name))
        await rm(path)
    }
  }
  await removeCompressed(site)
  await cp(join(root, 'cloudflare'), site, { recursive: true })
  // This is a fixed build-output directory, never a path taken from env/flags.
  await rm(output, { recursive: true, force: true })
  await mkdir(output, { recursive: true })
  await cp(site, output, { recursive: true })
  console.log('Cloudflare Pages site ready: frontend/dist')
}

build().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})
