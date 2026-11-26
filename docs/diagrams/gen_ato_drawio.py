"""ATO architecture and scan pipeline as a draw.io file (two pages).

Regenerate: python3 gen_ato_drawio.py
Export:     /Applications/draw.io.app/Contents/MacOS/draw.io -x -f png -p 1 -b 10 -o ato-architecture.png ato-architecture.drawio
            /Applications/draw.io.app/Contents/MacOS/draw.io -x -f png -p 2 -b 10 -o ato-pipeline.png ato-architecture.drawio
Sources: internal/* package docs, internal/engine/engine.go (Scan, scanFile, scanContent), internal/engine/line.go,
internal/cli/cli.go (runScan, exit codes), internal/config (defaults), docs/HOW-IT-WORKS.md, ADR 2, 4, 7, 8, 13, 15.
The worked example is the README's own sample finding; no identifier value appears in the diagram.
"""
from xml.sax.saxutils import escape

esc = lambda s: escape(s, {'"': "&quot;"})
B = "<br>"

# ATO palette (the DevTalks deck): royal ink, oat paper, highlight yellow, pen red
INK, PAPER, SURFACE, LINE, MUTED, TEXT = "#2340B0", "#F3EFE6", "#FFFDF8", "#8C8475", "#57534C", "#1C1B19"
INK_T, HIGH_T, HIGH_S, RED_T, RED_S, OK_T, OK_S = "#DCE3FA", "#FBEFC0", "#C9A93E", "#FBE3E5", "#D7263D", "#D8F8F1", "#076357"
GREY = "#8A847A"
ARROW = "#545B64"
MONO = "font-family:SFMono-Regular,Menlo,Consolas,monospace;font-size:10px;color:#6A645B"


def code(s):
    return f'<span style="{MONO}">{s}</span>'


class Page:
    def __init__(self, pid, name, w, h):
        self.pid, self.name, self.w, self.h = pid, name, w, h
        self.back, self.mid, self.front = [], [], []
        self.n = 0

    def nid(self, p):
        self.n += 1
        return f"{self.pid}-{p}{self.n}"

    def cell(self, id, value, style, x, y, w, h, layer="front"):
        getattr(self, layer).append(
            f'<mxCell id="{id}" value="{esc(value)}" style="{style}" vertex="1" parent="1">'
            f'<mxGeometry x="{x:.0f}" y="{y:.0f}" width="{w:.0f}" height="{h:.0f}" as="geometry" /></mxCell>')
        return id

    def edge(self, s, t, label="", main=False, dashed=False, color=None, ortho=False, exit=None, entry=None, lx=0, width=None):
        color = color or (INK if main else GREY)
        width = width or (3 if main else 1.6)
        st = (f"edgeStyle={'orthogonalEdgeStyle' if ortho else 'none'};rounded=1;html=1;endArrow=block;endFill=1;endSize=5;"
              f"strokeWidth={width};strokeColor={color};fontSize=11;fontColor={MUTED};labelBackgroundColor={PAPER};")
        if dashed:
            st += "dashed=1;dashPattern=6 4;"
        if exit:
            st += f"exitX={exit[0]};exitY={exit[1]};exitDx=0;exitDy=0;"
        if entry:
            st += f"entryX={entry[0]};entryY={entry[1]};entryDx=0;entryDy=0;"
        self.mid.append(f'<mxCell id="{self.nid("e")}" value="{esc(label)}" style="{st}" edge="1" parent="1" source="{s}" target="{t}">'
                        f'<mxGeometry x="{lx}" relative="1" as="geometry" /></mxCell>')

    def text(self, value, x, y, w, h, size=12, align="left", color=TEXT, layer="front", extra=""):
        return self.cell(self.nid("t"), value, f"text;html=1;align={align};verticalAlign=top;whiteSpace=wrap;fontSize={size};fontColor={color};spacing=0;{extra}",
                         x, y, w, h, layer)

    def xml(self, title, sub):
        bg = (f'<mxCell id="{self.pid}-bg" value="" style="rounded=0;html=1;fillColor={PAPER};strokeColor=none;" vertex="1" parent="1">'
              f'<mxGeometry x="0" y="0" width="{self.w}" height="{self.h}" as="geometry" /></mxCell>')
        self.text(f"<b>{title}</b>", 40, 32, 1600, 30, size=22, layer="back")
        self.text(sub, 40, 66, 1700, 24, size=13.5, color=MUTED, layer="back")
        body = "\n".join([bg] + self.back + self.mid + self.front)
        return (f'<diagram id="{self.pid}" name="{esc(self.name)}"><mxGraphModel dx="2800" dy="1600" grid="0" gridSize="10" guides="1" tooltips="1" '
                f'connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="{self.w}" pageHeight="{self.h}" math="0" shadow="0">'
                f'<root><mxCell id="0" /><mxCell id="1" parent="0" />{body}</root></mxGraphModel></diagram>')


