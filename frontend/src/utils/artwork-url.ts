/** Music providers still return HTTP covers; native Android requires HTTPS. */
export function normalizeArtworkUrl(value?: string): string | undefined {
  const url = value?.trim()
  if (!url)
    return undefined
  return url.replace(/^http:\/\//i, 'https://').replace(/^\/\//, 'https://')
}
