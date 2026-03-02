#!/usr/bin/env python3
import json
import os
import pathlib
import subprocess


def run_json(cmd):
    return json.loads(subprocess.check_output(cmd, text=True))


def human_size(n):
    units = ["B", "KB", "MB", "GB", "TB"]
    v = float(n)
    for u in units:
        if v < 1024.0 or u == units[-1]:
            return f"{v:.1f} {u}" if u != "B" else f"{int(v)} {u}"
        v /= 1024.0
    return f"{n} B"


def write_badge(path, label, message, color):
    payload = {
        "schemaVersion": 1,
        "label": label,
        "message": message,
        "color": color,
    }
    path.write_text(json.dumps(payload), encoding="utf-8")


def main():
    image_repo = os.environ["IMAGE_REPO"]
    out_dir = pathlib.Path(os.environ.get("BADGE_OUTPUT_DIR", ".deploy/badges"))
    out_dir.mkdir(parents=True, exist_ok=True)

    try:
        index = run_json(["docker", "manifest", "inspect", f"{image_repo}:latest"])
        manifests = index.get("manifests", [])
        digests = {}
        for m in manifests:
            p = m.get("platform") or {}
            if p.get("os") == "linux" and p.get("architecture") in {"amd64", "arm64"}:
                digests[p["architecture"]] = m.get("digest")

        for arch in ("amd64", "arm64"):
            digest = digests.get(arch)
            if not digest:
                write_badge(out_dir / f"image-size-{arch}.json", f"image size {arch}", "unknown", "lightgrey")
                continue

            manifest = run_json(["docker", "manifest", "inspect", f"{image_repo}@{digest}"])
            total = sum(int(layer.get("size", 0)) for layer in manifest.get("layers", []))
            write_badge(out_dir / f"image-size-{arch}.json", f"image size {arch}", human_size(total), "blue")
    except Exception:
        for arch in ("amd64", "arm64"):
            write_badge(out_dir / f"image-size-{arch}.json", f"image size {arch}", "unknown", "lightgrey")


if __name__ == "__main__":
    main()

