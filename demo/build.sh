#!/bin/bash
set -e
cd "$(dirname "$0")"
rm -rf dist
bun run build.ts
cat > dist/index.html << 'HTML'
<!DOCTYPE html>
<html lang="zh">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"><title>E2B Demo</title><link rel="stylesheet" href="./frontend.css"></head>
<body><div id="root"></div><script type="module" src="./frontend.js"></script></body>
</html>
HTML
cat > dist/admin.html << 'HTML'
<!DOCTYPE html>
<html lang="zh">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"><title>E2B 管理后台</title><link rel="stylesheet" href="./admin.css"></head>
<body><div id="root"></div><script type="module" src="./admin.js"></script></body>
</html>
HTML
echo "✅ Frontend built to dist/"
