"""Exports the pixel-art village for the game: ground, road overlays, bbox-cropped sprites, frame strips and layout.json.
Usage: python3 scripts/pixel/export.py <art/pixel-spike> <src/assets/pixel/village>"""
import glob
import json
import math
import os
import shutil
import subprocess
import sys
import tempfile

os.environ['SCENE_SCALE'] = '1.1,1.2'
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
A, OUT = sys.argv[1], sys.argv[2]
sys.argv = sys.argv[:1]
import ground as G
import parts
from PIL import Image

files = {}


def save(img, rel):
    os.makedirs(os.path.dirname(f'{OUT}/{rel}'), exist_ok=True)
    img.save(f'{OUT}/{rel}', optimize=True)
    files[rel] = list(img.size)
    return rel


def numbered(pattern):
    return sorted(glob.glob(pattern), key=lambda s: int(s.rsplit('_', 1)[1].split('.')[0]))


def strip(frames):
    w, h = frames[0].size
    out = Image.new('RGBA', (w * len(frames), h))
    for i, f in enumerate(frames):
        out.paste(f, (i * w, 0))
    return out


def union_crop(frames):
    """Crop all frames to their common bbox so the animation does not jitter."""
    boxes = [f.getbbox() for f in frames if f.getbbox()]
    bb = (min(b[0] for b in boxes), min(b[1] for b in boxes), max(b[2] for b in boxes), max(b[3] for b in boxes))
    return [f.crop(bb) for f in frames]


def r1(v):
    return round(v, 1)


def origin(anchor, size):
    """Top-left corner of a sprite anchored at its bottom-centre — same rounding as `Img` in the game."""
    return math.floor(anchor[0] - size[0] / 2), math.floor(anchor[1] - size[1])


def hit_path(img, cell=4):
    """Clickable shape of a sprite: its opaque area as 4 px cells, merged into horizontal runs (relative to top-left)."""
    a = img.getchannel('A')
    W, H = img.size
    parts_ = []
    for y in range(0, H, cell):
        x = 0
        while x < W:
            if a.crop((x, y, min(W, x + cell), min(H, y + cell))).getbbox():
                x0 = x
                while x < W and a.crop((x, y, min(W, x + cell), min(H, y + cell))).getbbox():
                    x += cell
                w, h = min(W, x) - x0, min(H, y + cell) - y
                parts_.append(f'M{x0} {y}h{w}v{h}h-{w}z')
            else:
                x += cell
    return ''.join(parts_)


def light(at, sprite, anchor):
    pts = [(x, y) for y in range(sprite.height) for x in range(sprite.width) if parts.hot(sprite.getpixel((x, y)))]
    if not pts:
        return None
    ox, oy = origin(anchor, sprite.size)
    return {'x': r1(ox + sum(p[0] for p in pts) / len(pts)), 'y': r1(oy + sum(p[1] for p in pts) / len(pts)), 'r': 28, 'at': at}


shutil.rmtree(OUT, ignore_errors=True)
os.makedirs(OUT)
tmp = tempfile.mkdtemp()

walls, ring, gates = {}, None, None
for kind, name, gate_dir in (('stone', 'wall-ring-big', 'stone-gate'), ('palisade', 'wall-ring-palisade', 'palisade-gate')):
    r = parts.ring_split(A, name, G.RING)
    ring, gates = r['ring'], r['gates']
    gfs = union_crop([Image.open(f).convert('RGBA') for f in numbered(f'{A}/anim/{gate_dir}/frame_*.png')])
    gw, gh = gfs[0].size
    walls[kind] = {
        'x': r['x'], 'y': r['y'], 'w': r['back'].width, 'h': r['back'].height,
        'back': save(r['back'], f'walls/{kind}-back.png'), 'front': save(r['front'], f'walls/{kind}-front.png'),
        'strips': r['strips'],
        'gate': {'file': save(strip(gfs), f'walls/{kind}-gate.png'), 'w': gw, 'h': gh, 'frames': len(gfs),
                 'x': r1(gates['s'][0]), 'y': r1(gates['s'][1] + 4), 'foot': gfs[0].getbbox()[3]},
    }

env = dict(os.environ, GATES=json.dumps({k: list(v) for k, v in gates.items()}), BASE=f'{tmp}/base.png', EXPORT_DIR=f'{tmp}/ex')
subprocess.run([sys.executable, f'{HERE}/ground.py', f'{tmp}/base.png'], env=env, check=True)
subprocess.run([sys.executable, f'{HERE}/roads.py', A, f'{tmp}/preview.png'], env=env, check=True)
save(Image.open(f'{tmp}/ex/ground.png').convert('RGBA'), 'ground.png')
roads = {}
for key, meta in json.load(open(f'{tmp}/ex/roads.json')).items():
    roads[key] = {'file': save(Image.open(f'{tmp}/ex/roads/{key}.png').convert('RGBA'), f'roads/{key}.png'),
                  'x': meta['x'], 'y': meta['y'], 'points': meta['points']}

