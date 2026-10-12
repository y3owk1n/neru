// The repository's star count, read from the GitHub API once per build. A
// visitor's browser never asks GitHub for it, so viewing a page sends nothing
// to a third party. A failed request leaves the count out instead of failing
// the build, so an offline build or a rate limit just shows no number.
import { env } from 'node:process';
import { repo } from './site.mjs';

// NERU_GITHUB_API points the request elsewhere, which lets you try a build
// without the network.
const api = env.NERU_GITHUB_API ?? 'https://api.github.com';
const slug = new URL(repo).pathname.replace(/^\/|\/$/g, '');

let pending;

/** The star count, or null when GitHub could not be asked. */
export function starCount() {
  pending ??= readStars();
  return pending;
}

async function readStars() {
  const headers = { Accept: 'application/vnd.github+json', 'User-Agent': 'neru-website' };
  if (env.GITHUB_TOKEN) headers.Authorization = `Bearer ${env.GITHUB_TOKEN}`;

  try {
    const response = await fetch(`${api}/repos/${slug}`, {
      headers,
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) return null;

    const { stargazers_count: count } = await response.json();
    return Number.isInteger(count) ? count : null;
  } catch {
    return null;
  }
}

/** A count as the header shows it: exact under 1,000, then 1.2k or 12k. */
export function formatStars(count) {
  if (count < 1000) return String(count);

  const thousands = count / 1000;
  return `${thousands.toFixed(thousands < 10 ? 1 : 0).replace(/\.0$/, '')}k`;
}
