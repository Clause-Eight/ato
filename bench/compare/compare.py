"""Compare pii-eu with Microsoft Presidio, scrubadub and HoundDog.ai on the same inputs (ADR 9).

Three tests, each with every tool at its documented defaults, adjusted only where the default would be unfair:

  values-synthetic  the pii-eu bench corpus (go run ./bench -export), several seeds
  values-oss        files from real open-source applications (fixtures, migrations, seeders, specs) plus a
                    generated application log, with generated values inserted on new lines
  sinks             generated code in eight languages: personal fields passed to log calls, and harmless ones

Every value is generated and belongs to nobody. See README.md for how to run it.

    venv/bin/python -I compare.py --pii-eu ./pii-eu --oss oss/ --out results/
"""

from __future__ import annotations

import argparse
import importlib.metadata as md
import json
import random
import re
import shutil
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

HERE = Path(__file__).resolve().parent
TOOLS = {"pii-eu", "presidio", "scrubadub", "hounddog"}
REPO = HERE.parent.parent

# Identifiers inserted into the OSS corpus: every one is on by default in pii-eu.
SEEDED = ["ro-cnp", "bg-egn", "hu-taj", "pl-pesel", "cz-birth-number", "hr-oib", "de-tax-id", "es-dni",
          "it-fiscal-code", "fr-nir", "nl-bsn", "iban", "card", "email", "eu-phone"]
NEUTRAL = ["ref", "value", "code", "external_id", "data", "number", "token_ref"]
OSS_DIRS = re.compile(r"(^|/)(fixtures?|migrations?|seeders?|seeds?|factories|spec|tests?|database|db|data|samples?)/", re.I)
PER_REPO_LINES = 25_000
OSS_EXT = {".json", ".yml", ".yaml", ".sql", ".py", ".rb", ".php", ".js", ".ts"}


# ---------------------------------------------------------------- tools

def presidio_engine():
    from presidio_analyzer import AnalyzerEngine, BatchAnalyzerEngine, RecognizerRegistry
    from presidio_analyzer import predefined_recognizers as rec
    from presidio_analyzer.nlp_engine import NlpEngineProvider

    nlp = NlpEngineProvider(nlp_configuration={
        "nlp_engine_name": "spacy", "models": [{"lang_code": "en", "model_name": "en_core_web_sm"}]}).create_engine()
    registry = RecognizerRegistry(supported_languages=["en"])
    registry.load_predefined_recognizers(languages=["en"])
    # Country recognizers ship for their own language only; enable them for English so Presidio is not
    # penalised for its language defaults.
    eu = ("AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "ES", "FI", "FR", "GR", "HR", "HU", "IE", "IT", "LT",
          "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK")
    extra = [("PhoneRecognizer", {"supported_regions": eu})]
    for name in dir(rec):
        if name.endswith("Recognizer") and name[:2] in {"Es", "It", "Pl", "Fi", "Se", "Uk", "De", "Nhs"}:
            extra.append((name, {"supported_language": "en"}))
    for name, kwargs in extra:
        try:
            registry.add_recognizer(getattr(rec, name)(**kwargs))
        except Exception:
            pass
    analyzer = AnalyzerEngine(nlp_engine=nlp, registry=registry, supported_languages=["en"])
    # Named-entity, date and URL recognisers are built for prose; on code they flag identifiers, dates and
    # call sites. Scanning code for personal data, a user would leave them out, so we do.
    prose = {"PERSON", "LOCATION", "NRP", "ORGANIZATION", "DATE_TIME", "URL"}
    entities = sorted({e for r in registry.recognizers for e in r.supported_entities} - prose)
    return BatchAnalyzerEngine(analyzer_engine=analyzer), entities


def run_presidio(lines: list[str]):
    batch, entities = presidio_engine()
    start = time.perf_counter()
    results = batch.analyze_iterator(lines, language="en", entities=entities, batch_size=256, n_process=1)
    hits = [(i, r.entity_type) for i, rs in enumerate(results) for r in rs if r.score >= 0.5]
    return hits, time.perf_counter() - start


def run_scrubadub(lines: list[str]):
    import scrubadub
    s = scrubadub.Scrubber()
    start = time.perf_counter()
    hits = [(i, f.type) for i, text in enumerate(lines) for f in s.iter_filth(text)]
    return hits, time.perf_counter() - start


