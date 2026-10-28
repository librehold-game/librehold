"""Exports the pixel-art UI kit for the classic skin: chrome pieces cut from the kits and the picked icons.
Usage: python3 scripts/pixel/ui.py <art/pixel-spike/ui> <src/assets/pixel/ui>"""
import os
import shutil
import sys

from PIL import Image, ImageChops

A, OUT = sys.argv[1], sys.argv[2]

CHROME = {
    'header': ('kit-a.png', (15, 16, 496, 68)),
    'btn-danger': ('kit-a.png', (15, 84, 217, 142)),
    'input': ('kit-a.png', (15, 164, 337, 209)),
    'frame': ('kit-a.png', (8, 228, 503, 502)),
    'tab': ('kit-b.png', (23, 23, 186, 80)),
    'tab-active': ('kit-b.png', (209, 23, 371, 80)),
    'iconbtn-active': ('kit-b.png', (19, 119, 98, 200)),
    'thumb': ('kit-b.png', (412, 23, 456, 252)),
    'panel': ('ui-sample.png', (94, 285, 418, 454)),
    'btn': ('ui-sample.png', (64, 64, 226, 129)),
}


def components(im):
    """Opaque 8-connected pixel groups of an image."""
    W, H = im.size
    a = im.getchannel('A').load()
    seen, comps = set(), []
    for y in range(H):
        for x in range(W):
            if a[x, y] and (x, y) not in seen:
                stack, comp = [(x, y)], []
                seen.add((x, y))
                while stack:
                    cx, cy = stack.pop()
                    comp.append((cx, cy))
                    for dx in (-1, 0, 1):
                        for dy in (-1, 0, 1):
                            nx, ny = cx + dx, cy + dy
                            if 0 <= nx < W and 0 <= ny < H and a[nx, ny] and (nx, ny) not in seen:
                                seen.add((nx, ny))
                                stack.append((nx, ny))
                comps.append(comp)
    return comps


def solid(im):
    """Hard alpha (generator noise below 128 dropped) and only the largest shape, cropped to it."""
    im = im.copy()
    a = im.getchannel('A').point(lambda v: 255 if v >= 128 else 0)
    im.putalpha(a)
    comps = components(im)
    main = max(comps, key=len)
    keep = Image.new('L', im.size, 0)
    kp = keep.load()
    for x, y in main:
        kp[x, y] = 255
    im.putalpha(ImageChops.multiply(a, keep))
    return im.crop(im.getbbox())


def clean(im):
    """Drop detached fragments that touch the canvas edge (bleed from neighbouring candidates) and centre the rest."""
    W, H = im.size
    comps = components(im)
    if not comps:
        return im
    main = max(comps, key=len)
    out = im.copy()
    px = out.load()
    for comp in comps:
        if comp is not main and any(x in (0, W - 1) or y in (0, H - 1) for x, y in comp):
            for x, y in comp:
                px[x, y] = (0, 0, 0, 0)
    bb = out.getbbox()
    return ImageChops.offset(out, (W - (bb[0] + bb[2])) // 2, (H - (bb[1] + bb[3])) // 2)


def unwrap(im):
    """Roll art that came back wrapped around the canvas edge so the longest empty run sits at the border, then centre."""
    a = im.getchannel('A')
    for axis in (0, 1):
        n = im.width if axis == 0 else im.height
        empty = [not (a.crop((i, 0, i + 1, im.height)) if axis == 0 else a.crop((0, i, im.width, i + 1))).getbbox() for i in range(n)]
        if all(empty) or not any(empty):
            continue
        best, start = (0, 0), None
        for i in range(2 * n):
            if empty[i % n]:
                start = i if start is None else start
                if i - start + 1 > best[0] and i - start + 1 <= n:
                    best = (i - start + 1, start)
            else:
                start = None
        length, first = best
        shift = (n - (first % n) - length // 2) % n
        im = ImageChops.offset(im, shift if axis == 0 else 0, shift if axis == 1 else 0)
        a = im.getchannel('A')
    bb = im.getbbox()
    return ImageChops.offset(im, (im.width - (bb[0] + bb[2])) // 2, (im.height - (bb[1] + bb[3])) // 2)


def plain_button(btn):
    """The sample button has "PLAY" painted on it: keep its edges and fill the middle with a clean strip of the plank."""
    left, strip, right = btn.crop((0, 0, 36, btn.height)), btn.crop((20, 0, 36, btn.height)), btn.crop((btn.width - 36, 0, btn.width, btn.height))
    out = Image.new('RGBA', (36 + 16 + 36, btn.height))
    out.paste(left, (0, 0))
    out.paste(strip, (36, 0))
    out.paste(right, (52, 0))
    return out


if not os.path.normpath(OUT).endswith(os.path.join('assets', 'pixel', 'ui')):
    sys.exit(f'refusing to replace {OUT}: expected a path ending in assets/pixel/ui')
shutil.rmtree(OUT, ignore_errors=True)
os.makedirs(f'{OUT}/chrome')
os.makedirs(f'{OUT}/icons')
for name, (src, box) in CHROME.items():
    piece = solid(Image.open(f'{A}/{src}').convert('RGBA').crop(box))
    if name == 'btn':
        piece = plain_button(piece)
    piece.save(f'{OUT}/chrome/{name}.png', optimize=True)
    if name == 'header':
        piece.crop((24, 0, piece.width - 24, piece.height)).save(f'{OUT}/chrome/plank.png', optimize=True)
leather = Image.open(f'{A}/bg/leather-1.png').convert('RGBA')
leather.crop((3, 3, leather.width - 3, leather.height - 3)).save(f'{OUT}/chrome/leather.png', optimize=True)

for line in open(f'{A}/icon-picks.txt'):
    parts = line.split()
    if not parts:
        continue
    name, index, method = parts[0], int(parts[1]), parts[2] if len(parts) > 2 else 'clean'
    icon = Image.open(f'{A}/icons/{name}/{index:02d}.png').convert('RGBA')
    (unwrap(icon) if method == 'unwrap' else clean(icon)).save(f'{OUT}/icons/{name}.png', optimize=True)
print('exported', len(os.listdir(f'{OUT}/chrome')), 'chrome pieces and', len(os.listdir(f'{OUT}/icons')), 'icons')
