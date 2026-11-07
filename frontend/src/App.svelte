<script lang="ts">
  import Sidebar from './components/layout/Sidebar.svelte'
  import EmptyState from './components/ui/EmptyState.svelte'
  import Home from './screens/Home.svelte'
  import Scan from './screens/Scan.svelte'
  import Results from './screens/Results.svelte'
  import AIReport from './screens/AIReport.svelte'
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
    {:else}
      <EmptyState title={strings.common.empty} description={strings.common.comingSoon} />
    {/if}
  </main>
</div>
