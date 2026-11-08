#!/usr/bin/env python3
"""Generates the docs diagrams (SVG) into docs/src/assets/diagrams/.

Run from anywhere:  python3 docs/diagrams/generate.py
Each diagram is plain SVG in the same style as architecture.svg: white
background, rounded boxes, slate/indigo/green/amber palette, system fonts.
Text widths are estimated (about 0.56 em per character) and every box is
checked against its longest line, so the script fails instead of emitting
overflowing text.
"""
from pathlib import Path
from xml.sax.saxutils import escape

OUT = Path(__file__).resolve().parent.parent / "src" / "assets" / "diagrams"

FONT = "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
MONO = "ui-monospace, 'SF Mono', Menlo, Consolas, monospace"

PAL = {
    "indigo": ("#eef2ff", "#6366f1", "#312e81", "#4338ca"),
    "slate": ("#f8fafc", "#94a3b8", "#0f172a", "#475569"),
    "white": ("#ffffff", "#cbd5e1", "#0f172a", "#475569"),
    "card": ("#ffffff", "#a5b4fc", "#1e1b4b", "#334155"),
    "green": ("#f0fdf4", "#22c55e", "#14532d", "#166534"),
    "amber": ("#fffbeb", "#f59e0b", "#78350f", "#92400e"),
    "rose": ("#fff1f2", "#fb7185", "#881337", "#9f1239"),
}


def text_width(s, size, mono=False):
    return len(s) * size * (0.62 if mono else 0.56)


class Svg:
    def __init__(self, w, h, title):
        self.w, self.h = w, h
        self.parts = [
            f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" '
            f'font-family="{FONT}" font-size="13">',
            f"<title>{escape(title)}</title>",
            "<defs>"
            '<marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" fill="#475569"/></marker>'
            '<marker id="b" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" fill="#4f46e5"/></marker>'
            "</defs>",
            f'<rect width="{w}" height="{h}" fill="#ffffff"/>',
        ]

    def add(self, s):
        self.parts.append(s)

    def text(self, x, y, s, size=13, color="#334155", weight=None, anchor=None, mono=False):
        attrs = [f'x="{x}"', f'y="{y}"', f'font-size="{size}"', f'fill="{color}"']
        if weight:
            attrs.append(f'font-weight="{weight}"')
        if anchor:
            attrs.append(f'text-anchor="{anchor}"')
        if mono:
            attrs.append(f'font-family="{MONO}"')
        self.add(f"<text {' '.join(attrs)}>{escape(s)}</text>")

    def box(self, x, y, w, h, title, lines=(), style="white", title_size=13.5, mono_lines=False,
            radius=10, stroke_width=1.2, pad=14):
        fill, stroke, tcol, lcol = PAL[style]
        self.add(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{radius}" fill="{fill}" '
                 f'stroke="{stroke}" stroke-width="{stroke_width}"/>')
        widest = max([text_width(title, title_size)] +
                     [text_width(l, 12.5, mono_lines) for l in lines])
        if widest > w - 2 * pad:
            raise SystemExit(f"text overflows box {title!r}: {widest:.0f} > {w - 2 * pad}")
        ty = y + pad + title_size
        self.text(x + pad, ty, title, size=title_size, color=tcol, weight="600")
        for i, l in enumerate(lines):
            self.text(x + pad, ty + 20 + i * 18, l, size=12.5, color=lcol, mono=mono_lines)
        need = pad + title_size + 20 + max(len(lines) - 1, 0) * 18 + (pad - 2 if lines else 0)
        if lines and need > h:
            raise SystemExit(f"box {title!r} too short: needs {need}, has {h}")

    def frame(self, x, y, w, h, title, sub=None, style="slate"):
        fill, stroke, tcol, lcol = PAL[style]
        self.add(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="14" fill="{fill}" '
                 f'stroke="{stroke}" stroke-width="1.5"/>')
        self.text(x + 18, y + 28, title, size=15, color=tcol, weight="700")
        if sub:
            self.text(x + 18, y + 47, sub, size=12.5, color=lcol)

    def arrow(self, pts, label=None, label_at=None, dashed=False, accent=False, both=False,
              label_anchor="middle", label_color=None):
        d = "M" + " L".join(f"{x} {y}" for x, y in pts)
        color = "#4f46e5" if accent else "#475569"
        marker = "b" if accent else "a"
        extra = ' stroke-dasharray="5 4"' if dashed else ""
        start = f' marker-start="url(#{marker})"' if both else ""
        self.add(f'<path d="{d}" fill="none" stroke="{color}" stroke-width="{2 if accent else 1.5}"'
                 f'{extra}{start} marker-end="url(#{marker})"/>')
        if label:
            lx, ly = label_at
            for i, line in enumerate(label if isinstance(label, list) else [label]):
                self.text(lx, ly + i * 16, line, size=12, color=label_color or color, anchor=label_anchor)

    def save(self, name):
        OUT.mkdir(parents=True, exist_ok=True)
        (OUT / name).write_text("\n".join(self.parts + ["</svg>"]) + "\n")
        print("wrote", OUT / name)


