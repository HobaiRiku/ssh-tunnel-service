// Regenerates the runtime icon set in ../public from the SVG masters in this folder.
//
//   pnpm add -D @resvg/resvg-js   # one-off; not a permanent dependency
//   node branding/gen-icons.mjs
//   pnpm remove @resvg/resvg-js
//
// Source of truth:
//   icon.svg          -> self-contained rounded badge (favicon + "any" PWA icon)
//   maskable-icon.svg -> full-bleed badge with safe-zone padding (maskable PWA icon)
//
// public/manifest.json is hand-maintained and references the files emitted here.
import { Resvg } from '@resvg/resvg-js'
import { readFileSync, writeFileSync, copyFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const pub = join(here, '..', 'public')

function rasterize(srcFile, outFile, size) {
  const svg = readFileSync(join(here, srcFile), 'utf8')
  const png = new Resvg(svg, { fitTo: { mode: 'width', value: size } }).render().asPng()
  writeFileSync(join(pub, outFile), png)
  console.log(`${outFile}  ${size}x${size}`)
}

// icon.svg ships as-is for the SVG favicon / "any" icon.
copyFileSync(join(here, 'icon.svg'), join(pub, 'icon.svg'))

rasterize('icon.svg', 'pwa-192x192.png', 192)
rasterize('icon.svg', 'pwa-512x512.png', 512)
rasterize('maskable-icon.svg', 'pwa-maskable-512x512.png', 512)
rasterize('maskable-icon.svg', 'apple-touch-icon.png', 180)
