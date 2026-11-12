#!/usr/bin/env python3
"""Upload to the App STT API, wait for completion, and save an SRT file."""
import argparse
import json
import os
from pathlib import Path
import re
import sys
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("audio", nargs="?")
    parser.add_argument("language", nargs="?", default="vi", choices=["vi", "en", "ja", "auto"])
    parser.add_argument("output", nargs="?")
    parser.add_argument("--job", help="Resume waiting for an existing job ID")
    parser.add_argument("--api", default="http://127.0.0.1:8091")
    args = parser.parse_args()
    token = os.environ.get("STT_API_TOKEN", "")
    if not token:
        config = Path("/etc/hquizlet-stt/service.env")
        if config.is_file():
            for line in config.read_text().splitlines():
                if line.startswith("STT_TOKEN="):
                    token = line.split("=", 1)[1]
    if not token:
        parser.error("Set STT_API_TOKEN or run on App with access to /etc/hquizlet-stt/service.env")

    def api(path, body=None, size=None):
        headers = {"Authorization": "Bearer " + token}
        if size is not None:
            headers.update({"Content-Type": "application/octet-stream", "Content-Length": str(size)})
        request = urllib.request.Request(args.api.rstrip("/") + path, data=body, headers=headers)
        with urllib.request.urlopen(request, timeout=300 if body is not None else 30) as response:
            data = response.read(4 * 1024 * 1024 + 1)
            if len(data) > 4 * 1024 * 1024:
                raise RuntimeError("API result exceeds size limit")
            return json.loads(data) if response.headers.get_content_type() == "application/json" else data

    if args.job:
        if not re.fullmatch(r"[a-f0-9]{32}", args.job):
            parser.error("Invalid job ID")
        identifier = args.job
        output = Path(args.output or (identifier + ".srt"))
    else:
        if not args.audio:
            parser.error("Provide an audio file or --job ID")
        audio = Path(args.audio).resolve(strict=True)
        if not 0 < audio.stat().st_size <= 64 * 1024 * 1024:
            parser.error("Audio must be between 1 byte and 64 MiB")
        output = Path(args.output) if args.output else audio.with_suffix(".srt")
        if output.resolve() == audio:
            parser.error("Output must differ from input audio")
        with audio.open("rb") as stream:
            job = api("/v1/transcriptions?language=" + args.language, stream, audio.stat().st_size)
        identifier = job["id"]
    print("Job:", identifier, flush=True)
    last_state = None
    while True:
        job = api("/v1/transcriptions/" + identifier)
        if job["status"] != last_state:
            print("Status:", job["status"], flush=True)
            last_state = job["status"]
        if job["status"] == "failed":
            raise RuntimeError(job.get("error", "Transcription failed"))
        if job["status"] == "completed":
            subtitle = api("/v1/transcriptions/" + identifier + "/srt")
            output.parent.mkdir(parents=True, exist_ok=True)
            temporary = output.with_name(output.name + "." + identifier + ".part")
            temporary.write_bytes(subtitle)
            temporary.replace(output)
            print("Saved:", output.resolve(), flush=True)
            return
        time.sleep(3)


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as error:
        print("API error:", error.code, error.read(4096).decode(errors="replace"), file=sys.stderr)
        sys.exit(1)
    except (OSError, RuntimeError) as error:
        print("Error:", str(error), file=sys.stderr)
        sys.exit(1)
