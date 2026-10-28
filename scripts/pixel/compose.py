"""Out-of-game village preview: a day GIF and a night frame on the same layout `export.py` exports.
Usage: SCENE_SCALE=1.1,1.2 python3 compose.py <art> <out-without-extension> early|mature <ground.png>"""
import glob
import json
import math
import os
import sys
from PIL import Image

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ground as G
import parts

A, OUT, STAGE, GROUND = sys.argv[1:5]
W, H = G.W, G.H
SLOTS, RALLY, P = G.SLOTS, G.RALLY, G.P
ST = A + '/villager-states'
DIRS = ('south', 'east', 'north', 'west')
load = parts.load

if STAGE == 'early':
    LEVEL = {b: 1 for b in SLOTS}
    LEVEL.update(statue=0, academy=0, workshop=0, stable=0, market=0, smithy=0)
    WALL, SITE = 1, 'barracks'
else:
    LEVEL = {b: 3 for b in SLOTS}
    LEVEL.update(wood=2, iron=2, farm=2, hide=2, storage=2)
    WALL, SITE = 3, 'workshop'


def frames(d, skip_ref=False):
    fs = [load(f) for f in sorted(glob.glob(d + '/*.png'))]
    return fs[1:] if skip_ref else fs


def resample(pts, step):
    out = [pts[0]]
    carry = 0.0
    for a, b in zip(pts, pts[1:]):
        L = math.dist(a, b)
        pos = step - carry
        while pos <= L:
            t = pos / L
            out.append((a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t))
            pos += step
        carry = L - (pos - step)
    return out


def facing(a, b):
    dx, dy = b[0] - a[0], b[1] - a[1]
    if abs(dx) >= abs(dy) * 1.2:
        return 'east' if dx > 0 else 'west'
    return 'south' if dy > 0 else 'north'


def bsprite(b):
    lvl = LEVEL[b]
    rel = f'buildings/{b}-{lvl}.png' if lvl != 2 and os.path.exists(f'{A}/buildings/{b}-{lvl}.png') else f'buildings/{b}.png'
    return parts.unplated(rel, load(f'{A}/{rel}'))


static = []


def put(sprite, x, y, z=None):
    static.append((y if z is None else z, sprite, int(x - sprite.width / 2), int(y - sprite.height)))


ring = parts.ring_split(A, 'wall-ring-big' if WALL >= 3 else 'wall-ring-palisade', G.RING)
cx, cy, rx, ry = ring['ring']
for s in ring['strips']:
    x0 = s['x0'] - ring['x']
    for img, z in ((ring['back'], s['zBack']), (ring['front'], s['zFront'])):
        static.append((z, img.crop((x0, 0, x0 + 8, img.height)), s['x0'], ring['y']))
gname = 'palisade-gate' if WALL < 3 else 'stone-gate'
gfs = [load(f) for f in sorted(glob.glob(f'{A}/anim/{gname}/frame_*.png'), key=lambda s: int(s.rsplit('_', 1)[1][:-4]))]
gate = {'pos': (ring['gates']['s'][0], ring['gates']['s'][1] + 4), 'frames': gfs, 'open': 0, 'z': cy + ry + 3}

LIGHTS = set()
ANIMATED = {b for b in ('wood', 'iron', 'farm') if LEVEL[b] == 2}
for b, (x, y) in SLOTS.items():
    if b == SITE:
        put(load(f'{A}/scenery/construction.png'), x, y + 12)
    elif LEVEL[b] == 0:
        continue
    elif b not in ANIMATED:
        s = bsprite(b)
        if b in ('iron', 'smithy'):
            LIGHTS.add(id(s))
        put(s, x, y + 12)
place = load(f'{A}/buildings/place.png')
LIGHTS.add(id(place))
put(place, RALLY[0] + 4, RALLY[1] + 22)
cache = {}
for src, x, y, z, flip in parts.scenery(A, (cx, cy, rx, ry)):
    if (src, flip) not in cache:
        im = parts.unplated(src, load(f'{A}/{src}'))
        cache[src, flip] = im.transpose(Image.FLIP_LEFT_RIGHT) if flip else im
    put(cache[src, flip], x, y, z)

walk = {d: frames(f'{ST}/Idle/animations/walk/{d}') for d in DIRS}
logw = {d: frames(f'{ST}/Carry_log/animations/walk/{d}', True) for d in DIRS}
sackw = {d: frames(f'{ST}/Carry_sack/animations/walk/{d}', True) for d in DIRS}
spearw = {d: frames(f'{A}/units/spear/Idle/animations/walk/{d}') for d in DIRS}
chop = frames(f'{ST}/Idle/animations/chop-axe/east')
hammer = frames(f'{ST}/Builder/animations/hammer/east')
FRONT = [0, 1, 2, 2, 2, 1, 0, 0]
wheel = [Image.open(f'{A}/anim/wood-wheel/frame_{i}.png').convert('RGBA').crop((10, 8, 36, 44)) for i in range(1, 9)]
fx = {b: [Image.open(f'{A}/anim/{b}/frame_{i}.png').convert('RGBA') for i in range(12)] for b in ('iron', 'farm')}