# =====================================================================================================
# Page 1: architecture. One main row, left to right; control above, side readers below.
# =====================================================================================================
A = Page("arch", "1 · Architecture", 3360, 1330)
X0, Y0, T = 300, 640, 100


def P(s, d):
    return X0 + T * s, Y0 + T / 2 * d


def label(id, cx, cy, name, what="", ident="", w=124):
    v = f'<b style="color:{TEXT};font-size:12px">{name}</b>'
    if what:
        v += B + what
    if ident:
        v += B + code(ident)
    A.cell(id + "-l", v, f"text;html=1;align=center;verticalAlign=top;whiteSpace=wrap;fontSize=11;fontColor={MUTED};spacing=0;", cx - w / 2, cy + 5, w, 70)


def shape(id, s, d, style, w, h, name, what, ident, lw):
    cx, cy = P(s, d)
    A.cell(id, "", style + "html=1;", cx - w / 2, cy - h, w, h)
    label(id, cx, cy, name, what, ident, lw)
    return id


def cube(id, s, d, name, what="", ident="", fill=SURFACE, stroke=LINE, lw=124, w=50, h=54):
    return shape(id, s, d, f"shape=isoCube2;isoAngle=15;fillColor={fill};strokeColor={stroke};strokeWidth=1.2;", w, h, name, what, ident, lw)


def chip(id, s, d, name, what="", ident="", fill=SURFACE, stroke=LINE, lw=124):
    return cube(id, s, d, name, what, ident, fill, stroke, lw, 34, 30)


def cyl(id, s, d, name, what="", ident="", fill=SURFACE, stroke=LINE, lw=124):
    return shape(id, s, d, f"shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=9;fillColor={fill};strokeColor={stroke};strokeWidth=1.2;", 50, 56, name, what, ident, lw)


def person(id, s, d, name, what):
    return shape(id, s, d, "shape=mxgraph.aws3d.end_user;strokeColor=none;fillColor=#6A645B;aspect=fixed;", 40, 80, name, what, "", 150)


def badge(n, s, d, dx=-38, dy=-74):
    cx, cy = P(s, d)
    A.cell(A.nid("n"), str(n), f"ellipse;html=1;fillColor={INK};strokeColor={PAPER};strokeWidth=2;fontColor=#FFFFFF;fontStyle=1;fontSize=13;align=center;verticalAlign=middle;",
           cx + dx - 13, cy + dy - 13, 26, 26)


def plate(id, sc, dc, r, title, sub, tint="#E8E2D6", stroke="#CFC6B6", pos="top"):
    x, y = P(sc - r, dc - r)
    A.cell(id, "", f"rhombus;whiteSpace=wrap;html=1;fillColor={tint};strokeColor={stroke};strokeWidth=1.5;", x, y, 2 * T * r, T * r, "back")
    if pos == "left":
        tx, ty = P(sc - r, dc)
        A.text(f"<b style=\"font-size:13px\">{title}</b>{B}<span style=\"color:{MUTED}\">{sub}</span>", tx - 262, ty - 28, 248, 60, 12, "right", layer="back")
    else:
        tx, ty = P(sc, dc - r)
        A.text(f"<b style=\"font-size:13px\">{title}</b>{B}<span style=\"color:{MUTED}\">{sub}</span>", tx - 270, ty - 44, 540, 40, 12, "center", layer="back")


