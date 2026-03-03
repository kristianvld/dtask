import { defineConfig } from 'vitepress'

const repo = process.env.GITHUB_REPOSITORY?.split('/')[1] ?? ''
const isUserSite = repo.endsWith('.github.io')
const base = process.env.VITEPRESS_BASE ?? (process.env.GITHUB_ACTIONS === 'true' && !isUserSite ? `/${repo}/` : '/')
const docsBasePath = process.env.DOCS_BASE_PATH ?? base
const docsVersionList = (process.env.DOCS_VERSION_LIST ?? '')
  .split(',')
  .map((v) => v.trim())
  .filter(Boolean)
const docsLatestVersion = process.env.DOCS_LATEST_VERSION ?? ''

// Version links use full URLs to avoid SPA navigation issues
const versionNavItems = docsVersionList.map((v) => ({
  text: v === docsLatestVersion ? `${v} (latest)` : v,
  link: v === docsLatestVersion ? docsBasePath : `${docsBasePath}${v}/`
}))
if (versionNavItems.length > 0) {
  versionNavItems.push({ text: 'Edge', link: `${docsBasePath}edge/` })
}

export default defineConfig({
  title: 'dtask',
  description: 'Docker task runner for Docker Compose stacks',
  base,
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}favicon.svg` }]],
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Documentation', link: '/' },
      ...(versionNavItems.length > 0 ? [{ text: 'Versions', items: versionNavItems }] : [])
    ],
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
