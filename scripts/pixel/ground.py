"""Village ground in the building palette: patchy grass with tufts, curved roads and the cobbled plaza (800x500 layout, scaled by SCENE_SCALE)."""
import math
import os
import random
import sys
from PIL import Image

out = sys.argv[1] if len(sys.argv) > 1 else None
SX, SY = (float(v) for v in os.environ.get('SCENE_SCALE', '1,1').split(','))
W, H = round(800 * SX), round(500 * SY)


def P(x, y):
    """800x500 layout coordinates scaled to the current scene (sprites stay 1:1)."""
    return (x * SX, y * SY)


RING = P(400, 262) + P(300, 166)
SLOTS = {k: P(*v) for k, v in {
    'main': (400, 168), 'academy': (566, 196), 'storage': (236, 192), 'hide': (168, 258), 'market': (298, 262),
    'statue': (400, 290), 'smithy': (278, 346), 'barracks': (580, 318), 'stable': (222, 320), 'workshop': (506, 356),
    'wood': (700, 444), 'clay': (96, 458), 'iron': (96, 118), 'farm': (722, 124),
}.items()}
RALLY = P(478, 322)
INSIDE = ['main', 'barracks', 'stable', 'workshop', 'academy', 'smithy', 'market', 'storage', 'hide']
GATE_W, GATE_E = P(188.0, 158.3), P(612.0, 158.3)
SIDE_ROADS = [
    [(SLOTS['storage'][0], SLOTS['storage'][1] + 3), P(212, 178), GATE_W, P(152, 138), (SLOTS['iron'][0] + 6, SLOTS['iron'][1] + 30)],
    [(SLOTS['academy'][0], SLOTS['academy'][1] + 3), P(590, 178), GATE_E, P(650, 138), (SLOTS['farm'][0] - 14, SLOTS['farm'][1] + 36)],
]


def h(s):
    return tuple(int(s[i:i + 2], 16) for i in (0, 2, 4)) + (255,)


GRASS = [h('557f2c'), h('5f8a30'), h('689435'), h('719c3a'), h('7ba340')]
TUFT_D, TUFT_L = h('46702a'), h('92b252')
DIRT = [h('b98a5c'), h('c99b6a'), h('d7b080'), h('dead7c'), h('e7cb9d')]
PEBBLE_D, PEBBLE_L = h('975f3a'), h('f4dcb0')
COBBLE = [h('8d8272'), h('a39882'), h('b8ad96')]
MORTAR = h('6e6355')
BAYER = [[0, 8, 2, 10], [12, 4, 14, 6], [3, 11, 1, 9], [15, 7, 13, 5]]


def value_noise(seed, scale):
    rng = random.Random(seed)
    gw, gh = W // scale + 2, H // scale + 2
    g = [[rng.random() for _ in range(gw)] for _ in range(gh)]

    def f(x, y):
        fx, fy = x / scale, y / scale
        ix, iy = int(fx), int(fy)
        tx, ty = fx - ix, fy - iy
        tx, ty = tx * tx * (3 - 2 * tx), ty * ty * (3 - 2 * ty)
        a = g[iy][ix] * (1 - tx) + g[iy][ix + 1] * tx
        b = g[iy + 1][ix] * (1 - tx) + g[iy + 1][ix + 1] * tx
        return a * (1 - ty) + b * ty
    return f


def quad(p0, c, p1, n=60):
    return [((1 - t) ** 2 * p0[0] + 2 * (1 - t) * t * c[0] + t * t * p1[0],
             (1 - t) ** 2 * p0[1] + 2 * (1 - t) * t * c[1] + t * t * p1[1]) for t in (i / n for i in range(n + 1))]


def roads():
    cx, cy, rx, ry = RING
    plaza = (SLOTS['statue'][0], SLOTS['statue'][1] + 10)
    gate = (cx, cy + ry)
    r = [([(cx, H + 10), gate] + quad(gate, (gate[0], gate[1] - 60), plaza)[1:], 22)]
    for b in INSIDE:
        x, y = SLOTS[b]
        r.append((quad(plaza, ((plaza[0] + x) / 2, (plaza[1] + y) / 2 + 12), (x, y + 3)), 14))
    r.append(([plaza, (RALLY[0] + 10, RALLY[1] + 8)], 14))
    for pts in SIDE_ROADS:
        r.append((pts, 13))
    r.append((quad((gate[0] + 34, gate[1] + 30), (SLOTS['wood'][0] - 80, SLOTS['wood'][1] + 20), (SLOTS['wood'][0] - 30, SLOTS['wood'][1] + 6)), 13))
    r.append((quad((gate[0] - 30, gate[1] + 30), P(220, 470), (SLOTS['clay'][0] + 40, SLOTS['clay'][1] - 4)), 13))
    return r


