<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  interface Props extends HTMLButtonAttributes {
    variant?: 'primary' | 'secondary' | 'ghost'
    children: Snippet
  }
  let { variant = 'secondary', type = 'button', children, class: extra = '', ...rest }: Props = $props()

  const base =
    'inline-flex items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition-colors disabled:opacity-50 disabled:pointer-events-none cursor-pointer'
  const variants = {
    primary: 'bg-accent text-white hover:bg-accent-hover',
    secondary: 'glass text-primary hover:bg-white/80',
    ghost: 'text-primary hover:bg-black/5',
  }
</script>

<button {type} class="{base} {variants[variant]} {extra}" {...rest}>
  {@render children()}
</button>
