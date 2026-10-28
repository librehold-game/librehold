"""Exports the army art for the map: marching units, the generic torch and shield groups, boats and ground backdrops.
Rewrites only <out>; a missing piece is left out of army.json and the game draws a stand-in for it.
Usage: python3 scripts/pixel/army.py <art/pixel-spike> <src/assets/pixel/army>"""
import glob
import json
import os
import shutil
import sys

from PIL import Image

A, OUT = sys.argv[1], sys.argv[2]
DIRS = ('south', 'east', 'north', 'west')
UNITS = ('spear', 'sword', 'axe', 'archer', 'scout', 'lightCav', 'mountedArcher', 'heavyCav', 'ram', 'catapult',
         'paladin', 'noble')
GROUNDS = ('meadow', 'forest', 'mountain', 'water')
MAX_FRAMES = 8


def frames_in(d):
    return [Image.open(f).convert('RGBA') for f in sorted(glob.glob(f'{d}/*.png'))][:MAX_FRAMES]


def strip(frames):
    w, h = frames[0].size
    out = Image.new('RGBA', (w * len(frames), h))
    for i, f in enumerate(frames):
        out.paste(f, (i * w, 0))
    return out


def save(img, rel):
    img.save(f'{OUT}/{rel}', optimize=True)
    return rel


def walk_dirs(candidates):
    """The first folder pattern that has frames for all four directions, as {dir: frames}."""
    for pattern in candidates:
        for base in sorted(glob.glob(pattern)):
            by_dir = {d: frames_in(f'{base}/{d}') for d in DIRS}
            if all(by_dir.values()):
                return by_dir
    return None


def still_dirs(base):
    """One still frame per direction from an object's rotations, for engines that roll rather than walk."""
    paths = {d: f'{base}/rotations/{d}.png' for d in DIRS}
    if all(os.path.exists(p) for p in paths.values()):
        return {d: [Image.open(p).convert('RGBA')] for d, p in paths.items()}
    return None


def content_height(by_dir):
    boxes = [f.getbbox() for f in by_dir['south'] if f.getbbox()]
    return max(b[3] for b in boxes) - min(b[1] for b in boxes)


def walk(name, by_dir, height=None):
    """A walk strip per direction; with `height`, a scale that draws the figure as tall as that."""
    n = min(len(fs) for fs in by_dir.values())
    w, h = by_dir['south'][0].size
    boxes = [f.getbbox() for fs in by_dir.values() for f in fs if f.getbbox()]
    out = {
        'file': {d: save(strip(fs[:n]), f'{name}-{d}.png') for d, fs in by_dir.items()},
        'w': w, 'h': h, 'frames': n,
        'anchor': [round((min(b[0] for b in boxes) + max(b[2] for b in boxes)) / 2), max(b[3] for b in boxes)],
    }
    if height:
        out['scale'] = round(height / content_height(by_dir), 3)
    return out


shutil.rmtree(OUT, ignore_errors=True)
os.makedirs(OUT)
manifest = {'units': {}, 'ground': {}}
found_units = {}
for u in UNITS:
    found = walk_dirs([f'{A}/army/units/{u}/walk', f'{A}/army/units/{u}/*/animations/walk*', f'{A}/units/{u}/*/animations/walk*']) \
        or still_dirs(f'{A}/army/siege/{u}')
    if found:
        found_units[u] = found
        manifest['units'][u] = walk(u, found)
# The generic groups were drawn on a larger canvas; they march as tall as a spearman.
foot = content_height(found_units['spear']) if 'spear' in found_units else None
for group in ('torch', 'shield'):
    found = walk_dirs([f'{A}/army/{group}/walk', f'{A}/army/{group}/*/animations/walk*'])
    if found:
        manifest[group] = walk(group, found, foot)
boat = walk_dirs([f'{A}/army/boat', f'{A}/army/boat/animations/*', f'{A}/army/boat/*/animations/*'])
if boat:
    manifest['boat'] = walk('boat', boat)
for g in GROUNDS:
    src = f'{A}/army/ground/{g}.png'
    if os.path.exists(src):
        manifest['ground'][g] = save(Image.open(src).convert('RGBA'), f'ground-{g}.png')
with open(f'{OUT}/army.json', 'w') as f:
    json.dump(manifest, f, indent=1)
    f.write('\n')
print(f"units: {sorted(manifest['units'])}, torch: {'torch' in manifest}, shield: {'shield' in manifest}, "
      f"boat: {'boat' in manifest}, ground: {sorted(manifest['ground'])}")