def run_piieu(binary: Path, folder: Path):
    """Default settings; the timed run is the scored run."""
    start = time.perf_counter()
    out = subprocess.run([str(binary), "scan", "--no-git", "--allow-partial", "--format", "json", str(folder)],
                         capture_output=True, text=True, check=False)
    elapsed = time.perf_counter() - start
    if out.returncode not in (0, 1):
        sys.exit(f"pii-eu exited {out.returncode}: {out.stderr}")
    return json.loads(out.stdout)["findings"], elapsed


# ---------------------------------------------------------------- scoring

def score(labels: dict, total_lines: int, flagged: dict) -> dict:
    """labels: line key -> detector. flagged: line key -> list of entity types (per value)."""
    caught = {k for k in flagged if k in labels}
    alarms = {k: v for k, v in flagged.items() if k not in labels}
    by_det = defaultdict(lambda: [0, 0])
    for k, d in labels.items():
        by_det[d][1] += 1
        if k in caught:
            by_det[d][0] += 1
    alarm_values = sum(len(v) for v in alarms.values())
    real_values = sum(len(flagged[k]) for k in caught)
    return {
        "caught": len(caught), "values": len(labels), "recall": len(caught) / len(labels),
        "alarm_lines": len(alarms), "alarm_values": alarm_values, "clean_lines": total_lines - len(labels),
        "precision_lines": len(caught) / (len(caught) + len(alarms)) if caught or alarms else 0.0,
        "precision_values": real_values / (real_values + alarm_values) if real_values or alarm_values else 0.0,
        "alarm_types": Counter(t for v in alarms.values() for t in v).most_common(8),
        "by_detector": {d: round(c / n, 3) for d, (c, n) in sorted(by_det.items())},
    }


def compare_lines(name: str, keys: list, lines: list[str], labels: dict, binary: Path, folder: Path, root_rel) -> dict:
    """Runs every tool over the same lines. keys[i] is the label key of lines[i]; root_rel maps a pii-eu
    finding to its key."""
    out = {}
    findings, t = run_piieu(binary, folder)
    flagged = defaultdict(list)
    for f in findings:
        if f.get("kind", "value") == "value":
            flagged[root_rel(f)].append(f["detector"])
    out["pii-eu"] = {**score(labels, len(lines), flagged), "seconds": round(t, 2)}
    for tool, runner in (("presidio", run_presidio), ("scrubadub", run_scrubadub)):
        if tool not in TOOLS:
            continue
        hits, t = runner(lines)
        flagged = defaultdict(list)
        for i, typ in hits:
            flagged[keys[i]].append(typ)
        out[tool] = {**score(labels, len(lines), flagged), "seconds": round(t, 2)}
        print(f"  {name} {tool}: {out[tool]['caught']}/{out[tool]['values']} caught, "
              f"{out[tool]['alarm_lines']} alarm lines, {t:.1f} s", file=sys.stderr)
    print(f"  {name} pii-eu: {out['pii-eu']['caught']}/{out['pii-eu']['values']} caught, "
          f"{out['pii-eu']['alarm_lines']} alarm lines", file=sys.stderr)
    return out


# ---------------------------------------------------------------- corpora

def values_synthetic(binary: Path, seed: int, work: Path) -> dict:
    folder = work / f"synthetic-{seed}"
    shutil.rmtree(folder, ignore_errors=True)
    subprocess.run(["go", "run", "./bench", "-lines", "30000", "-seed", str(seed), "-export", str(folder)],
                   cwd=REPO, check=True, capture_output=True)
    lines = (folder / "corpus.txt").read_text().splitlines()
    labels = {int(n): d for n, d in (r.split("\t") for r in (folder / "labels.tsv").read_text().splitlines())}
    (folder / "labels.tsv").rename(work / f"labels-{seed}.tsv")
    keys = list(range(1, len(lines) + 1))
    return compare_lines(f"synthetic seed {seed}", keys, lines, labels, binary, folder, lambda f: f["line"])


def generated_values(seed: int, n: int) -> list[dict]:
    out = subprocess.run(["go", "run", "./bench/compare/values", "-detectors", ",".join(SEEDED), "-n", str(n),
                          "-seed", str(seed)], cwd=REPO, check=True, capture_output=True, text=True).stdout
    return [json.loads(l) for l in out.splitlines()]


