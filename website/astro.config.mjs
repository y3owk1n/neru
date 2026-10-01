// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { unified } from '@astrojs/markdown-remark';
import remarkNeruDocs from './src/plugins/remark-neru-docs.mjs';
import { base, channel, ogImage, repo, repoRoot, sidebar, site, siteTitle } from './src/site.mjs';

export default defineConfig({
  site,
  base,
  trailingSlash: 'always',
  outDir: `./dist/${channel}`,
  markdown: { processor: unified({ remarkPlugins: [remarkNeruDocs] }) },
  integrations: [
    starlight({
      title: siteTitle,
      description: 'Hints, grids and vim keys for your whole desktop.',
      logo: { src: './src/assets/neru-appicon.png', alt: 'Neru' },
      favicon: '/favicon.png',
      head: [
        { tag: 'link', attrs: { rel: 'apple-touch-icon', href: `${base}apple-touch-icon.png` } },
        { tag: 'meta', attrs: { property: 'og:image', content: ogImage } },
        { tag: 'meta', attrs: { property: 'og:image:alt', content: 'Neru, the mouse is now optional' } },
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        // Nightly duplicates the release docs, so search engines index the release.
        ...(channel === 'nightly'
          ? [{ tag: /** @type {const} */ ('meta'), attrs: { name: 'robots', content: 'noindex' } }]
          : []),
      ],
      lastUpdated: true,
      // A branded page in src/pages/404.astro replaces Starlight's.
      disable404Route: true,
      social: [
        { icon: 'github', label: 'GitHub', href: repo },
        { icon: 'discord', label: 'Discord', href: 'https://discord.gg/KZwnwr9dz6' },
      ],
      // Starlight's asides and heading links only touch the dirs listed here:
      // docs/ and the root CHANGELOG.md.
      markdown: { processedDirs: [repoRoot] },
      // Compositor configs in the Linux guides read well as ini.
      expressiveCode: { shiki: { langAlias: { sway: 'ini', hyprlang: 'ini' } } },
      customCss: ['./src/styles/theme.css'],
      components: {
        Banner: './src/components/Banner.astro',
        SocialIcons: './src/components/SocialIcons.astro',
      },
      sidebar: sidebar(),
    }),
  ],
});
