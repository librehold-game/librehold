"""Exports the pixel map: Wang atlases of the lake, forest and mountain tilesets, the tree and peak sprites and the
village tiers (`villages/picks.txt`: tier and candidate).
Usage: python3 scripts/pixel/map.py <art/pixel-spike/map> <src/assets/pixel/map>

A village tier comes in four images of one size: `village-<t>` (red roofs, the player's own), `-barb` (slate roofs),
`-tint` (grey roofs to be coloured by the owner) and `-roof` (the roof pixels, a mask for that colour). The banner
(`villages/banner/<BANNER>.png`) has a white cloth; `banner-cloth` masks it for the bonus and active colours.
Meadow variety: ground decals (`decals/NN.png`, feathered into an oval at 0.7 opacity), flower clumps and decorations
(`decor/NN.png`).

An atlas holds 16 cells of 32 px; the cell for mask m sits at (m % 4, m // 4). Mask bits: 8 NW, 4 NE, 2 SW, 1 SE, a
set bit meaning that corner is the biome (the lower terrain of the lake tileset, the upper one of the others)."""
import colorsys
import json
import os
import sys
from pathlib import Path

from PIL import Image

A, OUT = Path(sys.argv[1]), Path(sys.argv[2])
SCENERY = Path(__file__).resolve().parents[2] / 'src' / 'assets' / 'pixel' / 'village' / 'scenery'

TILESETS = {'water': ('lake-pro', 'lower'), 'forest': ('forest', 'upper'), 'mountain': ('mountain', 'upper')}
PEAKS = ['00', '01', '04', '05', '08', '12', '13']
BANNER = '13'
DECALS = {'dry': ['00', '02', '03'], 'dirt': ['12', '14']}
FLOWERS = ['16', '19', '20', '21', '22']
DECOR = ['01', '05', '06', '09', '12', '49', '24', '26', '58', '33', '60', '63']
CELL = 32


def atlas(name, biome_corner):
    d = A / 'tilesets' / name
    sheet = Image.open(d / 'sheet.png').convert('RGBA')
    meta = json.loads((d / 'meta.json').read_text())
    tiles = meta['tileset_data']['tiles'] if 'tileset_data' in meta else meta['tiles']
    out = Image.new('RGBA', (4 * CELL, 4 * CELL))
    filled = set()
    for t in tiles:
        c = t['corners']
        mask = sum(bit for k, bit in (('NW', 8), ('NE', 4), ('SW', 2), ('SE', 1)) if c[k] == biome_corner)
        if mask in filled:
            continue
        b = t['bounding_box']
        cell = sheet.crop((b['x'], b['y'], b['x'] + b['width'], b['y'] + b['height']))
        out.paste(cell, (mask % 4 * CELL, mask // 4 * CELL))
        filled.add(mask)
    missing = set(range(16)) - filled
    if missing:
        sys.exit(f'{name}: no tile for masks {sorted(missing)}')
    return out


def scaled(path, k):
    im = Image.open(path).convert('RGBA')
    return im.resize((round(im.width * k), round(im.height * k)), Image.LANCZOS)


def is_roof(r, g, b):
    h, s, _ = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
    return (h < 0.04 or h > 0.93) and s > 0.35


def village_variants(im):
    """Barbarian, tintable and roof-mask variants of a red-roofed village sprite."""
    barb, tint, roof = im.copy(), im.copy(), Image.new('RGBA', im.size)
    src, bp, tp, rp = im.load(), barb.load(), tint.load(), roof.load()
    for y in range(im.height):
        for x in range(im.width):
            r, g, b, a = src[x, y]
            if not a:
                continue
            h, s, v = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
            if is_roof(r, g, b):
                bp[x, y] = (*(round(c * 255) for c in colorsys.hsv_to_rgb(0.6, s * 0.18, v * 0.72)), a)
                grey = round(min(1, v * 1.35) * 255)
                tp[x, y] = (grey, grey, grey, a)
                rp[x, y] = (255, 255, 255, a)
            else:
                bp[x, y] = (*(round(c * 255) for c in colorsys.hsv_to_rgb(h, s * 0.7, v * 0.9)), a)
    return barb, tint, roof


def soften(im, opacity=0.7):
    """Feathers a ground decal into an oval, so it reads as a tone change in the meadow."""
    im = im.copy()
    w, h = im.size
    px = im.load()
    for y in range(h):
        for x in range(w):
            r, g, b, a = px[x, y]
            d = ((x - w / 2) / (w / 2)) ** 2 + ((y - h / 2) / (h / 2)) ** 2
            f = max(0.0, min(1.0, (1 - d) / 0.45))
            px[x, y] = (r, g, b, round(a * f * f * (3 - 2 * f) * opacity))
    return im


def main():
    os.makedirs(OUT, exist_ok=True)
    for biome, (name, corner) in TILESETS.items():
        atlas(name, corner).save(OUT / f'wang-{biome}.png', optimize=True)
    for tree in ('oak', 'pine'):
        scaled(SCENERY / f'{tree}.png', 0.5).save(OUT / f'tree-{tree}.png', optimize=True)
    for i, p in enumerate(PEAKS):
        scaled(A / 'peaks' / f'{p}.png', 1.15).save(OUT / f'peak-{i}.png', optimize=True)
    sizes = {}
    for line in (A / 'villages' / 'picks.txt').read_text().split('\n'):
        if not line.strip():
            continue
        tier, pick = line.split()
        im = Image.open(A / 'villages' / f'tier-{tier}' / f'{pick}.png').convert('RGBA')
        im = im.crop(im.getbbox())
        barb, tint, roof = village_variants(im)
        sizes[f'village-{tier}'] = list(im.size)
        for suffix, v in (('', im), ('-barb', barb), ('-tint', tint), ('-roof', roof)):
            v.save(OUT / f'village-{tier}{suffix}.png', optimize=True)
    banner = Image.open(A / 'villages' / 'banner' / f'{BANNER}.png').convert('RGBA')
    banner = banner.crop(banner.getbbox())
    cloth = Image.new('RGBA', banner.size)
    bp, cp = banner.load(), cloth.load()
    for y in range(banner.height):
        for x in range(banner.width):
            r, g, b, a = bp[x, y]
            _, sat, v = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
            if a and sat < 0.15 and v > 0.6:
                cp[x, y] = (255, 255, 255, a)
    banner.save(OUT / 'banner.png', optimize=True)
    cloth.save(OUT / 'banner-cloth.png', optimize=True)
    sizes['banner'] = list(banner.size)
    (OUT / 'villages.json').write_text(json.dumps(sizes, indent=1) + '\n')
    for kind, picks in DECALS.items():
        for i, p in enumerate(picks):
            soften(Image.open(A / 'decals' / f'{p}.png').convert('RGBA')).save(OUT / f'decal-{kind}-{i}.png', optimize=True)
    for name, picks in (('flower', FLOWERS), ('decor', DECOR)):
        for i, p in enumerate(picks):
            im = Image.open(A / 'decor' / f'{p}.png').convert('RGBA')
            im.crop(im.getbbox()).save(OUT / f'{name}-{i}.png', optimize=True)
    print(OUT, sorted(os.listdir(OUT)))


if __name__ == '__main__':
    main()