# Zones: the main row sits on d = 0
plate("z-src", 2.5, 0, 2.3, "Inputs", "what you point ATO at · read only")
plate("z-walk", 7.6, 0, 1.9, "Walk", "pick the files · never runs the repo's code")
plate("z-eng", 13.85, 0, 3.8, "Engine", "read every file in parallel, turn values into findings", tint="#DFE4F5", stroke="#AEB9E2")
plate("z-dec", 20.9, 0, 2.8, "Decide", "drop what you accepted, choose the exit code", tint="#F5EBC8", stroke="#E0CF94")
plate("z-out", 26.3, 0, 2.4, "Report", "masked values only")
plate("z-cli", 7.6, -6.2, 1.9, "CLI", "reads flags and config, prints, exits")
plate("z-st", 13.85, 8.0, 3.2, "Structure readers", "what the data is, where it goes, how it is deleted")
plate("z-oth", 21.8, 7.6, 2.4, "Other commands", "built from the same scan")
plate("z-fx", 6.6, 8.0, 1.7, "Fixtures", "replace real-looking test data", pos="left")

# Who runs it
person("dev", -1.3, -6.8, "Developer", "laptop · pre-commit hook")
cube("ci", -1.3, -3.4, "CI pipeline", "GitHub · GitLab · Bitbucket" + B + "license from ATO_LICENSE", fill="#ECE6DA", lw=170)

# CLI
cube("cli", 7.0, -6.2, "ato", "scan · inventory · map · fix", "internal/cli", fill=INK_T, stroke=INK, lw=150)
chip("cfg", 8.5, -7.1, "Settings", "limits, detectors, countries", ".ato.toml", lw=140)
chip("lic", 8.5, -5.3, "License", "checked offline", "Ed25519", lw=120)

# Inputs
cube("tree", 1.7, -1.2, "Work tree", "code · logs · CSV · SQL dumps" + B + "Terraform", lw=150)
cube("staged", 3.3, -1.2, "Staged changes", "for pre-commit", "--staged")
cube("stdin", 1.7, 1.3, "A stream", "piped logs, read directly" + B + "by the engine", "ato scan -", lw=150)
cyl("image", 3.3, 1.3, "Container image", "saved with docker save", "--image app.tar")

# Walk
cube("walk", 7.0, -0.4, "List files", "git's list, so .gitignore" + B + "applies; risky git off", "walk.Files", lw=150)
chip("arch", 8.5, -0.4, "Archives", "tar · gz · zip, capped", "", lw=110)
chip("img", 8.0, 1.4, "Image layers", "each layer + config", "OpenImage", lw=120)

# Engine: one chain, left to right
EL = 120
cube("pool", 10.6, 0, "Workers", "one file per CPU core", "parallel()", fill=INK_T, stroke=INK, lw=EL)
cube("route", 11.9, 0, "Route by type", "looks at the first 8 KB", "scanContent", fill=INK_T, stroke=INK, lw=EL)
cube("line", 13.2, 0, "Find candidates", "line by line, also in" + B + "URL and base64 text", "scanText", fill=INK_T, stroke=INK, lw=EL)
cyl("det", 14.5, 0, "Validate", "check digits, dates," + B + "lengths (120 detectors)", "detectors", fill=INK_T, stroke=INK, lw=EL)
cube("ctx", 15.8, 0, "Score and mask", "keywords raise it," + B + "test paths lower it", "line.go", fill=INK_T, stroke=INK, lw=EL)
cube("fold", 17.1, 0, "Collect", "merge all workers," + B + "sort", "fold", fill=INK_T, stroke=INK, lw=EL)
cube("bin", 12.6, -2.75, "Binary metadata", "photo GPS · authors · PDF · pg_dump", "", lw=220)

# Decide
chip("ign", 19.4, 0, "Ignore comments", "lines marked ato:ignore", "", HIGH_T, HIGH_S, lw=120)
chip("base", 20.7, 0, "Baseline", "only new findings pass", ".ato-baseline.json", HIGH_T, HIGH_S, lw=120)
chip("minc", 22.0, 0, "Min confidence", "default: medium", "", HIGH_T, HIGH_S, lw=120)
cube("exit", 21.4, 2.2, "Exit code", "1 findings · 3 not all read" + B + "0 clean · 2 usage error", "", HIGH_T, HIGH_S, lw=170)

