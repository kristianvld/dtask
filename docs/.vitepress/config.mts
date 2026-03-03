import { defineConfig } from 'vitepress'
import { readdirSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))

function detectVersionsFromFs(): string[] {
  try {
    const versionsDir = resolve(__dirname, '..', 'versions')
    return readdirSync(versionsDir)
      .filter((name) => /^v\d+\.\d+\.\d+$/.test(name))
      .sort((a, b) => {
        const [a1, a2, a3] = a.slice(1).split('.').map(Number)
        const [b1, b2, b3] = b.slice(1).split('.').map(Number)
        return b1 - a1 || b2 - a2 || b3 - a3
      })
  } catch {
    return []
  }
}

const repo = process.env.GITHUB_REPOSITORY?.split('/')[1] ?? ''
const isUserSite = repo.endsWith('.github.io')
const base = process.env.VITEPRESS_BASE ?? (process.env.GITHUB_ACTIONS === 'true' && !isUserSite ? `/${repo}/` : '/')
const docsBasePath = process.env.DOCS_BASE_PATH ?? base
let docsVersionList = (process.env.DOCS_VERSION_LIST ?? '')
  .split(',')
  .map((v) => v.trim())
  .filter(Boolean)
if (docsVersionList.length === 0) {
  docsVersionList = detectVersionsFromFs()
}
const docsLatestVersion =
  process.env.DOCS_LATEST_VERSION ?? (docsVersionList.length > 0 ? docsVersionList[0] : '')

// In dev: /vX.Y.Z/, /bleeding/, /latest/ (redirects to latest). Root / redirects to /latest/.
// In prod: /base/vX.Y.Z/, /base/edge/ (we use "edge" in prod, "bleeding" in dev).
const isDev = base === '/' && process.env.GITHUB_ACTIONS !== 'true'
const versionLink = (v: string) =>
  isDev ? `/${v}/` : (v === docsLatestVersion ? docsBasePath : `${docsBasePath}${v}/`)
const edgeLink = isDev ? '/bleeding/' : `${docsBasePath}edge/`
const latestLink = isDev ? '/latest/' : docsBasePath

const versionNavItems = docsVersionList.map((v) => ({
  text: v === docsLatestVersion ? `${v} (latest)` : v,
  link: v === docsLatestVersion ? latestLink : versionLink(v)
}))
if (versionNavItems.length > 0) {
  versionNavItems.push({ text: 'Bleeding', link: edgeLink })
}

// Build rewrites for dev: /vX.Y.Z/, /bleeding/, /latest/, redirect-root at /
const rewrites: Record<string, string> = {}
if (isDev) {
  rewrites['index.md'] = 'bleeding/index.md'
  rewrites['redirect-root.md'] = 'index.md'
  rewrites['redirect-latest.md'] = 'latest/index.md'
  for (const v of docsVersionList) {
    rewrites[`versions/${v}/index.md`] = `${v}/index.md`
  }
}

export default defineConfig({
  title: 'dtask',
  description: 'Docker task runner for Docker Compose stacks',
  base,
  // /latest/ exists only in dev (rewrites); redirect-root.md links to it for root redirect
  ignoreDeadLinks: ['/latest/', '/latest/index'],
  rewrites: Object.keys(rewrites).length > 0 ? rewrites : undefined,
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}favicon.svg` }]],
  vite: {
    define: {
      __DOCS_LATEST__: JSON.stringify(docsLatestVersion),
      __DOCS_IS_DEV__: JSON.stringify(isDev)
    }
  },
  transformPageData(pageData) {
    // redirect-latest.md is rewritten to latest/index.md
    const isLatestRedirect =
      pageData.relativePath === 'redirect-latest.md' ||
      pageData.relativePath === 'latest/index.md'
    if (isLatestRedirect) {
      pageData.frontmatter ??= {}
      pageData.frontmatter.head ??= []
      pageData.frontmatter.head.push([
        'meta',
        { 'http-equiv': 'refresh', content: `0;url=/${docsLatestVersion}/` }
      ])
    }
  },
  transformHead(context) {
    if (context.page === 'latest/index.md' || context.page === 'latest/index.html') {
      return [
        ['meta', { 'http-equiv': 'refresh', content: `0;url=/${docsLatestVersion}/` }]
      ]
    }
  },
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Documentation', link: isDev ? '/bleeding/' : '/' },
      ...(versionNavItems.length > 0 ? [{ text: 'Versions', items: versionNavItems }] : [])
    ],
    sidebar: [
      {
        text: 'Sections',
        items: [
          { text: 'Introduction', link: (isDev ? '/bleeding/' : '/') + '#dtask' },
          { text: 'Design Goals', link: (isDev ? '/bleeding/' : '/') + '#design-goals' },
          { text: 'Quick Start', link: (isDev ? '/bleeding/' : '/') + '#quick-start' },
          { text: 'Configuration Model', link: (isDev ? '/bleeding/' : '/') + '#configuration-model' },
          { text: 'Environment Validation', link: (isDev ? '/bleeding/' : '/') + '#environment-validation' },
          { text: 'Runtime Semantics', link: (isDev ? '/bleeding/' : '/') + '#runtime-semantics' },
          { text: 'Option Index', link: (isDev ? '/bleeding/' : '/') + '#option-index' },
          { text: 'Options', link: (isDev ? '/bleeding/' : '/') + '#options' },
          { text: 'Required Task Keys', link: (isDev ? '/bleeding/' : '/') + '#required-task-keys' },
          { text: 'Examples', link: (isDev ? '/bleeding/' : '/') + '#additional-examples' }
        ]
      }
    ],
    outline: 'deep'
  }
})