buildings, sprites = {}, {}
for b in G.SLOTS:
    src = {1: f'{b}-1.png', 2: f'{b}.png', 3: f'{b}-3.png'}
    stages = {3: src[3]} if b == 'statue' else {v: s if os.path.exists(f'{A}/buildings/{s}') else f'{b}.png' for v, s in src.items()}
    buildings[b] = {}
    for v, s in stages.items():
        sprites[b, v] = parts.unplated(f'buildings/{s}', parts.load(f'{A}/buildings/{s}'))
        buildings[b][v] = save(sprites[b, v], f'buildings/{b}-{v}.png')
construction_img = parts.load(f'{A}/scenery/construction.png')
construction = save(construction_img, 'buildings/construction.png')
place_img = parts.load(f'{A}/buildings/place.png')
place = save(place_img, 'buildings/place.png')

scenery = []
for src, x, y, z, flip in parts.scenery(A, ring):
    rel = f'scenery/{os.path.basename(src)}'
    if rel not in files:
        save(parts.unplated(src, parts.load(f'{A}/{src}')), rel)
    item = {'file': rel, 'x': r1(x), 'y': r1(y), 'z': r1(z)}
    if flip:
        item['flip'] = True
    scenery.append(item)

anim = {}
for b in ('iron', 'farm'):
    fs = union_crop([Image.open(f).convert('RGBA') for f in numbered(f'{A}/anim/{b}/frame_*.png')])
    anim[b] = {'file': save(strip(fs), f'anim/{b}.png'), 'w': fs[0].width, 'h': fs[0].height, 'frames': len(fs), 'stage': 2}
wheel = [Image.open(f'{A}/anim/wood-wheel/frame_{i}.png').convert('RGBA').crop((10, 8, 36, 44)) for i in range(1, 9)]
wood_raw = Image.open(f'{A}/buildings/wood.png').convert('RGBA')
bx0, by0 = wood_raw.getbbox()[:2]
wx, wy = origin((G.SLOTS['wood'][0], G.SLOTS['wood'][1] + 12), sprites['wood', 2].size)
anim['wood'] = {'file': save(strip(wheel), 'anim/wood.png'), 'w': wheel[0].width, 'h': wheel[0].height, 'frames': len(wheel), 'stage': 2,
                'x': wx + 90 - bx0, 'y': wy + 44 - by0}

ST = f'{A}/villager-states'
VILLAGERS = {
    'walk': (f'{ST}/Idle/animations/walk', ('south', 'east', 'north', 'west'), slice(0, 6)),
    'carry_log': (f'{ST}/Carry_log/animations/walk', ('south', 'east', 'north', 'west'), slice(1, 9)),
    'carry_sack': (f'{ST}/Carry_sack/animations/walk', ('south', 'east', 'north', 'west'), slice(1, 9)),
    'spear': (f'{A}/units/spear/Idle/animations/walk', ('south', 'east', 'north', 'west'), slice(0, 6)),
    'chop': (f'{ST}/Idle/animations/chop-axe', ('east',), slice(0, 3)),
    'hammer': (f'{ST}/Builder/animations/hammer', ('east',), slice(0, 3)),
}
villagers = {}
for state, (d, dirs, sl) in VILLAGERS.items():
    by_dir = {k: [Image.open(f).convert('RGBA') for f in sorted(glob.glob(f'{d}/{k}/*.png'))][sl] for k in dirs}
    boxes = [f.getbbox() for fs in by_dir.values() for f in fs if f.getbbox()]
    w, h = next(iter(by_dir.values()))[0].size
    villagers[state] = {
        'file': {k: save(strip(fs), f'villagers/{state}-{k}.png') for k, fs in by_dir.items()},
        'w': w, 'h': h, 'frames': len(next(iter(by_dir.values()))),
        'anchor': [round((min(b[0] for b in boxes) + max(b[2] for b in boxes)) / 2), max(b[3] for b in boxes)],
    }

slot_anchor = {b: (x, y + 12) for b, (x, y) in G.SLOTS.items()}
lights = [l for l in (light('iron', sprites['iron', 2], slot_anchor['iron']), light('smithy', sprites['smithy', 3], slot_anchor['smithy']),
                      light('rally', place_img, (G.RALLY[0] + 4, G.RALLY[1] + 22))) if l]

cx, cy, rx, ry = ring
layout = {
    'w': G.W, 'h': G.H, 'files': files,
    'ring': {'cx': r1(cx), 'cy': r1(cy), 'rx': r1(rx), 'ry': r1(ry)},
    'slots': {b: [r1(x), r1(y)] for b, (x, y) in G.SLOTS.items()},
    'rally': [r1(v) for v in G.RALLY], 'plaza': [r1(v) for v in G.P(400, 300)],
    'gates': {k: [r1(v[0]), r1(v[1])] for k, v in gates.items()},
    'roads': roads, 'scenery': scenery, 'lights': lights, 'buildings': buildings,
    'construction': construction, 'place': place,
    'hits': {f: hit_path(img) for f, img in [(construction, construction_img), (place, place_img)]
             + [(buildings[b][v], sprites[b, v]) for b, v in sprites]},
    'walls': walls, 'anim': anim, 'villagers': villagers,
}
with open(f'{OUT}/layout.json', 'w') as f:
    json.dump(layout, f, separators=(',', ':'))
shutil.rmtree(tmp)
print('exported', len(files), 'files')