# Report
cube("rep", 25.4, 0, "Scan report", "terminal, JSON, SARIF," + B + "GitHub, GitLab, HTML", "internal/report", lw=150)
chip("pr", 27.3, -0.5, "Pull request", "annotations, code scanning", "", lw=130)

# Structure readers
chip("schema", 12.45, 6.7, "Schemas, models", "SQL, ORM, OpenAPI, proto", "schema", lw=130)
chip("classify", 13.85, 6.7, "Field names", "which category, Art. 9/10", "classify", lw=130)
chip("infra", 15.25, 6.7, "Terraform", "stores, encryption, region", "infra", lw=130)
chip("sinks", 12.45, 8.7, "Logging calls", "personal data in logs", "sinks", lw=130)
chip("recip", 13.85, 8.7, "Third parties", "deps, imports, API hosts", "recipients", lw=130)
chip("life", 15.25, 8.7, "Deletion paths", "erase, TTL, retention", "lifecycle", lw=130)
chip("art9", 13.85, 10.6, "Health records", "HL7, FHIR, VCF, biometrics", "art9", lw=140)

# Other commands
chip("inv", 20.9, 7.0, "Inventory", "every personal-data field", "ato inventory", lw=130)
chip("map", 22.6, 7.0, "Data map", "stores, logs, recipients", "ato map", lw=130)
chip("diff", 21.75, 8.9, "Map diff", "fails a PR on new data", "ato map --diff", lw=140)

# Fixtures
cube("fix", 6.0, 8.0, "Replace test data", "runs a scan, then rewrites" + B + "test paths", "ato fix", lw=150)
cyl("synth", 7.4, 8.0, "Generators", "valid, made-up values" + B + "with a synthetic mark", "synth", lw=140)

# Main path (thick blue): one `ato scan .`
A.edge("dev", "cli", "ato scan .", main=True)
A.edge("cli", "walk", "one request", main=True, lx=0.35)
A.edge("walk", "pool", "file list", main=True, lx=0.2)
A.edge("pool", "route", "", main=True)
A.edge("route", "line", "", main=True)
A.edge("line", "det", "", main=True)
A.edge("det", "ctx", "", main=True)
A.edge("ctx", "fold", "", main=True)
A.edge("fold", "ign", "", main=True)
A.edge("ign", "base", "", main=True)
A.edge("base", "minc", "", main=True)
A.edge("minc", "rep", "findings", main=True, lx=0.1)
A.edge("rep", "pr", "")
# Side paths (thin grey)
A.edge("ci", "cli", "--format sarif", lx=-0.2)
A.edge("cli", "cfg", "", dashed=True)
A.edge("cli", "lic", "", dashed=True)
A.edge("tree", "walk", "")
A.edge("staged", "walk", "")
A.edge("image", "img", "")
A.edge("walk", "arch", "")
A.edge("img", "pool", "layers")
A.edge("route", "bin", "")
A.edge("bin", "fold", "")
A.edge("route", "z-st", "schemas, Terraform, deps", lx=0.2)
A.edge("z-st", "fold", "fields, stores, recipients", lx=0.3)
A.edge("minc", "exit", "", dashed=True)
A.edge("fold", "z-oth", "same results", lx=-0.1)
A.edge("map", "diff", "")
A.edge("fix", "synth", "")

# Numbered route
for n, (s, d) in enumerate([(-1.3, -6.8), (7.0, -0.4), (10.6, 0), (11.9, 0), (13.2, 0), (14.5, 0), (15.8, 0), (17.1, 0), (20.7, 0), (25.4, 0)], 1):
    badge(n, s, d, dy=-92 if n == 1 else (-50 if n == 9 else -74))

