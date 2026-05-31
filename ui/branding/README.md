# Branding / app icons

SVG masters for the app icon. Everything the browser and PWA load at runtime
lives in [`../public`](../public) and is generated from these files.

## The mark

Two hosts joined by a secure tunnel — the project's own metaphor (an SSH `-L`/`-R`
port forward between two endpoints), on the brand blue (`#2563eb`). The same mark
is used in three places so they stay in sync:

- the in-app header (`src/App.vue`, inline SVG)
- the browser favicon (`public/icon.svg`)
- the installed PWA / home-screen icon (the PNGs)

## Files

| File                | Role                                                      |
| ------------------- | --------------------------------------------------------- |
| `icon.svg`          | Self-contained rounded badge. Favicon + `purpose: any`.   |
| `maskable-icon.svg` | Full-bleed badge; artwork kept inside the safe zone so the launcher mask never clips it. Source for the maskable PNG + Apple touch icon. |

## Regenerating `public/`

The rasterizer is intentionally **not** a permanent dependency — install it only
when regenerating:

```bash
pnpm add -D @resvg/resvg-js
node branding/gen-icons.mjs
pnpm remove @resvg/resvg-js
```

This rewrites `public/icon.svg`, `public/pwa-192x192.png`,
`public/pwa-512x512.png`, `public/pwa-maskable-512x512.png`, and
`public/apple-touch-icon.png`. If you add or rename an output, also update
`public/manifest.json` and the `manifest.icons` list in `vite.config.ts`.