def seeded_line(ext: str, key: str, value: str) -> str:
    return {
        ".json": f'    "{key}": "{value}",',
        ".yml": f'  {key}: "{value}"', ".yaml": f'  {key}: "{value}"',
        ".py": f'    {key} = "{value}"',
        ".rb": f'    {key}: "{value}",',
        ".php": f"            '{key}' => '{value}',",
        ".js": f'  {key}: "{value}",', ".ts": f'  {key}: "{value}",',
        ".sql": f"INSERT INTO people ({key}) VALUES ('{value}');",
        ".log": f"2026-10-29T{random.randint(0, 23):02d}:{random.randint(0, 59):02d}:07.412Z INFO checkout {key}={value}",
    }[ext]


def app_log(r: random.Random, n: int) -> list[str]:
    """An application log of the kind real services write: request IDs, millisecond timestamps, durations,
    numeric user and order IDs."""
    out = []
    for _ in range(n):
        ms = 1_700_000_000_000 + r.randrange(100_000_000_000)
        out.append(f"2026-10-{r.randint(1, 29):02d}T{r.randint(0, 23):02d}:{r.randint(0, 59):02d}:{r.randint(0, 59):02d}."
                   f"{r.randint(0, 999):03d}Z {r.choice(['INFO', 'WARN', 'DEBUG'])} {r.choice(['checkout', 'cart', 'auth', 'search'])} "
                   f"request_id={r.getrandbits(64):016x} ts={ms} duration_ms={r.randint(1, 4000)} "
                   f"user_id={r.randint(1, 9_999_999)} order_id={r.randint(10**12, 10**13 - 1)} status={r.choice([200, 201, 204, 400, 404, 500])}")
    return out


def values_oss(binary: Path, oss: Path, seed: int, work: Path) -> tuple[dict, dict]:
    r = random.Random(seed)
    random.seed(seed)
    files = {}
    for repo in sorted(p for p in oss.iterdir() if p.is_dir()):
        candidates = [p for p in sorted(repo.rglob("*")) if p.is_file() and p.suffix in OSS_EXT
                      and OSS_DIRS.search(p.relative_to(oss).as_posix()) and ".git" not in p.parts and p.stat().st_size < 512_000]
        r.shuffle(candidates)
        taken = 0
        for p in candidates:
            if taken >= PER_REPO_LINES:
                break
            text = p.read_text(errors="ignore").splitlines()
            if text and max(len(l) for l in text) < 2000:
                files[p.relative_to(oss).as_posix()] = text
                taken += len(text)
    files["generated/logs/app.log"] = app_log(r, 20_000)
    meta = {"repos": {p.name: subprocess.run(["git", "-c", "core.fsmonitor=false", "-C", str(p), "rev-parse", "--short", "HEAD"],
                                             capture_output=True, text=True).stdout.strip() for p in oss.iterdir() if p.is_dir()},
            "files": len(files), "original_lines": sum(len(v) for v in files.values())}
    values = generated_values(seed, 200)
    r.shuffle(values)
    names = sorted(files)
    inserts = defaultdict(list)
    for v in values:
        rel = r.choice(names)
        key = r.choice(v["keywords"][:2] or NEUTRAL) if r.random() < 0.5 else r.choice(NEUTRAL)
        if v["detector"] == "email" and key not in NEUTRAL:
            key = "email"
        inserts[rel].append((r.randrange(len(files[rel]) + 1), key, v))
    folder = work / f"oss-{seed}"
    shutil.rmtree(folder, ignore_errors=True)
    keys, lines, labels = [], [], {}
    for rel in names:
        body = list(files[rel])
        ext = Path(rel).suffix
        for pos, key, v in sorted(inserts.get(rel, []), key=lambda x: -x[0]):
            body.insert(pos, seeded_line(ext, key, v["value"]) + "\x00" + v["detector"])
        clean = []
        for i, l in enumerate(body, 1):
            if "\x00" in l:
                l, det = l.split("\x00")
                labels[(rel, i)] = det
            clean.append(l)
            keys.append((rel, i))
            lines.append(l)
        dest = folder / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text("\n".join(clean) + "\n")
    meta["seeded_values"] = len(labels)
    result = compare_lines(f"oss seed {seed}", keys, lines, labels, binary, folder, lambda f: (f["path"], f["line"]))
    return result, meta


