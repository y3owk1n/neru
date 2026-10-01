// Finds the page a reader is on in the other docs channel. A page renamed
// between channels is found under its other name. A page the other channel
// lacks lands on the page it was split from, else its first guide page, else
// its home. The channel bases, renamed and split pages arrive as data
// attributes on `el`, because site.mjs reads the build environment and cannot
// run in the browser.

const exists = (url: string) => fetch(url, { method: 'HEAD' }).then((r) => r.ok, () => false);

export async function pageIn(channel: string, el: HTMLElement): Promise<string> {
  const bases: Record<string, string> = JSON.parse(el.dataset.bases!);
  const renamed: Record<string, string> = JSON.parse(el.dataset.renamed!);
  const split: Record<string, string> = JSON.parse(el.dataset.split!);
  const from = Object.values(bases)
    .sort((a, b) => b.length - a.length)
    .find((b) => location.pathname.startsWith(b))!;
  const to = bases[channel];
  const slug = location.pathname.slice(from.length).replace(/\/$/, '');
  const other = renamed[slug] ?? Object.keys(renamed).find((old) => renamed[old] === slug);
  const candidates = [slug, other, split[slug], 'guide/getting-started', 'installation']
    .filter((s): s is string => Boolean(s))
    .map((s) => `${to}${s}/`);
  for (const url of candidates) {
    if (await exists(url)) return url === candidates[0] ? url + location.hash : url;
  }
  return to;
}
