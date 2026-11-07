<script lang="ts">
  import Sidebar from './components/layout/Sidebar.svelte'
  import EmptyState from './components/ui/EmptyState.svelte'
  import { route } from '$lib/route.svelte'
  import { strings } from '$lib/strings'
</script>

<div class="flex h-full">
  <Sidebar />
  <main class="min-w-0 flex-1 overflow-auto p-6">
    <h1 class="mb-4 text-xl font-semibold text-primary">{strings.nav[route.current]}</h1>
    <EmptyState title={strings.common.empty} description={strings.common.comingSoon} />
  </main>
</div>
