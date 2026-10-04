const http = require('http'), fs = require('fs'), path = require('path');
const root = path.join(__dirname, 'dist');
const types = {'.html':'text/html; charset=utf-8','.js':'text/javascript','.css':'text/css',
  '.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg',
  '.webp':'image/webp','.ico':'image/x-icon','.woff2':'font/woff2',
  '.xlsx':'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'};
const port = process.env.SERVER_PORT || process.env.PORT || 3000;

http.createServer((req, res) => {
  let f;
  try {
    const p = path.normalize(decodeURIComponent(req.url.split('?')[0]));
    f = path.join(root, p);
  } catch { f = root; }
  if (!f.startsWith(root) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
    f = path.join(root, 'index.html');
  }
  res.writeHead(200, {'Content-Type': types[path.extname(f)] || 'application/octet-stream'});
  fs.createReadStream(f).pipe(res);
}).listen(port, '0.0.0.0', () => console.log('listening on', port));
