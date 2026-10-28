"""Natural roads: curved axes, noisy width and edges, soft (smooth-min) joins and an irregular plaza, filled with the
32 px grass and dirt tiles. Also writes roads.json for the walkers."""
import json
import math
import os
import random
import sys
from PIL import Image

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ground as G

A, out = sys.argv[1], sys.argv[2]
W, H = G.W, G.H
S = G.SLOTS
PLAZA = G.P(400, 300)
GATE_S, GATE_W, GATE_E = (400, 428), (188.0, 158.3), (612.0, 158.3)
if os.environ.get('GATES'):
    _g = json.loads(os.environ['GATES'])
    GATE_S, GATE_W, GATE_E = tuple(_g['s']), tuple(_g['w']), tuple(_g['e'])
INSIDE = ['main', 'barracks', 'stable', 'workshop', 'academy', 'smithy', 'market', 'storage', 'hide']


def quad(p0, c, p1, n=48):
    return [((1 - t) ** 2 * p0[0] + 2 * (1 - t) * t * c[0] + t * t * p1[0],
             (1 - t) ** 2 * p0[1] + 2 * (1 - t) * t * c[1] + t * t * p1[1]) for t in (i / n for i in range(n + 1))]


def bend(a, b, k):
    mx, my = (a[0] + b[0]) / 2, (a[1] + b[1]) / 2
    dx, dy = b[0] - a[0], b[1] - a[1]
    return quad(a, (mx - dy * k, my + dx * k), b)


paths = {'gate': ([(GATE_S[0], H + 10), GATE_S] + bend(GATE_S, PLAZA, 0.12)[1:], 24)}
for i, b in enumerate(INSIDE):
    door = (S[b][0], S[b][1] + 3)
    paths[b] = (bend(PLAZA, door, 0.16 if i % 2 else -0.16), 15)
paths['rally'] = (bend(PLAZA, (G.RALLY[0] + 10, G.RALLY[1] + 8), 0.2), 14)
paths['side_w'] = (bend((S['storage'][0], S['storage'][1] + 3), GATE_W, 0.15) + bend(GATE_W, (S['iron'][0] + 6, S['iron'][1] + 30), -0.25)[1:], 14)
paths['side_e'] = (bend((S['academy'][0], S['academy'][1] + 3), GATE_E, -0.15) + bend(GATE_E, (S['farm'][0] - 14, S['farm'][1] + 36), 0.25)[1:], 14)
fork = (GATE_S[0], GATE_S[1] + 34)
paths['wood'] = (bend(fork, (S['wood'][0] - 30, S['wood'][1] + 6), -0.12), 14)
paths['clay'] = (bend(fork, (S['clay'][0] + 40, S['clay'][1] - 4), 0.12), 14)
json.dump({k: [[round(x, 1), round(y, 1)] for x, y in v[0]] for k, v in paths.items()}, open(f'{A}/ground/roads.json', 'w'))
for b in (sys.argv[3].split(',') if len(sys.argv) > 3 and sys.argv[3] else []):
    paths.pop(b, None)

meta = json.load(open(f'{A}/ground/tileset-pro.json'))
sheet = Image.open(f'{A}/ground/tileset-pro.png').convert('RGBA')
pure = {}
for t in meta['tileset_data']['tiles']:
    c = set(t['corners'].values())
    if len(c) == 1:
        bb = t['bounding_box']
        pure[c.pop()] = sheet.crop((bb['x'], bb['y'], bb['x'] + bb['width'], bb['y'] + bb['height'])).load()
grass, dirt = pure['upper'], pure['lower']


def lower_tile(name):
    m = json.load(open(f'{A}/ground/{name}.json'))
    sh = Image.open(f'{A}/ground/{name}.png').convert('RGBA')
    for t in m['tileset_data']['tiles']:
        if set(t['corners'].values()) == {'lower'}:
            bb = t['bounding_box']
            return sh.crop((bb['x'], bb['y'], bb['x'] + bb['width'], bb['y'] + bb['height'])).load()


clay_t = lower_tile('tileset-clay') if os.path.exists(f'{A}/ground/tileset-clay.json') else None
rock_t = lower_tile('tileset-rock') if os.path.exists(f'{A}/ground/tileset-rock.json') else None
bn = G.value_noise(21, 30)
BIOMES = [(n, G.P(*c), G.P(*e)) for n, c, e in (
    ('rock', (40, 50), (190, 125)),
    ('clay', (70, 475), (150, 72)),
    ('forest', (760, 470), (200, 105)),
    ('plain', (730, 80), (175, 105)),
)]


def biome_at(x, y):
    """Strongest biome at a point: a noise-blurred ellipse around a resource building; a value > 0 means inside."""
    best, val = None, 0.0
    for name, (bx, by), (ex, ey) in BIOMES:
        v = 1 - math.sqrt(((x - bx) / ex) ** 2 + ((y - by) / ey) ** 2) + (bn(x, y) - 0.5) * 0.5
        if v > val:
            best, val = name, v
    return best, val


