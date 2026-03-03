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

// Full URLs in prod avoid Vue Router prepending base (e.g. /dtask/v0.1.1/dtask/v0.1.0/)
const deployUrl =
  process.env.GITHUB_ACTIONS && process.env.GITHUB_REPOSITORY
    ? `https://${process.env.GITHUB_REPOSITORY.split('/')[0]}.github.io/${repo}/`
    : ''
const versionLink = (v: string) =>
  deployUrl
    ? (v === docsLatestVersion ? deployUrl : `${deployUrl}${v}/`)
    : (v === docsLatestVersion ? docsBasePath : `${docsBasePath}${v}/`)
const versionNavItems = docsVersionList.map((v) => ({
  text: v === docsLatestVersion ? `${v} (latest)` : v,
  link: versionLink(v),
  ...(deployUrl && { target: '_self' as const })
}))
if (versionNavItems.length > 0) {
  versionNavItems.push({
    text: 'Edge',
    link: deployUrl ? `${deployUrl}edge/` : `${docsBasePath}edge/`,
    ...(deployUrl && { target: '_self' as const })
  })
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
