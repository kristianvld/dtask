import { defineConfig } from 'vitepress'

const repo = process.env.GITHUB_REPOSITORY?.split('/')[1] ?? ''
const isUserSite = repo.endsWith('.github.io')
const base = process.env.VITEPRESS_BASE ?? (process.env.GITHUB_ACTIONS === 'true' && !isUserSite ? `/${repo}/` : '/')

export default defineConfig({
  title: 'dtask',
  description: 'Docker task runner for Docker Compose stacks',
  base,
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}logo.svg` }]],
  themeConfig: {
    logo: '/logo.svg',
    nav: [{ text: 'Documentation', link: '/' }],
    sidebar: [
      {
        text: 'Sections',
        items: [
          { text: 'Introduction', link: '/#dtask' },
          { text: 'Design Goals', link: '/#design-goals' },
          { text: 'Quick Start', link: '/#quick-start' },
          { text: 'Configuration Model', link: '/#configuration-model' },
          { text: 'Environment Validation', link: '/#environment-validation' },
          { text: 'Runtime Semantics', link: '/#runtime-semantics' },
          { text: 'Option Index', link: '/#option-index' },
          { text: 'Options', link: '/#options' },
          { text: 'Required Task Keys', link: '/#required-task-keys' },
          { text: 'Examples', link: '/#additional-examples' }
        ]
      }
    ],
    outline: 'deep'
  }
})