def tex(t, x, y):
    v = ((x // 32) * 83492791 ^ (y // 32) * 2654435761) % 8
    lx, ly = x % 32, y % 32
    if v & 1:
        lx = 31 - lx
    if v & 2:
        ly = 31 - ly
    if v & 4:
        lx, ly = ly, lx
    return t[lx, ly]


def shade(c, k, tint=(0, 0, 0)):
    return (max(0, min(255, int(c[0] * k + tint[0]))), max(0, min(255, int(c[1] * k + tint[1]))), max(0, min(255, int(c[2] * k + tint[2]))), 255)

groups = []
for pts, w in paths.values():
    xs, ys = [q[0] for q in pts], [q[1] for q in pts]
    groups.append(((min(xs) - 30, min(ys) - 30, max(xs) + 30, max(ys) + 30), [(pts[i], pts[i + 1]) for i in range(len(pts) - 1)], w))
wob = G.value_noise(11, 9)
wid = G.value_noise(12, 46)
grain = G.value_noise(13, 3)
sx, sy = S['statue']
K = 4.0


def field(x, y, gs=None, plaza=True):
    """Signed distance to the road edge: hard minimum within one road, soft minimum between roads and the plaza."""
    acc = math.exp(-max(-60.0, (math.sqrt(((x - sx) / (118 * G.SX)) ** 2 + ((y - sy - 8) / (46 * G.SY)) ** 2) - 1) * 40) / K) if plaza else 0.0
    for (x0, y0, x1, y1), sg, w in (groups if gs is None else gs):
        if not (x0 <= x <= x1 and y0 <= y <= y1):
            continue
        d = min(G.seg_dist(x, y, a, b) for a, b in sg) - w / 2 * (0.8 + 0.4 * wid(x, y))
        acc += math.exp(-max(-60.0, d) / K)
    d = -K * math.log(acc) if acc > 1e-30 else 99.0
    return d + (wob(x, y) - 0.5) * 5


def grass_at(x, y):
    v = ((x // 32) * 73856093 ^ (y // 32) * 19349663) % 8
    lx, ly = x % 32, y % 32
    if v & 1:
        lx = 31 - lx
    if v & 2:
        ly = 31 - ly
    if v & 4:
        lx, ly = ly, lx
    g = grass[lx, ly]
    bname, bval = biome_at(x, y)
    if bname and bval > 0:
        edge = bval > 0.12 or grain(x, y) < bval * 6
        if bname == 'rock' and rock_t and edge:
            g = tex(rock_t, x, y)
        elif bname == 'clay' and clay_t and edge:
            g = tex(clay_t, x, y)
        elif bname == 'forest':
            g = shade(g, 1 - min(0.35, bval * 0.9), (0, 2, 4))
        elif bname == 'plain':
            g = shade(g, 1 + min(0.12, bval * 0.3), (min(14, bval * 40), min(8, bval * 20), 0))
    return g


def surface(x, y, d, g):
    """Surface colour at distance d from the road edge; None where grass stays."""
    dt = dirt[x % 32, y % 32] if (x // 32 + 2 * (y // 32)) % 3 else dirt[x % 32, 31 - y % 32]
    if d < -2.5:
        return dt
    if d < 0:
        return None if grain(x, y) > 0.55 + d * 0.12 else dt
    if d < 2:
        return dt if grain(x, y) < 0.18 - d * 0.06 else None
    return None


def paint(img, box, gs, plaza):
    p = img.load()
    x0, y0, x1, y1 = box
    for y in range(max(0, int(y0)), min(H, int(y1) + 1)):
        for x in range(max(0, int(x0)), min(W, int(x1) + 1)):
            c = surface(x, y, field(x, y, gs, plaza), None)
            if c is not None:
                p[x, y] = c


COBBLE = {G.h(c) for c in ('8d8272', 'a39882', 'b8ad96', '6e6355')}
old = Image.open(os.environ.get('BASE', f'{A}/ground/village-ground.png')).convert('RGBA').load()


def cobbles(img):
    p = img.load()
    for y in range(H):
        for x in range(W):
            if old[x, y] in COBBLE:
                p[x, y] = old[x, y]


img = Image.new('RGBA', (W, H))
p = img.load()
for y in range(H):
    for x in range(W):
        p[x, y] = grass_at(x, y)
base = img.copy()
paint(img, (0, 0, W, H), None, True)
cobbles(img)
img.save(out)
print('ok')

EXPORT = os.environ.get('EXPORT_DIR')
if EXPORT:
    """Road-free ground (grass, biomes, plaza) plus every road as its own transparent overlay, so the game lays only roads of existing buildings."""
    os.makedirs(f'{EXPORT}/roads', exist_ok=True)
    paint(base, (0, 0, W, H), [], True)
    cobbles(base)
    base.save(f'{EXPORT}/ground.png')
    meta = {}
    for key, grp in zip(paths.keys(), groups):
        box = grp[0]
        layer = Image.new('RGBA', (W, H))
        paint(layer, box, [grp], False)
        bb = layer.getbbox()
        layer.crop(bb).save(f'{EXPORT}/roads/{key}.png')
        meta[key] = {'x': bb[0], 'y': bb[1], 'points': [[round(x, 1), round(y, 1)] for x, y in paths[key][0]]}
    json.dump(meta, open(f'{EXPORT}/roads.json', 'w'))
    print('exported', list(meta))
