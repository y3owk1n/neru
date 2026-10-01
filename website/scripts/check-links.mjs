// Checks every internal link in the built site. Each one must reach a page or
// file in dist/site, and each #anchor must reach an id on that page. Run after
// build.sh, with the same NERU_BASE the build used.
import fs from 'node:fs';
import path from 'node:path';

const root = path.join(path.dirname(new URL(import.meta.url).pathname), '..', 'dist', 'site');
const base = `/${(process.env.NERU_BASE ?? '/').replace(/^\/+|\/+$/g, '')}/`.replace('//', '/');

const pages = new Map();
for (const file of fs.readdirSync(root, { recursive: true })) {
  if (file.endsWith('.html')) pages.set(file.split(path.sep).join('/'), fs.readFileSync(path.join(root, file), 'utf8'));
}

const ids = new Map();
const idsOf = (file) => {
  if (!ids.has(file)) ids.set(file, new Set([...pages.get(file).matchAll(/\sid="([^"]+)"/g)].map((m) => m[1])));
  return ids.get(file);
};

const broken = [];
for (const [file, html] of pages) {
  for (const [, raw] of html.matchAll(/\shref="([^"]+)"/g)) {
    const href = raw.replaceAll('&amp;', '&');
    if (!href.startsWith('/') || href.startsWith('//')) continue;
    if (!href.startsWith(base)) {
      broken.push(`${file}: ${href} is outside the base ${base}`);
      continue;
    }
    const [target, fragment] = href.slice(base.length).split('#');
    const decoded = decodeURIComponent(target);
    const candidate = decoded === '' || decoded.endsWith('/') ? `${decoded}index.html` : decoded;
    if (!pages.has(candidate) && !fs.existsSync(path.join(root, candidate))) {
      broken.push(`${file}: ${href} does not exist`);
    } else if (fragment && pages.has(candidate) && !idsOf(candidate).has(decodeURIComponent(fragment))) {
      broken.push(`${file}: ${href} has no #${fragment}`);
    }
  }
}

if (broken.length) {
  console.error(`check-links: ${broken.length} broken link(s)\n${broken.join('\n')}`);
  process.exit(1);
}
console.log(`check-links: ${pages.size} pages, every internal link resolves`);
