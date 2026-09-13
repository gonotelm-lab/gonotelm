npx esbuild entry.cjs \
  --bundle \
  --platform=node \
  --format=cjs \
  --outfile=../../internal/infrastructure/sandbox/workspace/vendor/standalone.cjs

npx esbuild entry-browser.js \
  --bundle \
  --platform=browser \
  --format=iife \
  --minify \
  --outfile=../../internal/infrastructure/sandbox/workspace/vendor/gsap.browser.js
