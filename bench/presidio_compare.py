"""Score Microsoft Presidio on the pii-eu benchmark corpus, for the identifiers both tools cover.

    go run ./bench -lines 30000 -export /tmp/pii-eu-corpus
    python -m venv .venv && .venv/bin/pip install presidio-analyzer && .venv/bin/python -m spacy download en_core_web_sm
    .venv/bin/python bench/presidio_compare.py /tmp/pii-eu-corpus

Scoring matches bench/main.go: a labelled line counts as found once when a matching entity lands on it;
every matching entity on a line labelled otherwise is a false positive.
"""

from __future__ import annotations

import sys
import time
from collections import defaultdict
from pathlib import Path

from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_analyzer.nlp_engine import NlpEngineProvider
from presidio_analyzer.predefined_recognizers import (
    CreditCardRecognizer,
    EmailRecognizer,
    IbanRecognizer,
    PhoneRecognizer,
    PlPeselRecognizer,
)

ENTITY_TO_DETECTOR = {
    "CREDIT_CARD": "card",
    "IBAN_CODE": "iban",
    "EMAIL_ADDRESS": "email",
    "PHONE_NUMBER": "eu-phone",
    "PL_PESEL": "pl-pesel",
}


def engine() -> AnalyzerEngine:
    nlp = NlpEngineProvider(nlp_configuration={
        "nlp_engine_name": "spacy",
        "models": [{"lang_code": "en", "model_name": "en_core_web_sm"}],
    }).create_engine()
    registry = RecognizerRegistry(supported_languages=["en"])
    for rec in (
        CreditCardRecognizer(),
        IbanRecognizer(),
        EmailRecognizer(),
        PhoneRecognizer(supported_regions=("RO", "BG", "HU", "PL")),
        PlPeselRecognizer(supported_language="en"),
    ):
        registry.add_recognizer(rec)
    return AnalyzerEngine(nlp_engine=nlp, registry=registry, supported_languages=["en"])


def main(folder: str) -> None:
    root = Path(folder)
    lines = (root / "corpus.txt").read_text().splitlines()
    labels = {}
    for row in (root / "labels.tsv").read_text().splitlines():
        n, det = row.split("\t")
        labels[int(n)] = det

    analyzer = engine()
    found: dict[float, list[tuple[int, str]]] = {0.0: [], 0.5: []}
    start = time.perf_counter()
    for i, text in enumerate(lines, 1):
        for r in analyzer.analyze(text=text, language="en", entities=list(ENTITY_TO_DETECTOR)):
            det = ENTITY_TO_DETECTOR[r.entity_type]
            for threshold in found:
                if r.score >= threshold:
                    found[threshold].append((i, det))
    elapsed = time.perf_counter() - start

    print(f"Presidio {analyzer.__class__.__module__.split('.')[0]}, {len(lines)} lines in {elapsed:.1f} s ({len(lines) / elapsed:.0f} lines/s, one core)\n")
    print("| Detector | Values | Recall (score ≥ 0) | Precision (score ≥ 0) | Recall (score ≥ 0.5) | Precision (score ≥ 0.5) |")
    print("|---|---|---|---|---|---|")
    for det in ENTITY_TO_DETECTOR.values():
        cells = []
        for threshold in found:
            tp_lines, fp = set(), 0
            for n, d in found[threshold]:
                if d != det:
                    continue
                if labels.get(n) == det:
                    tp_lines.add(n)
                else:
                    fp += 1
            expected = sum(1 for v in labels.values() if v == det)
            recall = len(tp_lines) / expected if expected else float("nan")
            precision = len(tp_lines) / (len(tp_lines) + fp) if tp_lines or fp else float("nan")
            cells += [f"{recall:.1%}", f"{precision:.1%}"]
        expected = sum(1 for v in labels.values() if v == det)
        print(f"| `{det}` | {expected} | " + " | ".join(cells) + " |")


if __name__ == "__main__":
    main(sys.argv[1])
