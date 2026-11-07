<script lang="ts">
  import Icon, { type IconName } from '../ui/Icon.svelte'
  import { route, type Route } from '$lib/route.svelte'
  import { strings } from '$lib/strings'

  const items: { id: Route; icon: IconName }[] = [
    { id: 'home', icon: 'home' },
    { id: 'scan', icon: 'scan' },
    { id: 'results', icon: 'results' },
    { id: 'ai', icon: 'ai' },
    { id: 'history', icon: 'history' },
    { id: 'settings', icon: 'settings' },
  ]
</script>

<nav aria-label={strings.nav.label} class="glass flex w-56 shrink-0 flex-col gap-1 border-y-0 border-l-0 p-3">
  <div class="px-3 pb-3 pt-2 text-sm font-semibold text-primary">{strings.app.name}</div>
  {#each items as item (item.id)}
    {@const active = route.current === item.id}
    <button
      type="button"
      aria-current={active ? 'page' : undefined}
      onclick={() => (route.current = item.id)}
      class="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors
        {active ? 'bg-accent text-white' : 'text-primary hover:bg-black/5'}"
    >
      <Icon name={item.icon} />
      <span>{strings.nav[item.id]}</span>
    </button>
  {/each}
</nav>
