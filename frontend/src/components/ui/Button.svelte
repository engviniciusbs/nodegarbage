<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  interface Props extends HTMLButtonAttributes {
    variant?: 'primary' | 'secondary' | 'ghost'
    children: Snippet
  }
  let { variant = 'secondary', type = 'button', children, class: extra = '', ...rest }: Props = $props()

  const base =
    'inline-flex items-center justify-center gap-2 rounded-sm border px-3 py-1.5 text-[13px] transition-colors disabled:opacity-45 disabled:cursor-not-allowed cursor-pointer'
  const variants = {
    primary: 'border-accent bg-accent font-semibold text-on-accent enabled:hover:bg-accent-hover enabled:hover:border-accent-hover',
    secondary: 'border-stroke bg-field text-primary enabled:hover:bg-hover',
    ghost: 'border-transparent bg-transparent text-primary enabled:hover:bg-hover',
  }
</script>

<button {type} class="{base} {variants[variant]} {extra}" {...rest}>
  {@render children()}
</button>
