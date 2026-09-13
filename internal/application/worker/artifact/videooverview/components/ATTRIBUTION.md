# Attribution & Licenses

The animation components in this directory (`components/<name>/`) come from the
open-source **HyperFrames** project
(<https://github.com/heygen-com/hyperframes>, registry snapshot `v0.8.35`).
Copyright belongs to HeyGen and its contributors, licensed under the
**Apache License 2.0**. The full Apache-2.0 text is in the repository root
`LICENSE`.

To fit this project's offline sandbox and local GSAP, some files were
**modified**:

- Removed the CDN GSAP `<script src>` references from components (the host
  composition loads the local `vendor/gsap.browser.js` instead);
- On use, copy/colors are replaced per this project's palette and language.

## Third-party assets bundled with the components

| Asset | Component(s) | License |
| --- | --- | --- |
| Caveat font (woff2) | `hw-arrow` `hw-boil` `hw-box-label` `hw-callout-circle` `hw-underline` | SIL Open Font License 1.1 |
| Permanent Marker font | `marker-checklist-card` | Apache License 2.0 |
| Courier Prime font | `marker-checklist-card` | SIL Open Font License 1.1 |
| 66 texture maps (sourced from ambientCG) | `texture-mask-text` | ambientCG assets are CC0 (public domain) |

> The table above is compiled from in-file comments and asset provenance;
> confirm the asset licenses before a formal commercial release.
