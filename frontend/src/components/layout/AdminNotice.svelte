<script lang="ts">
  import { onMount } from 'svelte'
  import { IsElevated, RestartElevated } from '$lib/api'
  import { strings } from '$lib/strings'

  // Assume elevated until the backend says otherwise: never flash the notice.
  let elevated = $state(true)
  let error = $state('')

  onMount(async () => {
    try {
      elevated = await IsElevated()
    } catch {
      // Unknown: keep the notice hidden.
    }
  })

  async function restart() {
    error = ''
    try {
      await RestartElevated()
    } catch (e) {
      error = `${strings.home.adminError} ${e instanceof Error ? e.message : String(e)}`
    }
  }
</script>

{#if !elevated}
  <div
    role="region"
    aria-label={strings.home.adminTitle}
    title={strings.home.adminBody}
    class="mt-2 flex flex-col items-start gap-1 border-t border-stroke-soft px-3 pt-2.5 text-xs leading-snug text-muted"
  >
    <p>{strings.home.adminTitle}</p>
    <p class="sr-only">{strings.home.adminBody}</p>
    <button
      type="button"
      class="cursor-pointer rounded-sm text-left font-semibold text-accent hover:text-accent-hover hover:underline"
      onclick={restart}>{strings.home.adminRestart}</button
    >
    {#if error}<p role="alert" class="text-warn-text">{error}</p>{/if}
  </div>
{/if}
