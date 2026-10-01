import { spawnSync } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';
import { docsSchema } from '@astrojs/starlight/schema';
import { channel, changelogPath, docId, docsDir, repo, siteTitle, unpublished } from './site.mjs';

// The repo's docs/ is the only copy of the docs. Its pages open with a `# H1`
// and carry no frontmatter, so the loader derives what Starlight needs from
// the Markdown: the title from the H1 (remark-neru-docs drops it from the
// body), the description from the first sentence, the preview title, and on
// nightly an edit link to the file on main.
const inner = glob({
  base: docsDir,
  pattern: ['**/*.md', ...unpublished.map((u) => (u.endsWith('/') ? `!${u}**` : `!${u}`))],
  generateId: ({ entry }) => docId(entry),
});

// When the file last changed in git. Starlight reads dates only for its own
// content folder, and docs/ lives outside it.
function lastCommitDate(file: string): Date | undefined {
  const log = spawnSync('git', ['log', '-1', '--format=%cI', '--', path.basename(file)], {
    cwd: path.dirname(file),
    encoding: 'utf8',
  });
  const date = log.stdout?.trim();
  return date ? new Date(date) : undefined;
}

// The first sentence of the first paragraph, as plain text.
function firstSentence(markdown: string): string | undefined {
  const body = markdown.replace(/^#\s+.+$/m, '');
  for (const block of body.split(/\n\s*\n/)) {
    const text = block.trim();
    if (!text || /^(#|>|\||-|\*|```|<|!\[|\d+\.)/.test(text)) continue;
    const plain = text
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      .replace(/`([^`]+)`/g, '$1')
      .replace(/\*\*?([^*]+)\*\*?/g, '$1')
      .replace(/\s+/g, ' ');
    const sentence = plain.match(/^.+?[.!?](?=\s|$)/)?.[0] ?? plain;
    return sentence.length > 200 ? `${sentence.slice(0, 197).trimEnd()}...` : sentence;
  }
  return undefined;
}

const docs = defineCollection({
  loader: {
    name: 'neru-docs-loader',
    load: (context) =>
      inner.load({
        ...context,
        parseData: async (props) => {
          if ('title' in props.data || !props.filePath) return context.parseData(props);
          const source = await fs.readFile(props.filePath, 'utf8');
          const h1 = source.match(/^#\s+(.+)$/m)?.[1];
          const title = h1?.replaceAll('`', '').trim() ?? path.basename(props.filePath, '.md');
          const relPath = path.relative(docsDir, props.filePath).split(path.sep).join('/');
          const data = {
            ...props.data,
            title,
            description: firstSentence(source),
            lastUpdated: lastCommitDate(props.filePath),
            editUrl: channel === 'nightly' ? `${repo}/edit/main/docs/${relPath}` : false,
            head: [{ tag: 'meta', attrs: { property: 'og:title', content: `${title} | ${siteTitle}` } }],
          };
          return context.parseData({ ...props, data });
        },
      }),
  },
  schema: docsSchema(),
});

// The root CHANGELOG.md, written by Release Please, rendered at /changelog/.
const changelog = defineCollection({
  loader: glob({
    base: path.dirname(changelogPath),
    pattern: path.basename(changelogPath),
    generateId: () => 'changelog',
  }),
  schema: z.object({}).passthrough(),
});

export const collections = { docs, changelog };
