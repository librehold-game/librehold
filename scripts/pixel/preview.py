"""Composes the link-preview images (Open Graph cards) from the game's own pixel art.
Usage: python3 scripts/pixel/preview.py <public/account/og>

`home.png`: a meadow (the backdrop's grass colour strewn with the map's flowers and decorations) with trees, peaks
and villages on the left and the game's name on the right.
`unit-<id>.png` / `building-<id>.png`: one sprite, enlarged, on the same meadow, the name small in a corner.
Everything is drawn on a 600×315 canvas and enlarged ×2 nearest-neighbour; the lettering is drawn after the
enlargement so it stays sharp. Output is deterministic: the same art gives the same files."""
import json
import random
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

OUT = Path(sys.argv[1])
ROOT = Path(__file__).resolve().parents[2]
PIXEL = ROOT / 'src' / 'assets' / 'pixel'
FONT = ROOT / 'src' / 'assets' / 'fonts' / 'cinzel-latin.woff2'
NAME = json.loads((ROOT / 'src' / 'i18n' / 'catalog' / 'en' / 'common.json').read_text())['app']['name']

W, H, SCALE = 600, 315, 2
CARD_SPRITE_MAX_H = 260
GATE_FRAMES = 7
MAX_BYTES = 300_000
GRASS = (132, 194, 50, 255)
NAME_BOX = (315, 115, 590, 200)
CARD_NAME_BOX = (480, 280, 600, 315)
INK = (250, 236, 200, 255)
OUTLINE = (34, 20, 8, 255)

UNITS = ['spear', 'sword', 'axe', 'archer', 'scout', 'lightCav', 'mountedArcher', 'heavyCav', 'ram', 'catapult',
         'paladin', 'noble']
BUILDINGS = ['main', 'barracks', 'stable', 'workshop', 'academy', 'smithy', 'statue', 'market', 'wood', 'clay', 'iron',
             'farm', 'storage', 'hide', 'wall']


def load(rel: str) -> Image.Image:
    return Image.open(PIXEL / rel).convert('RGBA')


def meadow(seed: int, keep_clear: tuple[int, int, int, int] | None = None) -> Image.Image:
    """Grass in the meadow backdrop's own colour, with the map's flowers, decorations and dry patches strewn over it."""
    rng = random.Random(seed)
    canvas = Image.new('RGBA', (W, H), GRASS)
    decals = [load(f'map/decal-dry-{i}.png') for i in range(3)]
    for _ in range(5):
        d = rng.choice(decals)
        canvas.alpha_composite(d, (rng.randint(-20, W - 40), rng.randint(-20, H - 40)))
    small = [load(f'map/flower-{i}.png') for i in range(5)] + [load(f'map/decor-{i}.png') for i in range(12)]
    for _ in range(70):
        s = rng.choice(small)
        x, y = rng.randint(-10, W - 10), rng.randint(-10, H - 10)
        if keep_clear and x + s.width > keep_clear[0] and x < keep_clear[2] and y + s.height > keep_clear[1] and y < keep_clear[3]:
            continue
        canvas.alpha_composite(s, (x, y))
    return canvas


def place(canvas: Image.Image, sprite: Image.Image, cx: int, bottom: int) -> None:
    canvas.alpha_composite(sprite, (cx - sprite.width // 2, bottom - sprite.height))


def enlarge(canvas: Image.Image) -> Image.Image:
    return canvas.resize((W * SCALE, H * SCALE), Image.NEAREST)


def letter(img: Image.Image, text: str, size: int, xy: tuple[int, int], anchor: str, max_width: int | None = None) -> None:
    font = ImageFont.truetype(str(FONT), size)
    while max_width and font.getlength(text) > max_width:
        size -= 2
        font = ImageFont.truetype(str(FONT), size)
    ImageDraw.Draw(img).text(xy, text, font=font, fill=INK, anchor=anchor, stroke_width=max(2, size // 12),
                             stroke_fill=OUTLINE)


def home() -> Image.Image:
    rng = random.Random(21)
    canvas = meadow(21, NAME_BOX)
    sprites: list[tuple[int, int, Image.Image]] = []
    peaks = [load(f'map/peak-{i}.png') for i in range(7)]
    for _ in range(5):
        sprites.append((rng.randint(20, 300), rng.randint(70, 110), rng.choice(peaks)))
    trees = [load('map/tree-oak.png'), load('map/tree-pine.png')]
    for _ in range(26):
        sprites.append((rng.randint(0, 330), rng.randint(100, H + 10), rng.choice(trees)))
    for _ in range(6):
        sprites.append((rng.randint(320, W), rng.choice([rng.randint(30, 80), rng.randint(270, H + 10)]), rng.choice(trees)))
    villages = [(150, 230, 5), (60, 170, 2), (250, 175, 3), (70, 285, 1), (255, 290, 4), (180, 140, 0)]
    for cx, bottom, tier in villages:
        sprites.append((cx, bottom, load(f'map/village-{tier}.png')))
    for cx, bottom, sprite in sorted(sprites, key=lambda s: s[1]):
        place(canvas, sprite, cx, bottom)
    big = enlarge(canvas)
    letter(big, NAME, 150, (900, 630 // 2), 'mm', max_width=(NAME_BOX[2] - NAME_BOX[0]) * SCALE)
    return big


def unit_sprite(unit: str) -> Image.Image:
    meta = json.loads((PIXEL / 'army' / 'army.json').read_text())['units'][unit]
    return load(f'army/{unit}-south.png').crop((0, 0, meta['w'], meta['h']))


def building_sprite(building: str) -> Image.Image:
    if building == 'wall':
        gate = load('village/walls/stone-gate.png')
        return gate.crop((0, 0, gate.width // GATE_FRAMES, gate.height))
    return load(f'village/buildings/{building}-3.png')


def card(sprite: Image.Image) -> Image.Image:
    sprite = sprite.crop(sprite.getbbox())
    factor = max(1, min(CARD_SPRITE_MAX_H // sprite.height, (W - 40) // sprite.width))
    sprite = sprite.resize((sprite.width * factor, sprite.height * factor), Image.NEAREST)
    canvas = meadow(7, CARD_NAME_BOX)
    canvas.alpha_composite(sprite, ((W - sprite.width) // 2, (H - sprite.height) // 2))
    big = enlarge(canvas)
    letter(big, NAME, 44, (1200 - 28, 630 - 22), 'rs', max_width=(CARD_NAME_BOX[2] - CARD_NAME_BOX[0]) * SCALE)
    return big


def save(img: Image.Image, name: str) -> None:
    path = OUT / name
    img.convert('RGB').save(path, optimize=True)
    if path.stat().st_size > MAX_BYTES:
        img.convert('RGB').quantize(256, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE).save(path, optimize=True)


OUT.mkdir(parents=True, exist_ok=True)
save(home(), 'home.png')
for u in UNITS:
    save(card(unit_sprite(u)), f'unit-{u}.png')
for b in BUILDINGS:
    save(card(building_sprite(b)), f'building-{b}.png')