def seg_dist(px, py, a, b):
    vx, vy = b[0] - a[0], b[1] - a[1]
    L = vx * vx + vy * vy or 1e-9
    t = max(0, min(1, ((px - a[0]) * vx + (py - a[1]) * vy) / L))
    dx, dy = px - (a[0] + t * vx), py - (a[1] + t * vy)
    return math.hypot(dx, dy)


def main():
    img = Image.new('RGBA', (W, H))
    p = img.load()
    n1, n2, n3 = value_noise(1, 48), value_noise(2, 9), value_noise(3, 20)
    cx, cy, rx, ry = RING
    for y in range(H):
        for x in range(W):
            v = 0.5 + (n1(x, y) - 0.5) * 0.5 + (n2(x, y) - 0.5) * 0.35
            inside = ((x - cx) / (rx - 10)) ** 2 + ((y - cy) / (ry - 10)) ** 2 < 1
            if inside:
                v += 0.06
            v += (BAYER[y % 4][x % 4] / 16 - 0.5) * 0.18
            p[x, y] = GRASS[max(0, min(4, int(v * 5)))]
    rng = random.Random(5)
    for _ in range(1300):
        x, y = rng.randrange(1, W - 1), rng.randrange(2, H)
        p[x, y] = TUFT_D
        p[x - 1, y - 1] = TUFT_D
        p[x + 1, y - 1] = TUFT_D
        p[x, y - 2] = TUFT_L
    for _ in range(260):
        x, y = rng.randrange(W), rng.randrange(H)
        p[x, y] = rng.choice([h('f2f0e0'), h('f2d64a'), h('d9665a'), h('b9a7e0')])

    rs = roads()
    segs = [(pts[i], pts[i + 1], w) for pts, w in rs for i in range(len(pts) - 1)]
    sx, sy = SLOTS['statue']
    for y in range(H):
        for x in range(W):
            best = 99.0
            for a, b, w in segs:
                if min(a[0], b[0]) - w > x or max(a[0], b[0]) + w < x or min(a[1], b[1]) - w > y or max(a[1], b[1]) + w < y:
                    continue
                best = min(best, seg_dist(x, y, a, b) - w / 2)
            ex = ((x - sx) / (128 * SX)) ** 2 + ((y - (sy + 8)) / (56 * SY)) ** 2
            best = min(best, (math.sqrt(ex) - 1) * 50)
            th = (BAYER[y % 4][x % 4] / 16) * 2.2 - 1.1
            if best < th:
                v = n3(x, y) * 0.6 + n2(x, y) * 0.4 + (BAYER[(y + 1) % 4][(x + 2) % 4] / 16 - 0.5) * 0.2
                c = DIRT[max(0, min(4, int(v * 5)))]
                if best > th - 1.2:
                    c = DIRT[0] if y % 2 == 0 else DIRT[1]
                p[x, y] = c
    for _ in range(1400):
        x, y = rng.randrange(W - 1), rng.randrange(H - 1)
        if p[x, y] in DIRT[1:]:
            p[x, y] = PEBBLE_D
            if p[x, y - 1] in DIRT:
                p[x, y - 1] = PEBBLE_L
    crng = random.Random(9)
    shade = {}
    for y in range(H):
        for x in range(W):
            e = ((x - sx) / (68 * SX)) ** 2 + ((y - (sy + 8)) / (28 * SY)) ** 2
            if e < 1 + (BAYER[y % 4][x % 4] / 16 - 0.5) * 0.12:
                row = y // 4
                off = (row * 3) % 5
                bx = (x + off) // 5
                lx, ly = (x + off) % 5, y % 4
                key = (bx, row)
                if key not in shade:
                    shade[key] = crng.randrange(3)
                c = COBBLE[shade[key]]
                p[x, y] = MORTAR if lx == 0 or ly == 3 else (COBBLE[2] if ly == 0 and c != COBBLE[2] else c)
    for y in range(int(RING[1] + RING[3]) + 2, H):
        for x in (int(cx) - 6, int(cx) + 5):
            if (y // 3) % 3:
                p[x, y] = DIRT[0]
    img.save(out)


if __name__ == '__main__':
    main()
    print('ok')
