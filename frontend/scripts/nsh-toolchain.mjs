import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { createReadStream, createWriteStream } from 'node:fs'
import { access, chmod, mkdir, rename, rm } from 'node:fs/promises'
import { dirname, join, resolve } from 'node:path'
import process from 'node:process'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import { fileURLToPath } from 'node:url'

const version = '0.4.0'
const root = fileURLToPath(new URL('../', import.meta.url))
const distributions = {
  'linux-x64': ['nsh-linux-x86_64.tar.gz', '8806121aebf40bb51f8fb6aa703722fbb01253f9a435759c18d53beece1ec976'],
  'win32-x64': ['nsh-windows-x86_64.zip', 'e4cc3893700a1fcceb974c73ef8e08bc94a47a7a85c96d797d49c16dde95dff3'],
  'darwin-arm64': ['nsh-macos-arm64.tar.gz', 'f63c3d57c31b28c9b34227aac81f099d1e0e61f1db552a6ec5127d114eaf59cb'],
}

export function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', ...options })
  if (result.error)
    throw result.error
  if (result.status !== 0)
    throw new Error(`${command} failed (${result.status ?? result.signal})`)
  return result
}

async function exists(path) {
  try {
    await access(path)
    return true
  } catch {
    return false
  }
}

async function download(url, destination, checksum) {
  await mkdir(dirname(destination), { recursive: true })
  const partial = `${destination}.partial`
  console.log(`Downloading ${new URL(url).pathname.split('/').at(-1)}`)
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(10 * 60 * 1000) })
    if (!response.ok || !response.body)
      throw new Error(`Download failed: HTTP ${response.status}`)
    await pipeline(Readable.fromWeb(response.body), createWriteStream(partial))
    const hash = createHash('sha256')
    for await (const chunk of createReadStream(partial))
      hash.update(chunk)
    if (hash.digest('hex') !== checksum)
      throw new Error('Download checksum does not match the pinned toolchain')
    await rename(partial, destination)
  } finally {
    await rm(partial, { force: true })
  }
}

async function linuxLoader(cache) {
  // Pages currently builds on Ubuntu 22.04; NSH releases are built on 24.04.
  // Use a private loader only for the CLI, without replacing system libraries
  // or exporting LD_LIBRARY_PATH to npm, Node or the CLI's child processes.
  const directory = join(cache, 'glibc-2.39')
  const libraryPath = join(directory, 'usr/lib/x86_64-linux-gnu')
  const loader = join(libraryPath, 'ld-linux-x86-64.so.2')
  if (!await exists(loader)) {
    const archive = join(cache, 'libc6.deb')
    await download(
      'https://archive.ubuntu.com/ubuntu/pool/main/g/glibc/libc6_2.39-0ubuntu8.9_amd64.deb',
      archive,
      'ff5557d99b51f761c4b7c92368b9cc45565eda17df9bf9eb4b134d09825008be',
    )
    run('dpkg-deb', ['--extract', archive, directory])
    await rm(archive)
  }
  return { command: loader, args: ['--library-path', libraryPath] }
}

export async function ensureNsh() {
  const cache = join(root, '.nsh/toolchains', version, `${process.platform}-${process.arch}`)
  const executable = process.platform === 'win32' ? 'nsh.exe' : 'nsh'
  let command = process.env.NSH_BIN ? resolve(root, process.env.NSH_BIN) : executable
  let args = []
  let environment = process.env
  let result = spawnSync(command, ['--version'], { encoding: 'utf8' })
  if (!process.env.NSH_BIN && (result.error || result.status !== 0 || !result.stdout.startsWith(`nsh ${version} `))) {
    const distribution = distributions[`${process.platform}-${process.arch}`]
    if (!distribution)
      throw new Error('No prebuilt NSH toolchain for this platform; set NSH_BIN to a compatible CLI')
    command = join(cache, 'nsh/bin', executable)
    if (!await exists(command) || !await exists(join(cache, 'nsh/runtime/web/nsh-browser.wasm')) || !await exists(join(cache, 'nsh/runtime/web/nsh.js'))) {
      const [filename, checksum] = distribution
      const archive = join(cache, filename)
      await download(`https://github.com/longbridge/nsh/releases/download/v${version}/${filename}`, archive, checksum)
      run('tar', ['-xf', archive, '-C', cache, `nsh/bin/${executable}`, 'nsh/runtime/web'])
      await chmod(command, 0o755)
      await rm(archive)
    }
    result = spawnSync(command, ['--version'], { encoding: 'utf8' })
  }
  if (process.platform === 'linux' && process.arch === 'x64' && /GLIBC_[\d.].*not found/.test(result.stderr || '')) {
    const loader = await linuxLoader(cache)
    environment = { ...process.env, NSH_RUNTIME: process.env.NSH_RUNTIME || join(dirname(command), '../runtime') }
    args = [...loader.args, command]
    command = loader.command
    result = spawnSync(command, [...args, '--version'], { encoding: 'utf8' })
  }
  if (result.error || result.status !== 0)
    throw new Error(`Cannot run NSH: ${result.error?.message || result.stderr || result.stdout}`)
  if (!result.stdout.startsWith(`nsh ${version} `))
    throw new Error(`Expected NSH ${version}, got ${result.stdout.trim()}`)
  console.log(result.stdout.trim())
  return { command, args, env: environment }
}
