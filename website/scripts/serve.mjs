// Serves website/dist/site the way GitHub Pages does: files under NERU_BASE,
// directories through their index.html, a directory named without its
// trailing slash redirected to it, and the root 404.html with status 404 for
// anything else. Run after build.sh.
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';

const root = path.join(path.dirname(new URL(import.meta.url).pathname), '..', 'dist', 'site');
const base = `/${(process.env.NERU_BASE ?? '/').replace(/^\/+|\/+$/g, '')}/`.replace('//', '/');
const port = Number(process.env.PORT ?? 4321);
const types = {
  '.html': 'text/html; charset=utf-8', '.css': 'text/css', '.js': 'text/javascript',
  '.json': 'application/json', '.png': 'image/png', '.jpg': 'image/jpeg', '.svg': 'image/svg+xml',
  '.mp4': 'video/mp4', '.woff2': 'font/woff2', '.xml': 'application/xml', '.txt': 'text/plain',
};

function resolve(urlPath) {
  if (!urlPath.startsWith(base)) return undefined;
  const file = path.join(root, decodeURIComponent(urlPath.slice(base.length)));
  if (!file.startsWith(root)) return undefined;
  const stat = fs.statSync(file, { throwIfNoEntry: false });
  if (stat?.isFile()) return file;
  if (stat?.isDirectory() && fs.existsSync(path.join(file, 'index.html'))) return path.join(file, 'index.html');
  return undefined;
}

http
  .createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://localhost');
    const urlPath = url.pathname;
    if (!urlPath.endsWith('/') && resolve(`${urlPath}/`)?.endsWith('index.html')) {
      res.writeHead(301, { location: `${urlPath}/${url.search}` });
      return res.end();
    }
    const file = resolve(urlPath);
    const body = file ?? path.join(root, '404.html');
    const headers = {
      'content-type': types[path.extname(body)] ?? 'application/octet-stream',
      'accept-ranges': 'bytes',
    };
    // Safari plays video only from a server that answers byte ranges.
    const size = fs.statSync(body).size;
    const range = file && req.headers.range?.match(/^bytes=(\d*)-(\d*)$/);
    if (range) {
      const start = range[1] ? Number(range[1]) : size - Number(range[2]);
      const end = range[1] && range[2] ? Math.min(Number(range[2]), size - 1) : size - 1;
      res.writeHead(206, { ...headers, 'content-range': `bytes ${start}-${end}/${size}`, 'content-length': end - start + 1 });
      if (req.method === 'HEAD') return res.end();
      return fs.createReadStream(body, { start, end }).pipe(res);
    }
    res.writeHead(file ? 200 : 404, { ...headers, 'content-length': size });
    if (req.method === 'HEAD') return res.end();
    fs.createReadStream(body).pipe(res);
  })
  .listen(port, '127.0.0.1', () => console.log(`serve.mjs: http://127.0.0.1:${port}${base}`));
