import fs from 'node:fs';
import path from 'node:path';
import { base, changelogPath, docId, docsDir, isPublished, ref, repo } from '../site.mjs';

const repoRoot = path.dirname(docsDir);
const external = /^([a-z][a-z0-9+.-]*:|#|\/\/)/i;

// Renders docs/ as written for GitHub: drops the leading H1 (Starlight prints
// the title), turns GitHub alerts into asides, points links between published
// docs at their site routes, and points links to anything else in the repo at
// GitHub.
export default function remarkNeruDocs() {
  return (tree, file) => {
    if (!file.path?.startsWith(docsDir + path.sep) && file.path !== changelogPath) return;
    if (tree.children[0]?.type === 'heading' && tree.children[0].depth === 1) {
      tree.children.shift();
    }
    // Some docs use further H1s as top-level sections. Starlight's table of
    // contents starts at h2, so those pages shift every heading down a level.
    const headings = tree.children.filter((n) => n.type === 'heading');
    if (headings.some((h) => h.depth === 1)) {
      for (const h of headings) h.depth = Math.min(h.depth + 1, 6);
    }
    const dir = path.dirname(file.path);
    visit(tree, (node) => {
      if (node.type === 'blockquote') toAside(node);
      if (!['link', 'definition', 'image'].includes(node.type) || !node.url) return;
      if (external.test(node.url)) return;
      node.url = rewrite(dir, node.url, node.type === 'image');
    });
  };
}

// Maps each GitHub alert kind to a Starlight aside variant, plus a title when
// the variant's own title would say something else.
const alerts = {
  NOTE: ['note'],
  TIP: ['tip'],
  IMPORTANT: ['note', 'Important'],
  WARNING: ['caution', 'Warning'],
  CAUTION: ['danger', 'Caution'],
};

// Turns a `> [!NOTE]` blockquote into the container directive Starlight's
// asides plugin renders, which runs after this one.
function toAside(node) {
  const text = node.children[0]?.children?.[0];
  const match = text?.type === 'text' && text.value.match(/^\[!([A-Z]+)\][ \t]*\n?/);
  if (!match || !alerts[match[1]]) return;
  const [variant, title] = alerts[match[1]];
  text.value = text.value.slice(match[0].length);
  const first = node.children[0];
  if (!text.value) first.children.shift();
  if (first.children[0]?.type === 'break') first.children.shift();
  if (!first.children.length) node.children.shift();
  const label = title
    ? [{ type: 'paragraph', data: { directiveLabel: true }, children: [{ type: 'text', value: title }] }]
    : [];
  Object.assign(node, {
    type: 'containerDirective',
    name: variant,
    attributes: {},
    children: [...label, ...node.children],
  });
}

function rewrite(dir, url, isImage) {
  const [target, hash = ''] = url.split(/(?=#)/);
  const abs = path.resolve(dir, target);
  const fromDocs = path.relative(docsDir, abs).split(path.sep).join('/');
  if (!isImage && !fromDocs.startsWith('..') && isPublished(fromDocs)) {
    return `${base}${docId(fromDocs)}/${hash}`;
  }
  if (!isImage && abs === changelogPath) return `${base}changelog/${hash}`;
  const fromRoot = path.relative(repoRoot, abs).split(path.sep).join('/');
  if (isImage) return `https://raw.githubusercontent.com/y3owk1n/neru/${ref}/${fromRoot}`;
  const kind = fs.statSync(abs, { throwIfNoEntry: false })?.isDirectory() ? 'tree' : 'blob';
  return `${repo}/${kind}/${ref}/${fromRoot}${hash}`;
}

function visit(node, fn) {
  fn(node);
  for (const child of node.children ?? []) visit(child, fn);
}
