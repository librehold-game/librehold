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

# Where walkers pass through each gate, as [x0, y0, x1, y1) boxes in its frames: between the towers below the lintel (palisade),
# the arch and the paving in front of it (stone). The game draws the gate again over walkers with these left out.
DOORWAY = {'palisade': [[21, 33, 54, 73]], 'stone': [[32, 57, 66, 107]]}

walls, ring, gates = {}, None, None
for kind, name, gate_dir in (('stone', 'wall-ring-big', 'stone-gate'), ('palisade', 'wall-ring-palisade', 'palisade-gate-empty-1')):
    r = parts.ring_split(A, name, G.RING)
    ring, gates = r['ring'], r['gates']
    gfs = union_crop([Image.open(f).convert('RGBA') for f in numbered(f'{A}/anim/{gate_dir}/frame_*.png')])
    gw, gh = gfs[0].size
    gate = {'file': save(strip(gfs), f'walls/{kind}-gate.png'), 'w': gw, 'h': gh, 'frames': len(gfs),
            'x': r1(gates['s'][0]), 'y': r1(gates['s'][1] + 4), 'foot': gfs[0].getbbox()[3], 'doorway': DOORWAY[kind]}
    # The ring runs on behind the gate; open its front half behind the doorway, so the road shows between the open doors.
    gx, gy = math.floor(gate['x'] - gw / 2) - r['x'], math.floor(gate['y'] - gate['foot']) - r['y']
    for x0, _, x1, y1 in DOORWAY[kind]:
        r['front'].paste((0, 0, 0, 0), (gx + x0, gy, gx + x1, gy + y1))
    walls[kind] = {
        'x': r['x'], 'y': r['y'], 'w': r['back'].width, 'h': r['back'].height,
        'back': save(r['back'], f'walls/{kind}-back.png'), 'front': save(r['front'], f'walls/{kind}-front.png'),
        'strips': r['strips'], 'gate': gate,
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

PRO = f'{A}/villager-80-pro'
DIRS = ('south', 'east', 'north', 'west')
# `spear` is the soldier the map's armies march with; the village's own people, its guards included, are the rest.
VILLAGERS = {
    'walk': (f'{PRO}/Idle/animations/walk-12', DIRS, slice(None)),
    'carry_log': (f'{PRO}/Carry_log/animations/walk-12', DIRS, slice(None)),
    'carry_sack': (f'{PRO}/Carry_sack/animations/walk-12', DIRS, slice(None)),
    'guard': (f'{A}/guard-pro/Idle/animations/walk-12', DIRS, slice(None)),
    'spear': (f'{A}/units/spear/Idle/animations/walk', DIRS, slice(0, 6)),
    'chop': (f'{PRO}/Axe/animations/chop', ('east',), slice(None)),
    'hammer': (f'{PRO}/Mallet/animations/hammer-overhead', ('east',), slice(None)),
    'shoot': (f'{A}/archer-pro/Idle/animations/shoot-bow', ('east',), slice(None)),
    'thrust': (f'{A}/spear-pro/Idle/animations/thrust-two-hands', ('east',), slice(None)),
    'slash': (f'{A}/sword-pro/Idle/animations/slash', ('east',), slice(None)),
    'axe_chop': (f'{A}/axe-pro/Idle/animations/axe-chop', ('east',), slice(None)),
    'dig': (f'{PRO}/Shovel/animations/dig', ('east',), slice(None)),
    'forge': (f'{A}/smith-pro/Idle/animations/forge', ('east',), slice(None)),
    'command': (f'{A}/hetman-pro/Idle/animations/command', ('south',), slice(None)),
    # Only the last frame: seated, the tool laid aside; they sit still while the warehouse is full.
    'dig_rest': (f'{PRO}/Shovel/animations/rest-sit', ('east',), slice(7, 8)),
    'chop_rest': (f'{PRO}/Axe/animations/rest-sit', ('east',), slice(7, 8)),
}
# Stable riders walk their horse both ways; the frames are the horse's walk with the unit's rider in the saddle.
for unit in ('scout', 'lightCav', 'mountedArcher', 'heavyCav'):
    VILLAGERS[f'ride_{unit}'] = (f'{A}/stable/{unit}', ('east', 'west'), slice(None))
# Where the workers' boots stand in their frames: their tools, the chopping block and the scythe reach past them.
FEET = {'chop': (24, 42), 'hammer': (25, 41), 'shoot': (22, 36), 'thrust': (22, 37), 'slash': (22, 36), 'axe_chop': (22, 37), 'dig': (24, 42), 'dig_rest': (24, 42), 'chop_rest': (24, 42), 'forge': (22, 37)}
villagers = {}
for state, (d, dirs, sl) in VILLAGERS.items():
    by_dir = {k: [Image.open(f).convert('RGBA') for f in sorted(glob.glob(f'{d}/{k}/*.png'))][sl] for k in dirs}
    boxes = [f.getbbox() for fs in by_dir.values() for f in fs if f.getbbox()]
    w, h = next(iter(by_dir.values()))[0].size
    villagers[state] = {
        'file': {k: save(strip(fs), f'villagers/{state}-{k}.png') for k, fs in by_dir.items()},
        'w': w, 'h': h, 'frames': len(next(iter(by_dir.values()))),
        'anchor': list(FEET[state]) if state in FEET else
        [round((min(b[0] for b in boxes) + max(b[2] for b in boxes)) / 2), max(b[3] for b in boxes)],
    }

# The farmer mows facing east and walks back facing west, and the game swaps one sprite for the other where he stands, so
# both are cut to one box around his boots.
MOWER = {'mow': (f'{A}/farmer-pro/Scythe/animations/mow-scythe', 'east', (24, 41)),
         'scythe_walk': (f'{A}/farmer-pro/Scythe/animations/walk-scythe', 'west', (23, 41))}
mower = {state: [Image.open(f).convert('RGBA') for f in sorted(glob.glob(f'{d}/{k}/*.png'))] for state, (d, k, _) in MOWER.items()}
around = [(b[0] - fx, b[1] - fy, b[2] - fx, b[3] - fy)
          for state, (_, _, (fx, fy)) in MOWER.items() for b in (f.getbbox() for f in mower[state])]
bb = (min(b[0] for b in around), min(b[1] for b in around), max(b[2] for b in around), max(b[3] for b in around))
for state, (_, k, (fx, fy)) in MOWER.items():
    fs = [f.crop((fx + bb[0], fy + bb[1], fx + bb[2], fy + bb[3])) for f in mower[state]]
    villagers[state] = {'file': {k: save(strip(fs), f'villagers/{state}-{k}.png')}, 'w': fs[0].width, 'h': fs[0].height,
                        'frames': len(fs), 'anchor': [-bb[0], -bb[1]]}

# His pass runs east along the wheat field's diamond, a little in front of its middle, and ends where the blade still
# swings over the wheat.
wheat = next(s for s in scenery if s['file'] == 'scenery/plot-wheat.png')
ww = files[wheat['file']][0]
reach = villagers['mow']['w'] - villagers['mow']['anchor'][0]
mow_y = r1(wheat['y'] - ww / 4 + 4)
mow = [[r1(wheat['x'] - ww / 2 + 12), mow_y], [r1(wheat['x'] + ww / 2 - reach - 2), mow_y]]

slot_anchor = {b: (x, y + 12) for b, (x, y) in G.SLOTS.items()}

# Drills in the barracks yard, placed in each stage's own pixels: the unit stands facing east, its props stand at
# the same distance in front of it on every stage. On stage 3 the yard is fenced and each unit has its own spot by the
# hall; stages 1 and 2 only have the open ground in front of the hut. The archer's arrow sticks in the nearer target.
target = save(Image.open(f'{A}/props/target-0.png').convert('RGBA'), 'props/target.png')
dummy_img = Image.open(f'{A}/props/dummy-pro-1.png').convert('RGBA')
dummy = save(dummy_img.crop(dummy_img.getbbox()), 'props/dummy.png')


def local(building, stage, x, y):
    ox, oy = origin(slot_anchor[building], sprites[building, stage].size)
    return [ox + x, oy + y]


# (sprite, where the unit stands on stage 3, its props as (file, offset from the unit))
INFANTRY = {
    'archer': ('shoot', (62, 74), [('target', target, (42, -10)), ('target', target, (35, -2))]),
    'spear': ('thrust', (64, 78), [('dummy', dummy, (30, -3))]),
    'sword': ('slash', (66, 78), [('dummy', dummy, (22, -3))]),
    'axe': ('axe_chop', (64, 78), [('dummy', dummy, (24, -3))]),
}
BARRACKS_GROUND = {1: (22, 80), 2: (64, 94)}
drills = {}
for unit, (sprite, yard, props) in INFANTRY.items():
    drills[unit] = []
    for stage, (ux, uy) in [(1, BARRACKS_GROUND[1]), (2, BARRACKS_GROUND[2]), (3, yard)]:
        u = local('barracks', stage, ux, uy)
        d = {'building': 'barracks', 'stage': stage, 'at': u, 'sprite': sprite,
             'props': [{'name': n, 'file': f, 'at': [u[0] + dx, u[1] + dy]} for n, f, (dx, dy) in props], 'struck': len(props) - 1}
        if unit == 'archer':
            t = d['props'][-1]['at']
            d['arrow'] = {'from': [u[0] + 8, u[1] - 15], 'to': [t[0] - 3, t[1] - 9]}
        drills[unit].append(d)
# Riders in training walk their horse along the stable's front, turn and walk back: on stage 3 inside the paddock (the
# smithy hides its far end), on stage 2 past the open door. Stage 1's open shed has no room for a horse.
RIDE = {2: ((70, 96), (82, 90)), 3: ((51, 80), (69, 71))}
RIDERS = ('scout', 'lightCav', 'mountedArcher', 'heavyCav')
for unit in RIDERS:
    drills[unit] = [{'building': 'stable', 'stage': stage, 'at': local('stable', stage, *a), 'to': local('stable', stage, *b),
                     'sprite': f'ride_{unit}', 'props': [], 'struck': -1} for stage, (a, b) in RIDE.items()]
# The clay digger works on the open clay right of the pit's steps, beside the basin rather than in it, so its rim does
# not cut his legs.
cx_, cy_ = origin(slot_anchor['clay'], sprites['clay', 3].size)
# The blacksmith strikes the anvil on his right: on stage 1 the one on the stump by the hearth, on stage 3 the one in
# the forge mouth; stage 2's art has none, so the stump and anvil cut from stage 1 stand by its door. The sparks fly
# where the hammer lands.
anvil = save(Image.open(f'{A}/props/anvil-stump.png').convert('RGBA'), 'props/anvil.png')
SMITH = {1: (76, 82), 2: (70, 88), 3: (76, 78)}
smith = {}
for v, (px, py) in SMITH.items():
    at_ = local('smithy', v, px, py)
    smith[v] = {'at': at_, 'sparks': [at_[0] + 15, at_[1] - 6]}
smith[2]['anvil'] = {'file': anvil, 'at': local('smithy', 2, 86, 84)}
# The hetman in training stands on the paving right of the academy's door, facing the viewer; each stage's art has
# its own door.
HETMAN = {1: (94, 96), 2: (94, 106), 3: (94, 110)}
hetman = {v: local('academy', v, hx, hy) for v, (hx, hy) in HETMAN.items()}
# While the workshop builds a siege engine, the half-built engine stands in its yard (under the shed on stage 1) and a
# carpenter hammers at it from the west.
CARPENTER = {1: ((38, 100), (60, 96)), 2: ((38, 104), (60, 100)), 3: ((74, 88), (96, 84))}
carpenter = {v: {'at': local('workshop', v, *a), 'engine': local('workshop', v, *e)} for v, (a, e) in CARPENTER.items()}
siege = {u: save(Image.open(f'{A}/props/{u}-build-2.png').convert('RGBA').crop(
    Image.open(f'{A}/props/{u}-build-2.png').convert('RGBA').getbbox()), f'props/{u}-build.png') for u in ('ram', 'catapult')}
workers = {'clay': [cx_ + 104, cy_ + 106], 'hetman': hetman, 'carpenter': carpenter, 'siege': siege, 'smith': smith}

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
    'walls': walls, 'anim': anim, 'villagers': villagers, 'mow': mow, 'drills': drills, 'workers': workers,
}
with open(f'{OUT}/layout.json', 'w') as f:
    json.dump(layout, f, separators=(',', ':'))
shutil.rmtree(tmp)
print('exported', len(files), 'files')