def sequence(name, title, actors, steps, width=1000):
    """actors: [(label, sublabel, style)]; steps: (kind, from, to, text)
    kind: 'call' (solid), 'reply' (dashed), 'self' (note on one lifeline),
    'divider' (full-width text)."""
    left, right, top = 40, width - 40, 30
    n = len(actors)
    col = (right - left) / n
    xs = [left + col * i + col / 2 for i in range(n)]
    head_h = 62
    row = 40
    h = top + head_h + 24 + row * len(steps) + 30
    s = Svg(width, h, title)
    bw = col - 24
    for (label, sub, style), x in zip(actors, xs):
        s.box(x - bw / 2, top, bw, head_h, label, [sub] if sub else [], style=style)
    y0 = top + head_h
    y_end = h - 20
    for x in xs:
        s.add(f'<line x1="{x}" y1="{y0}" x2="{x}" y2="{y_end}" stroke="#cbd5e1" stroke-width="1.5" stroke-dasharray="3 4"/>')
    y = y0 + 24
    for kind, a, b, label in steps:
        y += row
        if kind == "divider":
            s.add(f'<line x1="{left}" y1="{y - 18}" x2="{right}" y2="{y - 18}" stroke="#e2e8f0"/>')
            s.text(width / 2, y - 4, label, size=12.5, color="#4338ca", weight="600", anchor="middle")
            continue
        if kind == "self":
            x = xs[a]
            tw = text_width(label, 12.5) + 24
            s.add(f'<rect x="{x - tw / 2}" y="{y - 26}" width="{tw}" height="28" rx="7" fill="#eef2ff" stroke="#a5b4fc"/>')
            s.text(x, y - 7, label, size=12.5, color="#312e81", anchor="middle")
            continue
        x1, x2 = xs[a], xs[b]
        span = abs(x2 - x1) - 16
        if text_width(label, 12.5) > span + 2 * col - 40 and abs(b - a) == 1:
            raise SystemExit(f"label too long for span: {label!r}")
        direction = 1 if x2 > x1 else -1
        s.arrow([(x1 + 4 * direction, y), (x2 - 4 * direction, y)], dashed=(kind == "reply"),
                accent=(kind == "call" and "Cloud Storage" in actors[b][0] or
                        kind == "call" and "Cloud Storage" in actors[a][0]))
        s.text((x1 + x2) / 2, y - 7, label, size=12.5,
               color="#0f172a" if kind == "call" else "#475569", anchor="middle")
    s.save(name)


# ---------------------------------------------------------------- sequences

ACTORS = [
    ("Browser", "Sender or recipient", "indigo"),
    ("file-vault", "Cloud Run service", "white"),
    ("Firestore", "Shares and tokens", "green"),
    ("Cloud Storage", "Ciphertext", "amber"),
]

sequence(
    "upload-sequence.svg",
    "Upload sequence",
    ACTORS,
    [
        ("self", 0, 0, "Generate a key, encrypt the metadata"),
        ("call", 0, 1, "POST /api/shares (size, encrypted metadata)"),
        ("call", 1, 2, "Pending share, admin token hash"),
        ("call", 1, 3, "Open resumable session, size pinned"),
        ("reply", 1, 0, "Session URL, admin token"),
        ("call", 0, 3, "PUT encrypted chunks of 8 MiB"),
        ("call", 0, 1, "POST /api/shares/finalize"),
        ("call", 1, 3, "Check the object size"),
        ("call", 1, 2, "Mark ready, first download token"),
        ("reply", 1, 0, "Download token"),
        ("self", 0, 0, "Build /d#token.key and /a#admin.key"),
    ],
)

sequence(
    "download-sequence.svg",
    "Download sequence",
    ACTORS,
    [
        ("self", 0, 0, "Read the token and the key from #"),
        ("call", 0, 1, "POST /api/download/peek"),
        ("call", 1, 2, "Read the token, no change"),
        ("reply", 1, 0, "Encrypted metadata, downloads remaining"),
        ("self", 0, 0, "Decrypt the file name, show Download"),
        ("divider", 0, 0, "The recipient clicks Download"),
        ("call", 0, 1, "POST /api/download/burn"),
        ("call", 1, 2, "Transaction: downloads remaining − 1"),
        ("reply", 1, 0, "Signed URL, valid 60 s"),
        ("call", 0, 3, "GET the ciphertext"),
        ("self", 0, 0, "Decrypt record by record, save"),
    ],
)

