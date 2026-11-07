<script lang="ts">
  import Sidebar from './components/layout/Sidebar.svelte'
  import Home from './screens/Home.svelte'
  import Scan from './screens/Scan.svelte'
  import Results from './screens/Results.svelte'
  import AIReport from './screens/AIReport.svelte'
  import History from './screens/History.svelte'
  import Settings from './screens/Settings.svelte'
  import { route } from '$lib/route.svelte'
  import { initScanEvents } from '$lib/stores/scan.svelte'
  import { initAIEvents } from '$lib/stores/ai.svelte'
  import { strings } from '$lib/strings'

  initScanEvents()
  initAIEvents()
</script>

<div class="flex h-full">
  <Sidebar />
  <main class="min-w-0 flex-1 overflow-auto p-6">
    <h1 class="mb-4 text-xl font-semibold text-primary">{strings.nav[route.current]}</h1>
    {#if route.current === 'home'}
      <Home />
    {:else if route.current === 'scan'}
      <Scan />
    {:else if route.current === 'results'}
      <Results />
    {:else if route.current === 'ai'}
      <AIReport />
    {:else if route.current === 'history'}
      <History />
    {:else if route.current === 'settings'}
      <Settings />
    {/if}
  </main>
</div>