steps = [
    "You run <b>ato scan</b> on a laptop, in a pre-commit hook or in CI. The CLI reads .ato.toml, checks the license offline and builds one request.",
    "<b>Walk</b> lists the files. It uses git's own list so .gitignore applies, with every git setting that could run code turned off (ADR 7). Archives and image layers open in place.",
    "<b>Workers</b>, one per CPU core, each take one file at a time.",
    "Each file is <b>routed</b> by its first 8 KB: binary metadata, Terraform, data files by column, schemas and models, or plain text.",
    "Each line is checked for <b>candidates</b>: a quick prefilter, then each detector's pattern. URL-encoded and base64 text is decoded and checked too.",
    "Only values that pass the identifier's own rules (<b>check digits, dates, lengths</b>) go on. Lookalikes stop here.",
    "<b>Confidence</b> rises for a nearby keyword or the usual format and drops in test paths. The value is <b>masked</b> here and never shown again.",
    "<b>Collect</b> merges every worker's results, with the fields, stores and recipients from the structure readers.",
    "The CLI drops findings on <b>ato:ignore</b> lines, in the <b>baseline</b>, and under the <b>minimum confidence</b>.",
    "The <b>report</b> is written and the <b>exit code</b> set: 1 if anything was found, 3 if a file was not read in full, otherwise 0.",
]
rows = "".join(f'<tr><td style="vertical-align:top;padding:3px 10px 5px 0"><span style="display:inline-block;width:20px;height:20px;line-height:20px;border-radius:10px;background:{INK};color:#fff;text-align:center;font-weight:bold">{i}</span></td>'
               f'<td style="vertical-align:top;padding:3px 18px 5px 0">{t}</td></tr>' for i, t in enumerate(steps, 1))
half = rows.split("</tr>")
col1 = "</tr>".join(half[:5]) + "</tr>"
col2 = "</tr>".join(half[5:10]) + "</tr>"
A.cell("steps", f'<b style="font-size:14px">Follow the numbers: one <span style="{MONO};font-size:12px">ato scan .</span></b>'
       f'<table style="margin-top:8px;font-size:12px;color:{TEXT}"><tr><td style="vertical-align:top;width:50%"><table>{col1}</table></td><td style="vertical-align:top"><table>{col2}</table></td></tr></table>',
       f"text;html=1;align=left;verticalAlign=top;whiteSpace=wrap;fontSize=12;spacing=16;fillColor={SURFACE};strokeColor=#DED7CA;rounded=1;arcSize=2;fontColor={TEXT};",
       2010, 110, 1300, 262)

legend = (f'<b style="font-size:13px">Key</b><br>'
          f'<span style="color:{INK};font-weight:bold">━━</span> the path of one scan &nbsp; <span style="color:{GREY}">──</span> side path &nbsp; <span style="color:{GREY}">╌╌</span> settings or result<br>'
          f'Blue: runs on every scan · yellow: gates in the CLI · grey: optional or other commands<br><br>'
          f'<b style="font-size:13px">Guarantees in the code</b><br>'
          f'No network calls; the license is checked offline (ADR 15)<br>'
          f'Values are masked at step 7; reports, baselines and SARIF never hold them<br>'
          f'A scanned repo\'s .git/config cannot run commands (ADR 7)<br>'
          f'A file not read in full means exit 3, never a silent pass (ADR 8)<br>'
          f'Binary readers treat input as hostile and fail closed (ADR 13)<br><br>'
          f'<b style="font-size:13px">Defaults</b><br>files up to 5 MiB · 100,000 rows per data file<br>archives up to 4 GiB, 1M entries, 3 levels deep')
A.cell("legend", legend, f"text;html=1;align=left;verticalAlign=top;whiteSpace=wrap;fontSize=12;spacing=16;fillColor={SURFACE};strokeColor=#DED7CA;rounded=1;arcSize=3;fontColor={TEXT};",
       2790, 830, 530, 330)

# =====================================================================================================
# Page 2: the scan pipeline, step by step, with one line traced through it
# =====================================================================================================
F = Page("pipe", "2 · Scan pipeline", 2240, 1500)
BOX = f"rounded=1;whiteSpace=wrap;html=1;arcSize=6;fontSize=11.5;fontColor={TEXT};spacing=9;align=left;verticalAlign=top;strokeWidth=1.2;"
TRACE = f"rounded=1;whiteSpace=wrap;html=1;arcSize=10;fontSize=11;fontColor={OK_S};spacing=7;align=left;verticalAlign=top;strokeWidth=1;fillColor={OK_T};strokeColor={OK_S};dashed=1;dashPattern=3 3;"


def box(x, y, title, body, fill=SURFACE, stroke=LINE, w=210, h=104, num=None):
    head = (f'<span style="color:{INK};font-weight:bold">{num}</span>&nbsp; ' if num else "") + f"<b>{title}</b>"
    return F.cell(F.nid("b"), f"{head}<br><span style=\"color:{MUTED}\">{body}</span>", BOX + f"fillColor={fill};strokeColor={stroke};", x, y, w, h)


