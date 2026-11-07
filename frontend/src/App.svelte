<script lang="ts">
  import Sidebar from './components/layout/Sidebar.svelte'
  import PageHeader from './components/layout/PageHeader.svelte'
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
  <!-- Content "layer": light, 1px stroke, rounded top-left corner, as in Win11 apps. -->
  <main
    class="flex min-w-0 flex-1 flex-col overflow-hidden rounded-tl-layer border-t border-l border-stroke bg-layer"
  >
    {#if route.current === 'results'}
      <!-- Results owns its header and keeps the page from scrolling (list and details scroll inside). -->
      <Results />
    {:else}
      <PageHeader title={strings.nav[route.current]} />
      <div class="min-h-0 flex-1 overflow-auto px-6 pb-6">
        {#if route.current === 'home'}
          <Home />
        {:else if route.current === 'scan'}
          <Scan />
        {:else if route.current === 'ai'}
          <AIReport />
        {:else if route.current === 'history'}
          <History />
        {:else if route.current === 'settings'}
          <Settings />
        {/if}
      </div>
    {/if}
  </main>
</div>