ROADS = {k: [tuple(q) for q in v] for k, v in json.load(open(f'{A}/ground/roads.json')).items()}


def loop(pts):
    return pts + pts[::-1]


main_road = ROADS['gate'][1:]
walkers = [(walk, resample(loop(ROADS[b]), 1.5), i * 41) for i, b in enumerate(['main', 'barracks', 'market', 'stable', 'academy', 'smithy']) if LEVEL[b] > 0 or b == SITE]
walkers.append((walk, resample(loop(main_road[::-1] + [(cx, cy + ry + 60)]), 1.5), 20))
walkers.append((logw, resample(loop(ROADS['wood'][::-1] + [(cx + 6, cy + ry + 26)] + main_road + ROADS[SITE][1:]), 1.5), 0))
walkers.append((sackw, resample(loop(ROADS['storage'][::-1] + ROADS[SITE][1:]), 1.5), 60))
walkers.append((walk, resample(loop(ROADS['storage'] + ROADS['side_w'][1:]), 1.5), 10))
walkers.append((sackw, resample(loop(ROADS['academy'] + ROADS['side_e'][1:]), 1.5), 70))
for start, step, n, off in ((math.pi + 0.3, 0.012, 130, 0), (-0.2, -0.012, 110, 50)):
    arc = [(cx + (rx - 30) * math.cos(start + i * step), cy + (ry - 24) * math.sin(start + i * step)) for i in range(n)]
    walkers.append((spearw, resample(loop(arc), 1.2), off))

N = 120
out = []
ground = Image.open(GROUND).convert('RGBA')
first = None
for t in range(-N, N):
    dyn = []
    pos = [path[(t + off) % len(path)] for _, path, off in walkers]
    near = any(math.dist(p, gate['pos']) < 44 for p in pos)
    gate['open'] = min(len(gfs) - 1, gate['open'] + 1) if near else max(0, gate['open'] - 1)
    g = gfs[gate['open']]
    dyn.append((gate['z'], g, int(gate['pos'][0] - g.width / 2), int(gate['pos'][1] - g.height)))
    if t < 0:
        continue
    for b in ANIMATED:
        x, y = SLOTS[b]
        if b == 'wood':
            m = Image.open(f'{A}/buildings/wood.png').convert('RGBA')
            m.paste(wheel[t % 8], (90, 44))
        else:
            m = fx[b][t % 12]
        m = m.crop(m.getbbox())
        if b == 'iron':
            LIGHTS.add(id(m))
        dyn.append((y + 12, m, int(x - m.width / 2), int(y + 12 - m.height)))
    for by_dir, path, off in walkers:
        i = (t + off) % len(path)
        a, b = path[i], path[(i + 1) % len(path)]
        seq = by_dir[facing(a, b)]
        fr = seq[(t // 2) % len(seq)]
        dyn.append((a[1], fr, int(a[0] - fr.width / 2), int(a[1] - fr.height)))
    c = chop[FRONT[(t // 2) % 8]]
    chx, chy = P(606, 476)
    dyn.append((chy, c, int(chx), int(chy - c.height)))
    sx, sy = SLOTS[SITE]
    hm = hammer[FRONT[(t // 2 + 3) % 8]]
    dyn.append((sy + 22, hm, int(sx - 60), int(sy + 22 - hm.height)))
    frame = ground.copy()
    layers = sorted(static + dyn, key=lambda q: q[0])
    for z, s, x, y in layers:
        frame.alpha_composite(s, (int(x), int(y)))
    first = first or layers
    out.append(frame)

out[0].save(OUT + '.png')
gif = [f.convert('RGB').convert('P', palette=Image.ADAPTIVE, colors=255) for f in out]
gif[0].save(OUT + '.gif', save_all=True, append_images=gif[1:], duration=90, loop=0, disposal=1)


hot = parts.hot
mask = Image.new('L', (W, H), 0)
for z, s, x, y in first:
    mask.paste(0, (int(x), int(y)), s.getchannel('A'))
    if id(s) in LIGHTS:
        m = Image.new('L', s.size, 0)
        mp, sp = m.load(), s.load()
        for yy in range(s.height):
            for xx in range(s.width):
                if hot(sp[xx, yy]):
                    mp[xx, yy] = 255
        mask.paste(255, (int(x), int(y)), m)
night = out[0].copy()
px, mk = night.load(), mask.load()
for y in range(H):
    for x in range(W):
        r, g, b, a = px[x, y]
        if not mk[x, y]:
            px[x, y] = (int(r * 0.32 + 6), int(g * 0.36 + 10), int(b * 0.55 + 30), 255)
night.save(OUT + '-night.png')
print('ok', len(out))