# ---------------------------------------------------------------- load balancer

s = Svg(1000, 470, "Load balancer request path")
s.box(24, 40, 160, 76, "DNS", ["A record → LB IP"], style="white")
s.box(24, 176, 160, 76, "Browser", ["https://<domain>"], style="indigo")
s.arrow([(104, 175), (104, 117)], dashed=True, label="Resolve", label_at=(112, 152), label_anchor="start")

s.frame(220, 24, 520, 330, "Your load balancer", "Created by you, outside file-vault")
s.box(240, 84, 150, 72, "Forwarding rule", ["Static IP, :443"])
s.box(410, 84, 150, 72, "HTTPS proxy", ["Managed cert"])
s.box(580, 84, 140, 72, "URL map", ["Host rule"])
s.arrow([(390, 120), (409, 120)])
s.arrow([(560, 120), (579, 120)])
s.arrow([(184, 214), (212, 214), (212, 120), (239, 120)])
s.box(240, 196, 300, 72, "Backend service", ["Logging on, Cloud Armor policy"])
s.arrow([(650, 156), (650, 180), (390, 180), (390, 195)])
s.text(258, 300, "X-Forwarded-For: <client-ip>, <lb-ip>", size=12, color="#475569", mono=True)
s.text(258, 320, "The service trusts the second entry from the right.", size=12, color="#475569")

s.frame(770, 24, 206, 330, "file-vault", "Created by its Terraform", style="indigo")
s.box(788, 196, 170, 72, "Serverless NEG", ["file-vault-<env>"], style="card", mono_lines=False)
s.box(788, 84, 170, 72, "Cloud Run", ["Ingress: LB only"], style="card")
s.arrow([(540, 232), (787, 232)])
s.arrow([(873, 196), (873, 157)])

s.text(500, 400, "Other hosts on the same load balancer keep their own host rules and backends.",
       size=12.5, color="#475569", anchor="middle")
s.text(500, 422, "Create the DNS record before the certificate: it validates only once the name resolves to the LB.",
       size=12.5, color="#475569", anchor="middle")
s.save("load-balancer.svg")

# ---------------------------------------------------------------- deploy pipeline

s = Svg(1000, 420, "Deploy pipeline")
s.box(24, 152, 140, 76, "git push", ["to main"], style="indigo")
s.frame(196, 24, 470, 370, "GitHub Actions", "deploy.yaml, on every push to main")
s.box(216, 76, 430, 104, "Build job", ["Authenticates as gha-build SA (WIF)", "Builds one image: UI embedded in the Go binary", "Pushes file-vault:<sha>"])
s.box(216, 216, 430, 104, "Deploy job", ["Authenticates as gha-tf SA (WIF)", "Writes terraform.tfvars from the TFVARS secret", "terraform apply -var image=file-vault:<sha>"])
s.arrow([(431, 180), (431, 215)])
s.text(441, 202, "image tag", size=12, color="#475569")
s.text(216, 348, "No stored keys: Workload Identity Federation exchanges each", size=12.5, color="#475569")
s.text(216, 368, "job's GitHub OIDC token for a short-lived Google token.", size=12.5, color="#475569")
s.arrow([(164, 190), (180, 190), (180, 128), (215, 128)])

s.frame(700, 24, 276, 370, "Google Cloud", "Your project")
s.box(718, 76, 240, 64, "Artifact Registry", ["file-vault:<sha>"], style="white")
s.box(718, 176, 240, 64, "Terraform state", ["GCS bucket, versioned"], style="white")
s.box(718, 260, 240, 104, "Applied resources", ["Cloud Run revision, NEG", "Bucket, Firestore, KMS", "Secret, alert policy"], style="white")
s.arrow([(646, 108), (717, 108)])
s.arrow([(646, 232), (717, 232)])
s.arrow([(646, 300), (717, 300)])
s.save("deploy-pipeline.svg")

# ---------------------------------------------------------------- security boundary

s = Svg(1000, 400, "What reaches Google Cloud")
s.frame(24, 24, 330, 350, "Your device", "Browser tab", style="green")
s.box(44, 84, 290, 140, "Stays in the browser", ["Unencrypted file content", "File name and type", "Decryption key (URL #fragment)"], style="white")
s.box(44, 244, 290, 110, "Sent, protected", ["Ciphertext, 1 MiB AES-GCM records", "Encrypted metadata blob", "Raw token, in a POST body only"], style="white")

