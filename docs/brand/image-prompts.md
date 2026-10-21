# Aletheia: image prompts

Use these prompts for photorealistic brand imagery in image generators (Midjourney, Firefly, DALL·E / GPT Image, Imagen, Flux, Ideogram). Run each prompt in every tool you use, keep the best two or three results per asset, and we'll choose from the pool.

## Art direction (shared by all prompts)

- **Subject:** a classic admiralty anchor (ring, iron stock, straight shank, curved arms, spade-shaped flukes). Forged steel or dark iron. Weathered but intact, not rusty junk.
- **Story:**
  - The river Lethe is forgetting; the anchor is memory that holds against it.
  - Calm, deep, dark water.
  - A few faint glints drifting past are forgotten things.
  - The anchor and its chain hold firm.
- **Mood:** quiet, precise, trustworthy. Think premium developer tool or engineering firm, not fantasy or pirate.
- **Palette:**
  - deep ink navy (#0b1220 to #16233a);
  - cold steel greys;
  - one restrained accent of verification green (#1d9e75), only as a faint light, never as paint;
  - light variants use a warm parchment white (#f4f1ea).
- **Light:** soft directional light from above, volumetric rays in water scenes, crisp specular highlights on steel.
- **Camera:** full-frame, 50–85 mm look, shallow depth of field where it helps, no fisheye.
- **Composition:** leave clean negative space for a headline (noted per asset).
- **Avoid:**
  - text, letters, logos, watermarks;
  - pirates, ships, sailors, treasure, skulls;
  - cartoon or illustration style;
  - heavy rust, barnacle clutter;
  - neon, oversaturated colors;
  - extra or deformed anchor parts (two stocks, three arms).

For Midjourney append `--style raw --v 7 --q 2` plus the aspect ratio given. Use the "Negative" line as `--no …`. Most other tools take the negative as a separate field, or as an "Avoid: …" sentence at the end of the prompt.

---

## 1. Landing page hero (dark), 16:9

> Photorealistic underwater photograph of a single classic admiralty anchor resting upright on a dark sandy riverbed, forged dark steel with subtle wear, its heavy chain rising out of frame toward the surface, calm deep navy water, soft volumetric light rays falling from above, a few faint particles drifting past in the current, crisp specular highlights on the steel, the anchor placed on the right third of the frame, wide empty dark water on the left for a headline, quiet and precise mood, premium tech brand photography, shot on full-frame camera, 50mm, high dynamic range

- Midjourney: `--ar 16:9`
- Negative: text, logo, watermark, pirate, ship, treasure, fish swarm, cartoon, illustration, heavy rust, barnacles, neon colors, deformed anchor, two stocks

## 2. Landing page hero (light variant), 16:9

> Studio product photograph of a classic admiralty anchor in forged dark steel standing on a warm off-white stone surface, soft diffused daylight from the upper left, long gentle shadow, a short length of heavy chain coiled at its base, clean minimal composition with the anchor on the right third and generous empty warm-white space on the left, precise and calm, premium engineering brand still life, 85mm, very sharp, natural color

- Midjourney: `--ar 16:9`
- Negative: text, logo, watermark, clutter, props, rust flakes, cartoon, illustration, colored background, deformed anchor

## 3. README banner, 1280×400 (3.2:1)

> Cinematic wide photograph of a heavy anchor chain descending diagonally into calm dark water, the top of a classic admiralty anchor just visible beneath the surface, deep navy tones, soft light on the water surface, subtle reflections, lots of calm empty space on the left half for a title, minimal and professional, photorealistic

- Midjourney: `--ar 16:5`
- Negative: text, logo, watermark, boat, people, waves crashing, storm, cartoon, illustration, oversaturated

## 4. "How it works" section: the river of forgetting, 3:2

> Photorealistic long-exposure photograph of a dark, slow-moving river at night, the water rendered smooth and silky, a single anchor chain entering the water and pulled taut, holding firm against the current, faint soft light streaks in the water like things being carried away, deep navy and steel-grey palette with one barely visible cool green glint near the chain, quiet, atmospheric, fine-art photography

- Midjourney: `--ar 3:2`
- Negative: text, logo, watermark, people, boat, bridge, city lights, fantasy, cartoon, neon

## 5. Detail macro: verification, 1:1

> Extreme close-up macro photograph of the ring and shackle of a forged steel anchor, precise machining marks and subtle wear, cold steel tones against a deep navy out-of-focus background, a tiny restrained green reflection on the polished edge, razor-sharp detail, shallow depth of field, premium product macro photography

- Midjourney: `--ar 1:1`
- Negative: text, logo, watermark, rust, dirt, cartoon, illustration, oversaturated green

## 6. Social preview / Open Graph image, 1200×630 (1.91:1)

> Photorealistic dark underwater scene, a classic admiralty anchor seen in clean silhouette with soft rim light on its steel edges, centered slightly to the right, deep navy water with gentle light rays from above, simple and iconic composition with clear empty space on the left for a title, premium and minimal

- Midjourney: `--ar 191:100`
- Negative: text, logo, watermark, fish, divers, ship, cartoon, illustration, clutter

## 7. Logo reference (for vectorizing, not used directly), 1:1

> Clean product photograph of a classic admiralty anchor shown straight-on and perfectly symmetrical, isolated on a plain light grey background, even soft lighting, every part clearly readable: ring, stock, shank, curved arms, spade-shaped flukes, sharp silhouette, no perspective distortion

- Midjourney: `--ar 1:1`
- Negative: text, logo, watermark, chain, rope, background props, perspective, tilt, shadows, rust

---

## Picking from the pool

- The anchor's anatomy must be correct: exactly one stock, two arms, two flukes. Generators often get this wrong.
- No text or watermark artifacts.
- Each image must leave a calm area for the headline, and the light and dark images must sit together as one set.
- Check each tool's license for commercial use before an image goes into the repo.
