/** All UI copy (PT-BR). Components never hardcode user-visible text. */
export const strings = {
  app: { name: 'NodeGarbage' },
  nav: {
    label: 'Navegação principal',
    home: 'Início',
    scan: 'Varredura',
    results: 'Resultados',
    ai: 'Relatório da IA',
    history: 'Histórico',
    settings: 'Configurações',
  },
  verdict: {
    safe: 'Seguro',
    caution: 'Cuidado',
    keep: 'Não apagar',
    unknown: 'Desconhecido',
  },
  source: {
    ai: 'sugerido pela IA',
  },
  category: {
    'dev-cache': 'Cache de desenvolvimento',
    'build-artifact': 'Artefatos de build',
    'pkg-cache': 'Cache de pacotes',
    temp: 'Temporários',
    'browser-cache': 'Cache de navegador',
    system: 'Sistema',
    'user-data': 'Dados do usuário',
    ai: 'Sugeridos pela IA',
  },
  common: {
    loading: 'Carregando…',
    retry: 'Tentar novamente',
    cancel: 'Cancelar',
    confirm: 'Confirmar',
    close: 'Fechar',
    save: 'Salvar',
    error: 'Algo deu errado',
    empty: 'Nada por aqui ainda',
    comingSoon: 'Esta tela será implementada em breve.',
  },
} as const

export type Verdict = keyof typeof strings.verdict
export type CategoryId = keyof typeof strings.category
