#!/usr/bin/env python3
"""Dark, minimalist line icon for aulycmail — full rectangle + V flap.

Matches a hand-drawn reference: a complete rounded rectangle (with top edge)
plus an envelope flap drawn as a V from the two top corners down to a center
point. Continuous line, medium-thin stroke, near-black squircle background.
"""
from PIL import Image, ImageDraw

S = 1024
SS = 4
W = S * SS
R = int(0.2237 * S) * SS

LINE = (212, 212, 218)
BG_TOP = (40, 40, 44)
BG_BOT = (18, 18, 22)
SW = int(W * 0.017)            # stroke width


def squircle(size, radius):
    m = Image.new("L", (size, size), 0)
    ImageDraw.Draw(m).rounded_rectangle(
        [0, 0, size - 1, size - 1], radius=radius, fill=255)
    return m


def vgrad(size, top, bot):
    g = Image.new("RGB", (size, size))
    px = g.load()
    for y in range(size):
        t = y / (size - 1)
        c = tuple(int(top[i] + (bot[i] - top[i]) * t) for i in range(3))
        for x in range(size):
            px[x, y] = c
    return g


# --- background: near-black squircle ---
icon = Image.new("RGBA", (W, W), (0, 0, 0, 0))
bg = vgrad(W, BG_TOP, BG_BOT).convert("RGBA")
icon.paste(bg, (0, 0), squircle(W, R))

d = ImageDraw.Draw(icon)

# --- envelope geometry: full rectangle + V flap ---
ew, eh = int(W * 0.54), int(W * 0.44)
left = (W - ew) // 2
top = (W - eh) // 2
right = left + ew
bottom = top + eh
cx = W // 2
er = int(eh * 0.06)            # small corner radius (crisp like the sketch)
flap_y = top + int(eh * 0.46)  # depth of the V flap

# full rectangle outline (includes the top edge)
d.rounded_rectangle([left, top, right, bottom], radius=er, outline=LINE, width=SW)

# flap V from the two top corners down to a center point
TL = (left + er, top)
TR = (right - er, top)
M = (cx, flap_y)
d.line([TL, M, TR], fill=LINE, width=SW, joint="curve")

# round the join vertices/caps
rr = SW // 2
for (px, py) in [TL, TR, M]:
    d.ellipse([px - rr, py - rr, px + rr, py + rr], fill=LINE)

# downsample for smooth anti-aliasing
icon = icon.resize((S, S), Image.LANCZOS)
icon.save("build/appicon.png")
print("wrote build/appicon.png", icon.size)

icon.resize((512, 512), Image.LANCZOS).save(
    "frontend/src/assets/images/logo-universal.png")
icon.resize((512, 512), Image.LANCZOS).save("brand/icon.png")
icon.resize((512, 512), Image.LANCZOS).save("brand/icon-beautyline.png")
print("wrote frontend logo + brand icons")