s.frame(646, 24, 330, 350, "Google Cloud", "Service, Firestore, bucket, logs", style="slate")
s.box(666, 84, 290, 140, "Stored", ["Ciphertext in the bucket", "Encrypted metadata", "HMAC(pepper, token), downloads left"], style="white")
s.box(666, 244, 290, 110, "Visible", ["Ciphertext size", "Client IPs, kept 30 days", "Method, path, status in logs"], style="white")

s.add('<line x1="500" y1="40" x2="500" y2="360" stroke="#fb7185" stroke-width="2" stroke-dasharray="8 6"/>')
s.text(500, 34, "Trust boundary", size=12.5, color="#9f1239", weight="600", anchor="middle")
s.arrow([(334, 300), (665, 300)], accent=True)
s.add('<rect x="420" y="276" width="160" height="18" fill="#ffffff"/>')
s.text(500, 290, "Ciphertext and tokens", size=12, color="#4f46e5", anchor="middle")
s.arrow([(334, 150), (480, 150)], dashed=True)
s.text(410, 140, "Key", size=12.5, color="#9f1239", anchor="middle")
s.add('<path d="M486 140 l12 20 M498 140 l-12 20" stroke="#e11d48" stroke-width="2.5"/>')
s.text(484, 190, "The key is never", size=12, color="#9f1239", anchor="end")
s.text(484, 206, "sent to the server.", size=12, color="#9f1239", anchor="end")
s.save("security-boundary.svg")

# ---------------------------------------------------------------- encryption format

s = Svg(1000, 440, "Key derivation and FVE1 layout")
s.box(380, 24, 240, 64, "File key", ["32 random bytes, in the link #"], style="indigo")
s.box(200, 128, 270, 84, "Content key", ["info: file-vault/v1/content", "Encrypts every record"], style="card")
s.box(530, 128, 270, 84, "Metadata key", ["info: file-vault/v1/meta", "Encrypts name, type and size"], style="card")
s.arrow([(470, 88), (380, 127)])
s.arrow([(530, 88), (620, 127)])
s.text(500, 114, "HKDF-SHA256", size=12, color="#475569", anchor="middle")

y = 246
s.text(24, y - 12, "Ciphertext object", size=13.5, color="#0f172a", weight="600")
cells = [
    (24, 104, "FVE1", "magic, 4 B", "slate"),
    (140, 220, "Record 0", "1 MiB + 16 B tag", "amber"),
    (372, 220, "Record 1", "1 MiB + 16 B tag", "amber"),
    (604, 100, "…", "", "white"),
    (716, 260, "Last record", "≤ 1 MiB + 16 B tag", "amber"),
]
for x, w, t, sub, st in cells:
    s.box(x, y, w, 64, t, [sub] if sub else [], style=st, title_size=13)
s.text(24, y + 104, "Each record: AES-256-GCM with the content key.", size=12.5, color="#334155")
s.text(24, y + 124, "Nonce = 4 zero bytes + record index (64-bit big-endian): reordering fails.", size=12.5, color="#334155")
s.text(24, y + 144, "Additional data = 1 byte: 0x01 on the last record, 0x00 otherwise: truncation fails.", size=12.5, color="#334155")
s.arrow([(335, 212), (250, 245)], dashed=True)
s.save("encryption-format.svg")

# ---------------------------------------------------------------- local dev

s = Svg(1000, 300, "Local development")
s.box(24, 100, 160, 76, "Browser", ["localhost:5173"], style="indigo")
s.box(240, 96, 210, 84, "Vite dev server", ["Hot reload, proxies", "/api and /_local"], style="white")
s.frame(510, 24, 466, 250, "Go server, LOCAL_MODE=1", "localhost:8080, no GCP needed", style="slate")
s.box(530, 84, 200, 72, "JSON API", ["Same handlers as prod"], style="white")
s.box(756, 84, 200, 72, "store.Memory", ["In-memory shares"], style="green")
s.box(530, 176, 220, 84, "blob.Local", ["/_local upload, signed GET", "Files in $TMPDIR"], style="amber")
s.arrow([(184, 138), (239, 138)])
s.arrow([(450, 120), (529, 120)], label="/api", label_at=(490, 112))
s.arrow([(450, 160), (490, 160), (490, 218), (529, 218)])
s.text(482, 206, "/_local", size=12, color="#475569", anchor="end")
s.arrow([(730, 120), (755, 120)])
s.text(776, 204, "make e2e runs this same", size=12.5, color="#475569")
s.text(776, 222, "setup from Node with the", size=12.5, color="#475569")
s.text(776, 240, "browser modules.", size=12.5, color="#475569")
s.save("local-dev.svg")
