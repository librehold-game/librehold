"""Shared by the preview (`compose.py`) and the export (`export.py`): scenery placement and the wall ring split."""
import json
import math
import random
from PIL import Image

import ground as G

SCENERY = ('scenery', 'biome')


def load(path):
    im = Image.open(path).convert('RGBA')
    return im.crop(im.getbbox())


def hot(c):
    """Fire or ember pixel — a light source at night."""
    r, g, b = c[:3]
    return c[3] == 255 and r > 200 and g > 90 and b < 110 and r - b > 120




def _grass(c):
    r, g, b = c[:3]
    return g >= r - 5 and g > b + 25


def _plate_grass(c):
    """Yellow-green plate top only, so blue-green foliage on the same sprite survives."""
    r, g, b = c[:3]
    return r >= 70 and b < 60 and g > b + 40


def _skirt(c):
    r, g, b = c[:3]
    return r + g + b < 300 or (_grass(c) and g < 140)


def unplate(im, grass=_grass, skirt_max=9):
    """Strip the diorama base plate a building sprite stands on — the dark earth skirt at the bottom of every
    column and every pixel `grass` accepts — so it stands on the scene ground.
    The canvas keeps its size so the anchor holds."""
    im = im.copy()
    p = im.load()
    W, H = im.size
    clear = (0, 0, 0, 0)
    for x in range(W):
        ys = [y for y in range(H) if p[x, y][3] > 0]
        for y in reversed(ys[-skirt_max:] if ys else []):
            if not _skirt(p[x, y]):
                break
            p[x, y] = clear
    for y in range(H):
        for x in range(W):
            if p[x, y][3] > 0 and grass(p[x, y]):
                p[x, y] = clear
    return im


WALL_OUTSIDE, WALL_INSIDE = 1.06, 0.92


def clear_of_wall(x, y, w, ring):
    """Slide a footprint `w` px wide along its radius until it stays clear of the wall ellipse on its own side."""
    cx, cy, rx, ry = ring

    def foot():
        return [math.hypot((x + t * w - cx) / rx, (y - cy) / ry) for t in (-0.5, -0.25, 0, 0.25, 0.5)]

    outside = math.hypot((x - cx) / rx, (y - cy) / ry) > 1
    ux, uy = x - cx, y - cy
    n = math.hypot(ux, uy) or 1
    step = 1 if outside else -1
    while (min(foot()) < WALL_OUTSIDE) if outside else (max(foot()) > WALL_INSIDE):
        x, y = x + step * ux / n, y + step * uy / n
    return x, y


def scenery(A, ring):
    """Non-building scenery as (source, x, y, z, flip); source is relative to `A`, the anchor is the bottom-centre."""
    P, W, H = G.P, G.W, G.H
    cx, cy, rx, ry = ring
    out = []

    widths = {}

    def put(src, x, y, z=None, flip=False):
        if src not in widths:
            widths[src] = load(f'{A}/{src}').width
        x, y = clear_of_wall(x, y, widths[src], ring)
        out.append((src, x, y, y if z is None else z, flip))

    put('scenery/well.png', *P(372, 356))
    put('scenery/woodpile.png', *P(640, 474))
    put('scenery/cart.png', *P(338, 298))
    for i in range(3):
        put('scenery/fence.png', *P(652 + i * 30, 170 + i * 4))

    keep = [P(x, y) + (r,) for x, y, r in ((96, 110, 84), (716, 100, 96), (700, 430, 80), (96, 450, 76), (400, 470, 50), (640, 470, 34),
                                          (150, 140, 34), (125, 145, 30), (650, 140, 34), (680, 152, 30))]
    biomes = [(n, P(*c), P(*e)) for n, c, e in (('rock', (40, 50), (190, 125)), ('clay', (70, 475), (150, 72)),
                                                ('forest', (760, 470), (200, 105)), ('plain', (730, 80), (175, 105)))]

    def biome(x, y):
        for name, (bx, by), (ex, ey) in biomes:
            if ((x - bx) / ex) ** 2 + ((y - by) / ey) ** 2 < 1:
                return name
        return None

    def in_ring(x, y, m):
        return ((x - cx) / (rx + m)) ** 2 + ((y - cy) / (ry + m)) ** 2 < 1

    for src, x, y, flip in (('cliff-a', 40, 30, False), ('cliff-b', 150, 34, False), ('cliff-a', 230, 20, True), ('cliff-b', -8, 92, True),
                            ('boulder-big', 196, 70, False), ('scree', 172, 92, False), ('scree', 14, 140, False), ('boulders-a', 26, 186, False),
                            ('boulders-b', 222, 44, False), ('boulders-b', 8, 222, False)):
        put(f'biome/{src}.png', *P(x, y), flip=flip)
    fields = [(s,) + P(x, y) for s, x, y in (('field-wheat', 610, 64), ('field-plowed', 700, 36), ('field-wheat', 772, 214), ('field-plowed', 768, 262))]
    for s, x, y in fields:
        put(f'biome/{s}.png', x, y, z=0)
    keep += [(x, y - 16, 44) for _, x, y in fields]
    for x, y in (P(786, 412), P(742, 506), P(800, 486)):
        put('biome/forest-b.png', x, y)
        keep.append((x, y - 20, 40))
    put('biome/stumps.png', *P(652, 418))
    for x, y in (P(24, 190), P(176, 26), P(140, 196)):
        put('scenery/rocks.png', x, y)

    rng = random.Random(11)
    tree_p = {'forest': 1.0, 'plain': 0.0, 'rock': 0.0, 'clay': 0.05, None: 0.22}
    keep += [P(110, 80) + (130,), P(90, 470) + (110,)]
    trees = []
    for _ in range(9000):
        if len(trees) >= round(58 * G.SX * G.SY):
            break
        x, y = 6 + rng.random() * (W - 12), 40 + rng.random() * (H - 30)
        bio = biome(x, y)
        if rng.random() > tree_p[bio]:
            continue
        if in_ring(x, y, 34) or any((x - a) ** 2 + (y - b) ** 2 < r * r for a, b, r in keep):
            continue
        gap = 13 if bio == 'forest' else 24
        if any((x - a) ** 2 + (y - b) ** 2 < gap ** 2 for a, b in trees) or (abs(x - cx) < 70 and y > cy + ry):
            continue
        trees.append((x, y))
        put('scenery/pine.png' if bio in ('rock', 'forest') and rng.random() < 0.7 else 'scenery/oak.png' if rng.random() < 0.5 else 'scenery/pine.png', x, y)
    for _ in range(18):
        for _g in range(300):
            x, y = 10 + rng.random() * (W - 20), 30 + rng.random() * (H - 20)
            if not in_ring(x, y, 12) and not any((x - a) ** 2 + (y - b) ** 2 < r * r for a, b, r in keep):
                put('scenery/bush.png' if rng.random() < 0.7 else 'scenery/rocks.png', x, y)
                break
    return out


