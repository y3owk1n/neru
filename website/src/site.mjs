// Build settings shared by the Astro config, the content loader and the
// components. scripts/build.sh sets them once per channel. The defaults give
// `npm run dev` a nightly build of the working tree's docs/.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const websiteDir = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const env = process.env;

export const repo = 'https://github.com/y3owk1n/neru';
export const channel = env.NERU_DOCS_CHANNEL === 'latest' ? 'latest' : 'nightly';
export const docsDir = path.resolve(websiteDir, env.NERU_DOCS_DIR ?? '../docs');
// The git ref docs/ was read from, for links to files the site does not publish.
export const ref = env.NERU_DOCS_REF ?? 'main';
export const latestVersion = env.NERU_LATEST_VERSION ?? 'latest';
export const site = env.NERU_SITE ?? 'https://y3owk1n.github.io';
// The root every channel lives under, e.g. "/neru/" on a GitHub Pages project site.
const rootBase = withSlashes(env.NERU_BASE ?? '/');
export const base = channel === 'latest' ? rootBase : `${rootBase}nightly/`;
export const channelBases = { latest: rootBase, nightly: `${rootBase}nightly/` };
// The site name in titles and previews, so a nightly tab or shared link says so.
export const siteTitle = channel === 'latest' ? 'Neru' : 'Neru nightly';
export const ogImage = new URL(`${rootBase}og.png`, site).href;
export const repoRoot = path.dirname(docsDir);
export const changelogPath = path.join(repoRoot, 'CHANGELOG.md');

function withSlashes(p) {
  return `/${p.replace(/^\/+|\/+$/g, '')}/`.replace('//', '/');
}

// Page slugs that changed when the docs moved into guide/ and reference/, as
// old: new. The version switcher uses them to land on the same page in a
// release from before the move. Getting started is new and has no old page.
export const renamedPages = {
  installation: 'guide/installation',
  'tips-tricks': 'guide/recipes',
  troubleshooting: 'guide/troubleshooting',
  'linux-setup': 'guide/linux',
  'linux-desktops': 'guide/linux-desktops',
  configuration: 'reference/configuration',
  cli: 'reference/cli',
  'cross-platform': 'reference/platform-support',
  roadmap: 'project/roadmap',
  'config-showcases': 'project/showcases',
};

// Pages split out of an older page, as new: old, so switching to a release
// from before the split lands on the page that held the content.
export const splitPages = {
  'guide/getting-started': 'configuration',
  'reference/scripting': 'cli',
};

// Turns a docs path into a page slug. CROSS_PLATFORM.md becomes
// cross-platform, and adr/0001-x.md becomes adr/0001-x.
export function docId(relPath) {
  return relPath.replace(/\.md$/, '').toLowerCase().replaceAll('_', '-');
}

// The site is for people using Neru. Contributor docs stay on GitHub, and
// links to them point there. The last three entries are their names in
// releases from before the docs moved. docs/README.md is the GitHub index,
// which the sidebar replaces, and docs/agents/ holds untracked local files.
export const unpublished = [
  'README.md',
  'agents/',
  'contributing/',
  'adr/',
  'go/',
  'ARCHITECTURE.md',
  'DEVELOPMENT.md',
];

export function isPublished(relPath) {
  return relPath.endsWith('.md') && !unpublished.some((u) => relPath === u || (u.endsWith('/') && relPath.startsWith(u)));
}

// Every published doc, as paths relative to docsDir.
function publishedDocs() {
  return fs
    .readdirSync(docsDir, { recursive: true })
    .map((p) => p.split(path.sep).join('/'))
    .filter(isPublished)
    .sort();
}

// The sidebar follows docs/README.md. Each `## Section` is a group and each
// link under it an entry, in order. A link to a directory becomes a collapsed
// group of its files. Docs the index does not link land in "More", and a tree
// without an index (a release from before docs/README.md) lists every file.
export function sidebar() {
  const docs = publishedDocs();
  const indexPath = path.join(docsDir, 'README.md');
  if (!fs.existsSync(indexPath)) {
    const dirs = [...new Set(docs.filter((d) => d.includes('/')).map((d) => d.split('/')[0]))];
    return [
      ...docs.filter((d) => !d.includes('/')).map(docId),
      ...(fs.existsSync(changelogPath) ? [{ label: 'Changelog', link: '/changelog/' }] : []),
      ...dirs.map((dir) => ({
        label: dir,
        collapsed: true,
        items: docs.filter((d) => d.startsWith(`${dir}/`)).map(docId),
      })),
    ];
  }

  const listed = new Set();
  const groups = [];
  for (const line of fs.readFileSync(indexPath, 'utf8').split('\n')) {
    const heading = line.match(/^## (.+)/);
    if (heading) {
      groups.push({ label: heading[1].trim(), items: [] });
      continue;
    }
    const link = line.match(/^- \[([^\]]+)\]\(([^)#]+)\)/);
    if (!link || !groups.length) continue;
    const [, label, target] = link;
    if (target.endsWith('/')) {
      const files = docs.filter((d) => d.startsWith(target));
      files.forEach((d) => listed.add(d));
      if (files.length) groups.at(-1).items.push({ label, collapsed: true, items: files.map(docId) });
    } else if (target === '../CHANGELOG.md') {
      if (fs.existsSync(changelogPath)) groups.at(-1).items.push({ label, link: '/changelog/' });
    } else if (docs.includes(target)) {
      listed.add(target);
      groups.at(-1).items.push(docId(target));
    }
  }

  const rest = docs.filter((d) => !listed.has(d));
  if (rest.length) groups.push({ label: 'More', items: rest.map(docId) });
  return groups.filter((g) => g.items.length);
}

// The first of several doc paths that exists in this channel's docs/, as a
// site link. Pages moved between releases, so the latest and nightly channels
// can name the same page differently.
export function docHref(...relPaths) {
  const found = relPaths.find((p) => fs.existsSync(path.join(docsDir, p))) ?? relPaths[0];
  return `${base}${docId(found)}/`;
}