# Per language: file name, file template, personal field access, harmless field access, local assignment.
# A quarter of the personal cases go through a local variable first (v = user.email; log(v)), a case pii-eu's
# lexical check misses (#265); we keep it in so the test does not favour pii-eu.
SINK_LANGS = {
    "py": ("app/{n}.py", "import logging\nlog = logging.getLogger(__name__)\n\n\ndef handle(user, order):\n{pre}    log.info(\"done %s\", {expr})\n", "user.{f}", "order.{f}", "    v = {src}\n", "v"),
    "js": ("web/{n}.js", "export function handle(user, order) {{\n{pre}  console.log('done', {expr});\n}}\n", "user.{f}", "order.{f}", "  const v = {src};\n", "v"),
    "ts": ("web/{n}.ts", "import {{ logger }} from './logger';\nexport function handle(user: any, order: any) {{\n{pre}  logger.info('done', {expr});\n}}\n", "user.{f}", "order.{f}", "  const v = {src};\n", "v"),
    "java": ("src/H{n}.java", "import org.slf4j.Logger;\nimport org.slf4j.LoggerFactory;\npublic class H{n} {{\n  private static final Logger log = LoggerFactory.getLogger(H{n}.class);\n  void handle(User user, Order order) {{\n{pre}    log.info(\"done {{}}\", {expr});\n  }}\n}}\n", "user.get{F}()", "order.get{F}()", "    var v = {src};\n", "v"),
    "go": ("svc/h{n}.go", "package svc\n\nimport \"log\"\n\nfunc Handle{n}(user User, order Order) {{\n{pre}\tlog.Printf(\"done %v\", {expr})\n}}\n", "user.{F}", "order.{F}", "\tv := {src}\n", "v"),
    "php": ("php/H{n}.php", "<?php\nclass H{n} {{\n  public function handle($user, $order) {{\n{pre}    $this->logger->info('done', ['v' => {expr}]);\n  }}\n}}\n", "$user->{f}", "$order->{f}", "    $v = {src};\n", "$v"),
    "rb": ("rb/h{n}.rb", "class H{n}\n  def handle(user, order)\n{pre}    Rails.logger.info(\"done #{{{expr}}}\")\n  end\nend\n", "user.{f}", "order.{f}", "    v = {src}\n", "v"),
    "kt": ("kt/H{n}.kt", "import org.slf4j.LoggerFactory\nclass H{n} {{\n  private val log = LoggerFactory.getLogger(H{n}::class.java)\n  fun handle(user: User, order: Order) {{\n{pre}    log.info(\"done {{}}\", {expr})\n  }}\n}}\n", "user.{f}", "order.{f}", "    val v = {src}\n", "v"),
}
PERSONAL = ["email", "phone", "first_name", "last_name", "date_of_birth", "iban", "address", "national_id", "ssn", "passport_number"]
HARMLESS = ["total", "status", "currency", "created_at", "item_count", "sku", "version", "region"]


def camel(f: str) -> str:
    return "".join(p.capitalize() for p in f.split("_"))


def sinks(binary: Path, seed: int, work: Path) -> dict:
    r = random.Random(seed)
    folder = work / "sinks"
    shutil.rmtree(folder, ignore_errors=True)
    labels, n = {}, 0
    for lang, (path, tmpl, pos, neg, assign, var) in SINK_LANGS.items():
        for case in ["direct"] * 15 + ["indirect"] * 5 + ["harmless"] * 20:
            n += 1
            f = r.choice(HARMLESS if case == "harmless" else PERSONAL)
            src = (neg if case == "harmless" else pos).format(f=f, F=camel(f))
            pre, expr = (assign.format(src=src), var) if case == "indirect" else ("", src)
            rel = path.format(n=n)
            (folder / rel).parent.mkdir(parents=True, exist_ok=True)
            (folder / rel).write_text(tmpl.format(n=n, expr=expr, pre=pre))
            labels[rel] = (lang, case)
    out = {}
    findings, t = run_piieu(binary, folder)
    out["pii-eu"] = sink_score(labels, {f["path"] for f in findings if f.get("kind") == "sink"}, t)
    out["hounddog"] = hounddog(folder, labels)
    return out


