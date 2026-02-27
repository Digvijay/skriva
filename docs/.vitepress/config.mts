import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Skriva',
  description: 'A lightweight, single-binary personal blog engine written in Go',
  
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'Skriva — Personal Blog Engine' }],
    ['meta', { name: 'og:description', content: 'A lightweight, single-binary personal blog engine written in Go. Self-hosted, secure, and fediverse-native.' }],
    ['meta', { name: 'og:site_name', content: 'Skriva' }],
  ],

  // Custom domain
  hostname: 'https://skriva.digvijay.dev',

  // Ignore localhost links in docs (they're example URLs for the reader)
  ignoreDeadLinks: [
    /localhost/,
  ],

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/configuration' },
      { text: 'Themes', link: '/themes/overview' },
      { text: 'Security', link: '/security/overview' },
      { text: 'Changelog', link: '/changelog' },
      {
        text: 'v0.1.0',
        items: [
          { text: 'Changelog', link: '/changelog' },
          { text: 'GitHub Releases', link: 'https://github.com/Digvijay/skriva/releases' },
        ]
      }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'What is Skriva?', link: '/guide/what-is-skriva' },
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Quick Start', link: '/guide/quick-start' },
          ]
        },
        {
          text: 'Deployment',
          items: [
            { text: 'Docker', link: '/guide/deploy-docker' },
            { text: 'Azure Container Apps', link: '/guide/deploy-azure' },
            { text: 'Self-Host (Cloudflare)', link: '/guide/deploy-cloudflare' },
          ]
        },
        {
          text: 'Features',
          items: [
            { text: 'Writing Posts', link: '/guide/writing-posts' },
            { text: 'Newsletter', link: '/guide/newsletter' },
            { text: 'Fediverse & IndieWeb', link: '/guide/fediverse' },
            { text: 'Importing Content', link: '/guide/importing' },
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'Configuration', link: '/reference/configuration' },
            { text: 'API Endpoints', link: '/reference/api' },
            { text: 'Environment Variables', link: '/reference/environment' },
            { text: 'Database Migrations', link: '/reference/migrations' },
            { text: 'CLI Commands', link: '/reference/cli' },
          ]
        }
      ],
      '/themes/': [
        {
          text: 'Themes',
          items: [
            { text: 'Overview', link: '/themes/overview' },
            { text: 'Creating a Theme', link: '/themes/creating' },
            { text: 'Template Data', link: '/themes/template-data' },
            { text: 'Template Functions', link: '/themes/functions' },
          ]
        }
      ],
      '/security/': [
        {
          text: 'Security',
          items: [
            { text: 'Architecture', link: '/security/overview' },
            { text: 'Authentication', link: '/security/authentication' },
            { text: 'Network & SSRF', link: '/security/network' },
            { text: 'Federation', link: '/security/federation' },
            { text: 'Reporting Vulnerabilities', link: '/security/reporting' },
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/Digvijay/skriva' }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: '© 2025-present Digvijay Chauhan'
    },

    search: {
      provider: 'local'
    },

    editLink: {
      pattern: 'https://github.com/Digvijay/skriva/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