def ring_split(A, name, ring):
    """The wall as one ring image cut into 8 px columns, each split into a back and a front half whose depth is the ellipse
    ground line in that column, so buildings inside draw between them. Ellipse and gates come from `<name>.json`."""
    cx, cy, rx, ry = ring
    meta = json.load(open(f'{A}/scenery/{name}.json'))
    im = Image.open(f'{A}/scenery/{name}.png').convert('RGBA')
    gl, gt, gr, gb = meta['ground']
    ox, oy = meta.get('offset') or (round(cx - (gl + gr) / 2), round(cy + ry - gb))
    cy, rx, ry = oy + (gt + gb) / 2, (gr - gl) / 2, (gb - gt) / 2
    back, front = Image.new('RGBA', im.size), Image.new('RGBA', im.size)
    p, bp, fp = im.load(), back.load(), front.load()
    strips = []
    for x0 in range(0, im.width, 8):
        xm = ox + x0 + 4
        k = max(0.0, 1 - ((xm - cx) / rx) ** 2) ** 0.5
        yn, ys = cy - ry * k, cy + ry * k
        used = False
        for x in range(x0, min(x0 + 8, im.width)):
            runs, prev = [], False
            for y in range(im.height):
                o = p[x, y][3] > 0
                if o and not prev:
                    start = y
                if prev and not o:
                    runs.append((start, y))
                prev = o
            if prev:
                runs.append((start, im.height))
            if not runs:
                continue
            used = True
            split = runs[0][1] if len(runs) > 1 else round((gt + gb) / 2)
            for y in range(im.height):
                if p[x, y][3] > 0:
                    (bp if y < split else fp)[x, y] = p[x, y]
        if used:
            strips.append({'x0': ox + x0, 'zBack': round(yn - 1, 1), 'zFront': round(ys + 1, 1)})
    gates = {k: (v[0] + ox, v[1] + oy) for k, v in meta['gates'].items()}
    return {'back': back, 'front': front, 'x': ox, 'y': oy, 'strips': strips, 'gates': gates, 'ring': (cx, cy, rx, ry)}


UNPLATED = {'buildings/clay': _grass, 'buildings/wood': _grass, 'biome/stumps': _grass, 'scenery/woodpile': _grass,
            'scenery/rocks': _grass, 'biome/forest-b': _plate_grass}


def unplated(rel, im):
    """`im` without its base plate when the source (path without extension or stage suffix) has one."""
    key = rel.rsplit('.', 1)[0]
    key = key[:-2] if key[-2:] in ('-1', '-3') else key
    return unplate(im, UNPLATED[key]) if key in UNPLATED else im