def trace(x, y, body, w=240, h=74):
    return F.cell(F.nid("tr"), body, TRACE, x, y, w, h)


def diamond(x, y, txt, w=170, h=96):
    return F.cell(F.nid("d"), txt, f"rhombus;whiteSpace=wrap;html=1;fontSize=11;fontColor={TEXT};fillColor={HIGH_T};strokeColor={HIGH_S};strokeWidth=1.2;", x, y, w, h)


def section(y, n, title, sub):
    F.cell(F.nid("sn"), str(n), f"ellipse;html=1;fillColor={INK};strokeColor=none;fontColor=#FFFFFF;fontStyle=1;fontSize=14;align=center;verticalAlign=middle;", 40, y, 28, 28, "back")
    F.text(f"<b style=\"font-size:15px\">{title}</b> &nbsp;<span style=\"color:{MUTED}\">{sub}</span>", 80, y + 4, 1900, 26, 13, layer="back")
    F.cell(F.nid("r"), "", "line;html=1;strokeColor=#DED7CA;strokeWidth=1;", 40, y + 34, 2160, 4, "back")


# Key under the title
F.text(f'<span style="background:{INK_T};border:1px solid {INK};padding:1px 8px">core logic</span> &nbsp; '
       f'<span style="background:{HIGH_T};border:1px solid {HIGH_S};padding:1px 8px">decision or gate</span> &nbsp; '
       f'<span style="background:{RED_T};border:1px solid {RED_S};padding:1px 8px">not read: exit 3</span> &nbsp; '
       f'<span style="background:{OK_T};border:1px dashed {OK_S};padding:1px 8px;color:{OK_S}">worked example: one line traced through every step</span>',
       40, 98, 1800, 22, 12, layer="back")

# 1 · Pick files
section(140, 1, "Pick files", "which files are read, and how safely · internal/walk")
w1 = box(40, 190, "Git's file list", "In a git work tree, git ls-files, so .gitignore applies exactly as git applies it. Settings that run commands (fsmonitor, hooks, pager, external diff, credential helper) are forced off (ADR 7). Without git: a plain directory walk.", INK_T, INK, w=330, h=124)
w2 = box(400, 190, "Staged changes (--staged)", "For pre-commit: the staged file names, and their content read from the index with git cat-file. No filters or text conversion run.", w=280, h=124)
w3 = box(710, 190, "Archives", "tar, .gz and .zip are opened in place. Caps: 4 GiB, 1M entries, 3 levels deep. Over a cap means not read in full.", w=260, h=124)
w4 = box(1000, 190, "Container images (--image)", "A docker save tar. Each layer is scanned in parallel, plus the image config (environment, labels, history).", w=280, h=124)
w5 = box(1310, 190, "A stream (ato scan -)", "Piped text, such as a log tail, goes straight to the line scanner. No file list, no baseline.", w=250, h=124)
w6 = box(1590, 190, "One request", "Everything later steps need: detectors on by config and --countries, minimum confidence, row limit, which extra readers to run.", "#ECE6DA", LINE, w=300, h=124)

