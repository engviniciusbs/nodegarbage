import './styles/app.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { markSolidBackdrop } from '$lib/platform'

void markSolidBackdrop()

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
