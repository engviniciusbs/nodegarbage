<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLAttributes } from 'svelte/elements'

  interface Props extends HTMLAttributes<HTMLDivElement> {
    children: Snippet
  }
  let { children, class: extra = '', ...rest }: Props = $props()
</script>

<div class="glass rounded-lg p-4 {extra}" {...rest}>
  {@render children()}
</div>
