"""Full wall ring from a drawn left half: the half plus its mirror image, joined at the cut axis.
The axis is the last column the wall reaches; prints column runs and dark clusters (gate arches) to locate the ground ellipse."""
import collections
import sys
from PIL import Image

src, out = sys.argv[1], sys.argv[2]
L = Image.open(src).convert('RGBA')
p = L.load()
W, H = L.size
cut = max(x for x in range(W) if any(p[x, y][3] > 0 for y in range(H))) + 1
left = L.crop((0, 0, cut, H))
full = Image.new('RGBA', (cut * 2, H))
full.paste(left, (0, 0))
full.paste(left.transpose(Image.FLIP_LEFT_RIGHT), (cut, 0))
full.save(out)
q = full.load()
FW = cut * 2


def runs(x):
    r, prev = [], False
    for y in range(H):
        o = q[x, y][3] > 0
        if o and not prev:
            s = y
        if prev and not o:
            r.append((s, y - 1))
        prev = o
    if prev:
        r.append((s, H - 1))
    return r


print('size', full.size, 'cut', cut)
for x in range(0, FW, FW // 16):
    print(x, runs(x))
print('centre', runs(cut - 1), runs(cut))
dark = [(x, y) for y in range(H) for x in range(cut) if q[x, y][3] > 128 and sum(q[x, y][:3]) < 60]
cl = collections.Counter((x // 8 * 8, y // 8 * 8) for x, y in dark)
print('dark', sorted(cl.items(), key=lambda kv: -kv[1])[:10])
