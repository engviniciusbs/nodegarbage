export type Route = 'home' | 'scan' | 'results' | 'ai' | 'history' | 'settings'

export const route = $state({ current: 'home' as Route, volume: '' as string })
