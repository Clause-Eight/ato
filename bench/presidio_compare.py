"""Score Microsoft Presidio on the pii-eu benchmark corpus, for the identifiers both tools cover (see PAIRS).

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
from presidio_analyzer import predefined_recognizers as rec

# Presidio recognizer, its keyword arguments, and the pii-eu detector it is scored against. Only identifiers
# both tools cover are compared; each recognizer is enabled for English text so it is not penalised for its
# language defaults.
EU = ("AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "ES", "FI", "FR", "GR", "HR", "HU", "IE", "IT", "LT", "LU",
      "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK")
PAIRS = [
    ("CreditCardRecognizer", {}, "card"),
    ("IbanRecognizer", {}, "iban"),
    ("EmailRecognizer", {}, "email"),
    ("PhoneRecognizer", {"supported_regions": EU}, "eu-phone"),
    ("PlPeselRecognizer", {"supported_language": "en"}, "pl-pesel"),
    ("DeTaxIdRecognizer", {"supported_language": "en"}, "de-tax-id"),
    ("DeIdCardRecognizer", {"supported_language": "en"}, "de-id-card"),
    ("DeSocialSecurityRecognizer", {"supported_language": "en"}, "de-pension"),
    ("EsNifRecognizer", {"supported_language": "en"}, "es-dni"),
    ("EsNieRecognizer", {"supported_language": "en"}, "es-nie"),
    ("FiPersonalIdentityCodeRecognizer", {"supported_language": "en"}, "fi-hetu"),
    ("ItFiscalCodeRecognizer", {"supported_language": "en"}, "it-fiscal-code"),
    ("SePersonnummerRecognizer", {"supported_language": "en"}, "se-personnummer"),
    ("UkNinoRecognizer", {"supported_language": "en"}, "gb-nino"),
    ("NhsRecognizer", {"supported_language": "en"}, "gb-nhs"),
    ("IpRecognizer", {}, "ip-address"),
    ("MacAddressRecognizer", {"supported_language": "en"}, "mac-address"),
    ("CryptoRecognizer", {}, "crypto-wallet"),
]


def recognizers():
    for name, kwargs, detector in PAIRS:
        recognizer = getattr(rec, name)(**kwargs)
        yield recognizer, detector


ENTITY_TO_DETECTOR = {}


def engine() -> AnalyzerEngine:
    nlp = NlpEngineProvider(nlp_configuration={
        "nlp_engine_name": "spacy",
        "models": [{"lang_code": "en", "model_name": "en_core_web_sm"}],
    }).create_engine()
    registry = RecognizerRegistry(supported_languages=["en"])
    for recognizer, detector in recognizers():
        registry.add_recognizer(recognizer)
        for entity in recognizer.supported_entities:
            ENTITY_TO_DETECTOR[entity] = detector
    return AnalyzerEngine(nlp_engine=nlp, registry=registry, supported_languages=["en"])


def main(folder: str) -> None:
    root = Path(folder)
    lines = (root / "corpus.txt").read_text().splitlines()
    labels = {}
    for row in (root / "labels.tsv").read_text().splitlines():
        n, det = row.split("\t")
        labels[int(n)] = det

    analyzer = engine()
    import importlib.metadata
    version = importlib.metadata.version("presidio-analyzer")
    found: dict[float, list[tuple[int, str]]] = {0.0: [], 0.5: []}
    start = time.perf_counter()
    for i, text in enumerate(lines, 1):
        for r in analyzer.analyze(text=text, language="en", entities=list(ENTITY_TO_DETECTOR)):
            det = ENTITY_TO_DETECTOR[r.entity_type]
            for threshold in found:
                if r.score >= threshold:
                    found[threshold].append((i, det))
    elapsed = time.perf_counter() - start

    print(f"presidio-analyzer {version}, {len(lines)} lines in {elapsed:.1f} s ({len(lines) / elapsed:.0f} lines/s, one core)\n")
    print("| Detector | Values | Recall (score ≥ 0) | Precision (score ≥ 0) | Recall (score ≥ 0.5) | Precision (score ≥ 0.5) |")
    print("|---|---|---|---|---|---|")
    for det in dict.fromkeys(ENTITY_TO_DETECTOR.values()):
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
