<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLAttributes } from 'svelte/elements'

  interface Props extends HTMLAttributes<HTMLDivElement> {
    children: Snippet
  }
  let { children, class: extra = '', ...rest }: Props = $props()
</script>

<!-- A card inside the content layer: near-opaque white, 1px stroke (no glass). -->
<div class="rounded-md border border-stroke bg-card p-4 {extra}" {...rest}>
  {@render children()}
</div>