# 2 · Route each file
section(350, 2, "Route each file", "one worker per CPU core, one file per job; the first 8 KB decide the route · engine.scanContent")
pool = box(40, 410, "Workers", "runtime.NumCPU() goroutines take file numbers from a channel. Each returns its results; they are merged in order at the end.", INK_T, INK, w=240, h=124)
qs = ["known binary<br>format?", "other binary?", "Terraform?", "data file?", "schema, model,<br>deps or deletion code?"]
ds = [diamond(330 + i * 300, 422, t) for i, t in enumerate(qs)]
rs = [
    box(310, 575, "Read the metadata", "<i>photo.jpg, contract.docx, report.pdf, db.dump</i><br>GPS in photos, document authors, PDF text, pg_dump tables. Hostile input, fail closed (ADR 13).", w=210, h=136),
    box(610, 575, "Skip", "<i>logo.png, app.exe</i>: skipped.<br><i>sales.xlsx, data.sqlite, x.7z</i>: formats that often hold personal data but are not opened, so marked not read: exit 3 (ADR 8).", RED_T, RED_S, w=210, h=136),
    box(910, 575, "Stores and lines", "<i>main.tf</i><br>Data stores with encryption, retention and region (no Terraform run, unknown is never guessed), plus the normal line scan.", w=210, h=136),
    box(1210, 575, "By column", "<i>users.csv, events.jsonl, dump.sql</i><br>Headers go to field-name checks; values are scanned with their column name. Over 100,000 rows: sampled, marked not read.", w=210, h=136),
    box(1510, 575, "Whole file", "<i>models.py, schema.prisma, package.json</i><br>Field names, third-party services and deletion paths, plus the normal line scan.", w=210, h=136),
]
other = box(1830, 422, "Line by line", "<i>app.py, server.log, users.json</i><br>Everything else that is text. Logging calls are also checked for personal data.", INK_T, INK, w=250, h=124)
F.edge(pool, ds[0], "", color=ARROW)
for i, d in enumerate(ds):
    F.edge(d, rs[i], "yes", color=ARROW, exit=(0.5, 1), entry=(0.5, 0))
    F.edge(d, ds[i + 1] if i + 1 < len(ds) else other, "no", color=ARROW)

# 3 · One line, one finding
section(750, 3, "One line becomes one finding", "every text line goes through these eight steps in order · engine/line.go and the detector registry")
F.text(f'<span style="color:{OK_S}"><b>Worked example</b>, the README\'s sample: line 4 of <span style="{MONO};font-size:11px;color:{OK_S}">tests/fixtures/users.json</span> is <span style="{MONO};font-size:11px;color:{OK_S}">"cnp": "&lt;13-digit CNP&gt;"</span>, and line 6 has an <span style="{MONO};font-size:11px;color:{OK_S}">order_id</span> with the same 13-digit shape.</span>',
       40, 790, 2100, 20, 12, layer="back")
steps = [
    ("Prefilter", "Skips detectors that cannot match this line: too few digits, a required word missing. Keeps scans fast as detectors are added.",
     "13 digits on the line: the CNP detector and others that need 13 digits stay; most are skipped."),
    ("Pattern", "Each remaining detector's pattern finds candidates. The characters around a candidate must not be letters or digits.",
     "The CNP pattern matches the 13-digit run. The quotes around it are clean edges."),
    ("Decode", "The line is also read with URL escapes and whole base64 tokens (12 to 64 characters) decoded. A finding says which encoding it was in.",
     "Plain text: nothing to decode."),
    ("Validate", "The identifier's own rules: check digits, dates, lengths, region codes. Only valid values go on. A product barcode next to the word barcode is not a card.",
     "Date, county code and check digit are valid.<br><b style=\"color:#8E1B2A\">order_id: month 41 does not exist. Dropped.</b>"),
    ("Guards", "Some identifiers need a keyword nearby. Numbers inside UUIDs, millisecond timestamps and number tables are ignored.",
     "Not inside a UUID, a timestamp or a table of numbers."),
    ("Confidence", "Starts at the detector's base. +1 for the usual format, +1 for a keyword on the line, the line before or the column (24 languages). −1 in test paths, −1 next to order or invoice. Published samples: low.",
     "Base medium. +1 for the word cnp: high. −1 for the tests/ path: <b>medium</b>."),
    ("Overlaps", "The most confident match wins. A value valid for two detectors is reported once; the nearby name decides, or it says Possibly X or Y.",
     "No other detector accepts it: reported once."),
    ("Mask", "The value is masked here. Reports, baselines and SARIF never see it again (--show-values only prints to a terminal).",
     "Shown as ••••••••••••• from now on."),
]
prev = None
for i, (t, b, ex) in enumerate(steps):
    x = 40 + i * 270
    core = t in ("Validate", "Confidence", "Mask")
    bid = box(x, 822, t, b, INK_T if core else SURFACE, INK if core else LINE, w=240, h=150, num=f"3.{i + 1}")
    if prev:
        F.edge(prev, bid, "", color=ARROW)
    tid = trace(x, 984, ex, h=82)
    prev = bid

