"""Independent consumer for the fictional restored component; stdlib only."""
import importlib.util
import json
import pathlib
import sys


def check(normalize):
    cases = [
        ([], []),
        (["  ", "\t"], []),
        (["  Blue ", "blue", "BLUE", " Amber", ""], ["amber", "blue"]),
        (["Straße", "STRASSE", " Ω ", "ω", "Ä", "ä"], ["strasse", "ä", "ω"]),
        (["a b", " A  B ", "a b"], ["a  b", "a b"]),
    ]
    for inputs, expected in cases:
        original = inputs[:]
        actual = normalize(inputs)
        assert actual == expected, (inputs, expected, actual)
        assert inputs == original, "input mutated"
        assert normalize(actual) == expected, "not idempotent"


if __name__ == "__main__":
    if sys.argv[1:] == ["--oracle-control"]:
        check(lambda values: sorted({v.strip().casefold() for v in values if v.strip()}))
        try:
            check(lambda values: sorted(v.strip() for v in values if v.strip()))
        except AssertionError:
            print("ORACLE_CONTROL_PASS correct accepted, original defect rejected")
        else:
            raise AssertionError("oracle accepted original defect")
    else:
        root = pathlib.Path(sys.argv[1]).resolve()
        spec = importlib.util.spec_from_file_location("restored_labels", root / "labels.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        check(module.normalize_labels)
        print(json.dumps({"result":"TASK_PASS", "cases":5, "inputPreserved":True, "idempotent":True}))
