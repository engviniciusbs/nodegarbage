type UAData = { getHighEntropyValues?: (hints: string[]) => Promise<{ platform?: string; platformVersion?: string }> }

/**
 * Windows 10 has no Mica: main.go makes the window opaque there, and this marks
 * <html> with `.solid` so the page is painted with --surface instead of --mica.
 * Windows 11 reports a platformVersion major >= 13; Windows 10 reports < 13.
 */
export async function markSolidBackdrop(doc: Document = document, nav: Navigator = navigator): Promise<void> {
  const uad = (nav as Navigator & { userAgentData?: UAData }).userAgentData
  if (!uad?.getHighEntropyValues) return
  try {
    const { platform, platformVersion } = await uad.getHighEntropyValues(['platformVersion'])
    const major = Number.parseInt(platformVersion ?? '', 10)
    if (platform === 'Windows' && Number.isFinite(major) && major < 13) doc.documentElement.classList.add('solid')
  } catch {
    // Unknown platform: keep the Mica tint (still a near-opaque light background).
  }
}