def hounddog(folder: Path, labels: dict) -> dict:
    """HoundDog's Docker image needs git metadata naming a repository; it runs with networking off and its AI
    review turned off. A file counts as caught when HoundDog reports a dataflow from a data element into a sink."""
    git = ["git", "-c", "core.fsmonitor=false", "-c", "user.name=bench", "-c", "user.email=bench@example.invalid", "-C", str(folder)]
    for args in (["init", "-q"], ["add", "-A"], ["commit", "-qm", "corpus"], ["remote", "add", "origin", "https://github.com/example/sinks.git"]):
        subprocess.run(git + args, check=True, capture_output=True)
    out_dir = folder.parent / "hounddog-out"
    out_dir.mkdir(exist_ok=True)
    image = "hounddogai/hounddog:latest"
    start = time.perf_counter()
    run = subprocess.run(["docker", "run", "--rm", "--network", "none", "-v", f"{folder}:/data:ro", "-v", f"{out_dir}:/out",
                          "-w", "/data", image, "hounddog", "scan", "--no-ai-review", "--no-cache", "--no-tips",
                          "--severity", "all", "--output-format", "json", "--output-path", "/out/r.json", "/data"],
                         capture_output=True, text=True)
    elapsed = time.perf_counter() - start
    if not (out_dir / "r.json").exists():
        return {"error": (run.stderr or run.stdout)[-400:]}
    doc = json.loads((out_dir / "r.json").read_text())
    files = set()

    def walk(x):
        if isinstance(x, dict):
            if "rel_file_path" in x:
                files.add(x["rel_file_path"])
            for v in x.values():
                walk(v)
        elif isinstance(x, list):
            for v in x:
                walk(v)
    walk(doc.get("dataflows", []))
    res = sink_score(labels, files, elapsed)
    res["version"] = doc.get("hounddog_version")
    return res


def sink_score(labels: dict, hit: set, seconds) -> dict:
    by_lang = defaultdict(lambda: Counter())
    for rel, (lang, case) in labels.items():
        by_lang[lang][case] += 1
        if rel in hit:
            by_lang[lang][case + "_hit"] += 1
    tot = sum(by_lang.values(), Counter())
    return {"direct": f"{tot['direct_hit']}/{tot['direct']}", "indirect": f"{tot['indirect_hit']}/{tot['indirect']}",
            "false_alarms": f"{tot['harmless_hit']}/{tot['harmless']}",
            "by_language": {k: f"{v['direct_hit']}/{v['direct']} direct, {v['indirect_hit']}/{v['indirect']} indirect, "
                               f"{v['harmless_hit']}/{v['harmless']} false" for k, v in sorted(by_lang.items())},
            "seconds": None if seconds is None else round(seconds, 2)}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--pii-eu", required=True, type=Path)
    ap.add_argument("--oss", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--seeds", default="1,2,3")
    ap.add_argument("--tools", default=",".join(sorted(TOOLS)), help="comma-separated subset, e.g. pii-eu for a quick run")
    a = ap.parse_args()
    TOOLS.intersection_update(a.tools.split(","))
    a.out.mkdir(parents=True, exist_ok=True)
    work = a.out / "work"
    work.mkdir(exist_ok=True)
    seeds = [int(s) for s in a.seeds.split(",")]
    version = subprocess.run([str(a.pii_eu), "version"], capture_output=True, text=True).stdout.strip()
    versions = {"pii-eu": version}
    for pkg in ("presidio-analyzer", "spacy", "scrubadub"):
        try:
            versions[pkg] = md.version(pkg)
        except md.PackageNotFoundError:
            pass
    res = {"date": time.strftime("%Y-%m-%d"), "versions": versions, "synthetic": {}, "oss": {}}
    for s in seeds:
        res["synthetic"][s] = values_synthetic(a.pii_eu, s, work)
        res["oss"][s], res["oss_meta"] = values_oss(a.pii_eu, a.oss, s, work)
    if "hounddog" in TOOLS:
        res["sinks"] = sinks(a.pii_eu, seeds[0], work)
    (a.out / "results.json").write_text(json.dumps(res, indent=1, default=str))
    print(json.dumps({k: v for k, v in res.items() if k in ("versions", "sinks", "oss_meta")}, indent=1, default=str))


if __name__ == "__main__":
    main()
