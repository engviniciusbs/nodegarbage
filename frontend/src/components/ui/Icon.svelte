<script lang="ts" module>
  export type IconName =
    | 'home' | 'scan' | 'results' | 'ai' | 'history' | 'settings'
    | 'safe' | 'caution' | 'keep' | 'unknown' | 'inbox' | 'link' | 'cloud' | 'denied' | 'folder' | 'close'

  // 24x24 stroke icons (inline SVG, no icon library).
  const paths: Record<IconName, string> = {
    home: 'M3 11l9-8 9 8M5 10v10h5v-6h4v6h5V10',
    scan: 'M11 4a7 7 0 1 0 0 14a7 7 0 0 0 0-14zM21 21l-5-5',
    results: 'M4 6h16M4 12h16M4 18h10',
    ai: 'M12 3l2 5 5 2-5 2-2 5-2-5-5-2 5-2zM19 16v4M17 18h4',
    history: 'M12 7v5l3 2M4 12a8 8 0 1 0 2.5-5.8M4 4v4h4',
    settings: 'M4 7h10M18 7h2M4 17h2M10 17h10M14 5v4M6 15v4',
    safe: 'M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18zM8 12l3 3 5-6',
    caution: 'M12 3l10 18H2zM12 10v4M12 17.5v.01',
    keep: 'M6 11h12v9H6zM8 11V8a4 4 0 0 1 8 0v3',
    unknown: 'M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18zM9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.7.4-1 1-1 1.7M12 17v.01',
    link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
    cloud: 'M7 18a4 4 0 0 1-.5-8A6 6 0 0 1 18 9.5a4.2 4.2 0 0 1-.5 8.5z',
    denied: 'M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18zM5.6 5.6l12.8 12.8',
    folder: 'M3 6.5a1.5 1.5 0 0 1 1.5-1.5h4.6l2 2h8.4a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z',
    inbox: 'M3 13l3-8h12l3 8v6H3zM3 13h5l1 3h6l1-3h5',
    close: 'M6 6l12 12M18 6L6 18',
  }
</script>

<script lang="ts">
  let { name, size = 20 }: { name: IconName; size?: number } = $props()
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.75"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  <path d={paths[name]} />
</svg>