# 4 · Decide and report
section(1100, 4, "Decide and report", "results are merged, then the CLI filters them, writes the report and picks the exit code · engine.fold, cli.runScan")
d_steps = [
    ("Collect", "Merge every worker's results, count files, sort. Also gather fields, Terraform stores, third parties and deletion paths.", INK_T, INK,
     "1 finding in users.json."),
    ("Ignore comments", "Findings on a line with an ato:ignore comment are dropped and counted.", HIGH_T, HIGH_S,
     "No ato:ignore on line 4: kept."),
    ("Baseline", ".ato-baseline.json holds fingerprints made from the detector, the path and the line with values replaced. Only new findings pass.", HIGH_T, HIGH_S,
     "Fingerprint not in the baseline: new, kept."),
    ("Minimum confidence", "Default medium, or set per detector in .ato.toml. Low findings, such as published sample values, are hidden by default.", HIGH_T, HIGH_S,
     "medium ≥ medium: kept."),
    ("Report", "Terminal, JSON, SARIF, GitHub annotations, GitLab Code Quality or HTML. Masked values only.", SURFACE, LINE,
     "<span style=\"font-family:Menlo,monospace;font-size:10px\">4:13 medium Romanian personal number (CNP) •••••••••••••</span>"),
]
prev = None
for i, (t, b, fill, stroke, ex) in enumerate(d_steps):
    x = 40 + i * 300
    bid = box(x, 1150, t, b, fill, stroke, w=260, h=120, num=f"4.{i + 1}")
    if prev:
        F.edge(prev, bid, "", color=ARROW)
    trace(x, 1282, ex, w=260, h=60)
    prev = bid
ex = F.cell(F.nid("x"), f"<span style=\"color:{INK};font-weight:bold\">4.6</span>&nbsp; <b>Exit code</b><br>1 · findings (wins over 3)<br>3 · a file not read in full<br>&nbsp;&nbsp;&nbsp;&nbsp;(--allow-partial keeps 0 or 1)<br>0 · nothing found, everything read<br>2 · usage or settings error",
            BOX + f"fillColor={HIGH_T};strokeColor={HIGH_S};strokeWidth=1.6;", 1540, 1150, 260, 132)
F.edge(prev, ex, "", color=ARROW)
trace(1540, 1294, "<b>Exit 1.</b> In CI the step fails and the pull request shows the finding.", w=260, h=48)
side = [
    ("ato inventory", "Every personal-data field with its table, category, special category, file and line. CSV, JSON or HTML: a first draft of the Art. 30 record."),
    ("ato map, ato map --diff", "Per component: stores and categories, logs of personal data, third parties (EEA or not). --diff exits 1 when anything is new."),
]
for i, (t, b) in enumerate(side):
    F.cell(F.nid("o"), f"<b>{t}</b><br><span style=\"color:{MUTED}\">{b}</span>", BOX + f"fillColor={SURFACE};strokeColor={LINE};dashed=1;", 1880, 1150 + i * 150, 320, 128)
F.cell("pipe-lic", "<b>License, off the scan path</b><br>"
       "A signed token (Ed25519) in ATO_LICENSE or the user's config folder, checked offline. Valid until its end date plus 30 days. "
       "Without one ATO still scans and prints one line on stderr; output and exit codes are the same (ADR 15).",
       BOX + f"fillColor=#ECE6DA;strokeColor={LINE};", 40, 1380, 900, 84)
F.cell("pipe-fix", "<b>ato fix</b><br>Runs the same scan, then rewrites values in test, fixture and example paths with made-up ones that pass the same detector, "
       "keep the format and country, and carry a synthetic mark, so later scans report them as low-confidence samples.",
       BOX + f"fillColor={OK_T};strokeColor={OK_S};", 970, 1380, 880, 84)

out = ('<mxfile host="app.diagrams.net">'
       + A.xml("ATO · how a scan works", "Clause Eight's EU personal-data scanner · one Go binary, no network calls · follow the blue numbers from the developer to the report · pii-eu main, 31 Oct 2026")
       + F.xml("ATO · scan pipeline, step by step", "How files are chosen and routed, how one line becomes a masked finding, and how the CLI decides what to report · one fixture line traced through every step")
       + "</mxfile>")
open("ato-architecture.drawio", "w").write(out)
print("wrote ato-architecture.drawio")
