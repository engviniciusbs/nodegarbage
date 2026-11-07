<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import Button from '../ui/Button.svelte'
  import Icon from '../ui/Icon.svelte'
  import { RestartElevated } from '$lib/api'
  import { strings } from '$lib/strings'

  let error = $state('')

  async function restart() {
    error = ''
    try {
      await RestartElevated()
    } catch (e) {
      error = `${strings.home.adminError} ${e instanceof Error ? e.message : String(e)}`
    }
  }
</script>

<Panel class="flex items-start gap-3" role="region" aria-label={strings.home.adminTitle}>
  <span class="mt-0.5 text-warn-text"><Icon name="caution" /></span>
  <div class="min-w-0 flex-1 space-y-2">
    <p class="text-sm font-semibold text-primary">{strings.home.adminTitle}</p>
    <p class="text-sm text-muted">{strings.home.adminBody}</p>
    {#if error}<p role="alert" class="text-sm text-warn-text">{error}</p>{/if}
    <Button onclick={restart}>{strings.home.adminRestart}</Button>
  </div>
</Panel>
