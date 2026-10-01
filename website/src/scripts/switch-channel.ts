// Finds the page a reader is on in the other docs channel, and the page a
// missing URL most likely meant. The channel bases, renamed and split pages and
// moved sections arrive as data attributes on `el`, because site.mjs reads the
// build environment and cannot run in the browser.

const exists = (url: string) => fetch(url, { method: 'HEAD' }).then((r) => r.ok, () => false);

type Candidate = { slug: string; renamedOnly: boolean };

// Every page `slug` may be called in another release, nearest first. A rename
// is followed both ways, since either channel may be the older one. A split is
// followed only from the new page to the one it came from, since the old page
// was split several ways. `renamedOnly` says whether the page was reached by
// renames alone, which is when its headings, and so the URL's anchor, carry
// over.
export function candidates(
  slug: string,
  renamed: Record<string, string>,
  split: Record<string, string>,
): Candidate[] {
  const seen = new Set([slug]);
  const found: Candidate[] = [{ slug, renamedOnly: true }];
  for (let i = 0; i < found.length; i++) {
    const { slug: at, renamedOnly } = found[i];
    const next: Candidate[] = [
      ...Object.entries(renamed)
        .filter(([old, now]) => old === at || now === at)
        .map(([old, now]) => ({ slug: old === at ? now : old, renamedOnly })),
      ...(split[at] ? [{ slug: split[at], renamedOnly: false }] : []),
    ];
    for (const candidate of next) {
      if (seen.has(candidate.slug)) continue;
      seen.add(candidate.slug);
      found.push(candidate);
    }
  }
  return found;
}

// The candidates for a URL. An anchor naming a section that became its own
// page puts that page, without the anchor, ahead of the page itself.
function candidatesFor(slug: string, hash: string, el: HTMLElement): Candidate[] {
  const renamed: Record<string, string> = JSON.parse(el.dataset.renamed!);
  const split: Record<string, string> = JSON.parse(el.dataset.split!);
  const sections: Record<string, string> = JSON.parse(el.dataset.sections!);
  const section = sections[hash.slice(1)];
  const moved = section && section !== slug ? candidates(section, renamed, split).map((c) => ({ ...c, renamedOnly: false })) : [];
  return [...moved, ...candidates(slug, renamed, split)];
}

// Undefined when the channel is not served at all, as under `astro dev`, which
// builds nightly only, so the caller stays put rather than opening a 404.
export async function pageIn(channel: string, el: HTMLElement): Promise<string | undefined> {
  const bases: Record<string, string> = JSON.parse(el.dataset.bases!);
  const from = Object.values(bases)
    .sort((a, b) => b.length - a.length)
    .find((b) => location.pathname.startsWith(b))!;
  const to = bases[channel];
  const slug = location.pathname.slice(from.length).replace(/\/$/, '');
  if (slug) {
    for (const { slug: candidate, renamedOnly } of candidatesFor(slug, location.hash, el)) {
      const url = `${to}${candidate}/`;
      if (await exists(url)) return renamedOnly ? url + location.hash : url;
    }
  }
  return (await exists(to)) ? to : undefined;
}

// The page a missing URL most likely meant, for the 404 page. GitHub Pages
// answers every missing path with one 404 page, so it may be either channel's
// path. The missing slug is tidied the way old links were written (a file name
// such as CONFIGURATION.md), then matched in its own channel first and the
// other channel second. With no match it lands on its own channel's root. Only
// a URL that answers, and is not this one, is ever a target, so it cannot loop.
export async function redirectFor(el: HTMLElement): Promise<string | undefined> {
  const bases: Record<string, string> = JSON.parse(el.dataset.bases!);
  const path = location.pathname;
  const ordered = Object.entries(bases).sort(([, a], [, b]) => b.length - a.length);
  const match = ordered.find(([, base]) => path.startsWith(base));
  if (!match || /\/404(\.html)?$/.test(path)) return undefined;
  const [here, base] = match;
  const slug = path
    .slice(base.length)
    .replace(/^docs\//, '')
    .replace(/\/(index(\.html)?)?$/, '')
    .replace(/\.(md|html)$/i, '')
    .toLowerCase()
    .replaceAll('_', '-');
  const channels = [here, ...Object.keys(bases).filter((c) => c !== here)];
  for (const channel of channels) {
    for (const { slug: candidate, renamedOnly } of candidatesFor(slug, location.hash, el)) {
      const url = `${bases[channel]}${candidate}/`;
      if (url === path) continue;
      if (await exists(url)) return renamedOnly ? url + location.hash : url;
    }
  }
  return base !== path && (await exists(base)) ? base : undefined;
}
