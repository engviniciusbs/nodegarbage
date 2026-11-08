<script lang="ts">
  import Icon, { type IconName } from '../ui/Icon.svelte'
  import AdminNotice from './AdminNotice.svelte'
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

<!-- Transparent nav pane over the Mica tint (Win11 NavigationView): no panel of its own. -->
<nav aria-label={strings.nav.label} class="flex w-[220px] shrink-0 flex-col gap-0.5 px-2 pt-2 pb-4">
  {#each items as item (item.id)}
    {@const active = route.current === item.id}
    <button
      type="button"
      aria-current={active ? 'page' : undefined}
      onclick={() => (route.current = item.id)}
      class="relative flex h-[38px] cursor-pointer items-center gap-3 rounded-sm px-3 text-left text-sm text-primary transition-colors
        {active
        ? 'bg-nav-active font-semibold before:absolute before:inset-y-[11px] before:left-0 before:w-[3px] before:rounded-sm before:bg-accent'
        : 'hover:bg-hover'}"
    >
      <Icon name={item.icon} size={16} />
      <span>{strings.nav[item.id]}</span>
    </button>
  {/each}
  <div class="flex-1"></div>
  <AdminNotice />
</nav>
