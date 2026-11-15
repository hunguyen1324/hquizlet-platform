"""Convert local KanjiVG SVGs to lazy-loaded JSON shards. Derived data: CC BY-SA 3.0."""
import json
import re
import shutil
import sys
from pathlib import Path
import xml.etree.ElementTree as ET

source = Path(sys.argv[1])
target = Path(__file__).resolve().parents[1] / "public" / "kanji"
target.mkdir(parents=True, exist_ok=True)
shards = {}
def attr(node, name):
    return next((value for key, value in node.attrib.items() if key.endswith("}" + name)), None)
def group(node):
    return {"char": attr(node, "element"), "position": attr(node, "position"),
            "radical": bool(attr(node, "radical")), "phonetic": bool(attr(node, "phon")),
            "children": [group(child) for child in node if child.tag.endswith("}g")]}

count = 0
for file in sorted((source / "kanji").glob("*.svg")):
    if not re.fullmatch(r"[0-9a-f]{5,6}", file.stem):
        continue
    root = ET.parse(file).getroot()
    paths = next((g for g in root if "StrokePaths" in g.get("id", "")), None)
    numbers = next((g for g in root if "StrokeNumbers" in g.get("id", "")), None)
    if paths is None:
        continue
    strokes = [p.get("d") for p in paths.iter() if p.tag.endswith("}path") and p.get("d")]
    labels = []
    if numbers is not None:
        for node in numbers:
            matrix = re.search(r"matrix\(([^)]+)\)", node.get("transform", ""))
            if matrix:
                values = re.split(r"[\s,]+", matrix[1].strip())
                if len(values) == 6:
                    labels.append({"x": float(values[4]), "y": float(values[5]), "n": node.text})
    shards.setdefault(file.stem[:-2], {})[chr(int(file.stem, 16))] = {
        "strokes": strokes, "numbers": labels, "tree": group(paths)}
    count += 1
for key, data in shards.items():
    (target / (key + ".json")).write_text(json.dumps(data, ensure_ascii=False, separators=(",", ":")), encoding="utf-8")
shutil.copyfile(source / "COPYING", target / "LICENSE.txt")
(target / "README.txt").write_text("Derived from KanjiVG, Copyright Ulrich Apel and contributors.\nhttps://kanjivg.tagaini.net/\nData converted from SVG to JSON; licensed CC BY-SA 3.0.\nhttps://creativecommons.org/licenses/by-sa/3.0/\nSee LICENSE.txt.\n", encoding="utf-8")
print(f"Generated {count} characters in {len(shards)} shards")
