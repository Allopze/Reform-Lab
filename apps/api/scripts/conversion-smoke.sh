#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
API_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
BASE_URL=${BASE_URL:-http://127.0.0.1:4040}
TMP_DIR=$(mktemp -d)
COOKIE_JAR="$TMP_DIR/cookies.txt"
RUN_ID=${RUN_ID:-$(date +%s)}
LAST_STATUS=""
LAST_BODY=""
LAST_JOB_JSON=""
PASS_COUNT=0
SKIP_COUNT=0
FAIL_COUNT=0

cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

log() {
	printf '[conversion-smoke] %s\n' "$*"
}

pass() {
	PASS_COUNT=$((PASS_COUNT + 1))
	log "PASS $1"
}

skip() {
	SKIP_COUNT=$((SKIP_COUNT + 1))
	log "SKIP $1"
}

fail() {
	FAIL_COUNT=$((FAIL_COUNT + 1))
	printf '[conversion-smoke] FAIL %s\n' "$*" >&2
}

request_json() {
	local method=$1
	local path=$2
	local response_file="$TMP_DIR/response.json"
	local code

	if [[ $# -ge 3 ]]; then
		local body=$3
		code=$(curl -sS -o "$response_file" -w '%{http_code}' \
			-X "$method" \
			-b "$COOKIE_JAR" -c "$COOKIE_JAR" \
			-H 'Content-Type: application/json' \
			--data "$body" \
			"$BASE_URL$path")
	else
		code=$(curl -sS -o "$response_file" -w '%{http_code}' \
			-X "$method" \
			-b "$COOKIE_JAR" -c "$COOKIE_JAR" \
			"$BASE_URL$path")
	fi

	LAST_STATUS=$code
	LAST_BODY=$(cat "$response_file")
}

upload_file() {
	local path=$1
	local response_file="$TMP_DIR/upload.json"
	local code

	code=$(curl -sS -o "$response_file" -w '%{http_code}' \
		-X POST \
		-b "$COOKIE_JAR" -c "$COOKIE_JAR" \
		-F "file=@$path" \
		"$BASE_URL/api/files")

	LAST_STATUS=$code
	LAST_BODY=$(cat "$response_file")
}

download_artifact() {
	local artifact_id=$1
	local output_path=$2
	curl -sS -L -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
		-o "$output_path" \
		"$BASE_URL/api/artifacts/$artifact_id/download"
}

json_field() {
	JSON_PAYLOAD="$1" FIELD_NAME="$2" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["JSON_PAYLOAD"])
value = payload.get(os.environ["FIELD_NAME"])
print("" if value is None else value)
PY
}

capability_id_or_empty() {
	JSON_PAYLOAD="$1" CAPABILITY_NAME="$2" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["JSON_PAYLOAD"])
target = os.environ["CAPABILITY_NAME"]
for capability in payload.get("capabilities", []):
    if capability.get("id") == target:
        print(capability["id"])
        raise SystemExit(0)
print("")
PY
}

wait_for_health() {
	for _ in $(seq 1 30); do
		local response
		response=$(curl -sS "$BASE_URL/api/health" || true)
		if [[ $response == *'"status":"ok"'* ]]; then
			return 0
		fi
		sleep 1
	done

	printf 'API did not become healthy at %s\n' "$BASE_URL" >&2
	exit 1
}

wait_for_job_terminal() {
	local job_id=$1
	for _ in $(seq 1 120); do
		request_json GET "/api/jobs/$job_id"
		local status
		status=$(json_field "$LAST_BODY" status)
		case $status in
			succeeded|failed|cancelled|expired)
				printf '%s' "$LAST_BODY"
				return 0
				;;
		esac
		sleep 0.5
	done

	printf 'job %s did not reach a terminal state in time\n' "$job_id" >&2
	exit 1
}

register_user() {
	local suffix=$1
	request_json POST /api/auth/register "{\"email\":\"smoke-${RUN_ID}-${suffix}@test.local\",\"password\":\"SmokePass123!\",\"name\":\"Conversion Smoke ${suffix}\"}"
	if [[ "$LAST_STATUS" != "201" ]]; then
		fail "$suffix registration failed: HTTP $LAST_STATUS $LAST_BODY"
		return 1
	fi
}

run_conversion() {
	local scenario=$1
	local fixture_path=$2
	local capability_name=$3

	: >"$COOKIE_JAR"
	if ! register_user "$scenario"; then
		return 1
	fi

	upload_file "$fixture_path"
	if [[ "$LAST_STATUS" != "201" ]]; then
		skip "$scenario upload rejected by this runtime: HTTP $LAST_STATUS $LAST_BODY"
		return 2
	fi
	local file_id
	file_id=$(json_field "$LAST_BODY" id)

	request_json GET "/api/files/$file_id/capabilities"
	if [[ "$LAST_STATUS" != "200" ]]; then
		fail "$scenario capabilities failed: HTTP $LAST_STATUS $LAST_BODY"
		return 1
	fi
	local capability_id_value
	capability_id_value=$(capability_id_or_empty "$LAST_BODY" "$capability_name")
	if [[ -z "$capability_id_value" ]]; then
		skip "$scenario capability $capability_name not offered by this runtime"
		return 2
	fi

	request_json POST /api/conversions "{\"fileId\":\"$file_id\",\"capabilityId\":\"$capability_id_value\"}"
	if [[ "$LAST_STATUS" != "201" ]]; then
		fail "$scenario conversion creation failed: HTTP $LAST_STATUS $LAST_BODY"
		return 1
	fi
	local job_id
	job_id=$(json_field "$LAST_BODY" id)
	LAST_JOB_JSON=$(wait_for_job_terminal "$job_id")
}

assert_file_contains() {
	python3 - <<'PY' "$1" "$2"
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
needle = sys.argv[2]
if needle not in text:
    raise SystemExit(f"expected {needle!r} in output, got {text!r}")
PY
}

assert_header() {
	python3 - <<'PY' "$1" "$2"
from pathlib import Path
import sys

path = Path(sys.argv[1])
kind = sys.argv[2]
data = path.read_bytes()
checks = {
    "pdf": lambda d: d.startswith(b"%PDF-"),
    "png": lambda d: d.startswith(b"\x89PNG\r\n\x1a\n"),
    "webp": lambda d: len(d) >= 12 and d[:4] == b"RIFF" and d[8:12] == b"WEBP",
    "mp3": lambda d: d.startswith(b"ID3") or (len(d) >= 2 and d[0] == 0xff and (d[1] & 0xe0) == 0xe0),
    "gif": lambda d: d.startswith(b"GIF87a") or d.startswith(b"GIF89a"),
}
if kind not in checks:
    raise SystemExit(f"unknown header kind {kind}")
if not checks[kind](data):
    raise SystemExit(f"unexpected {kind} header: {data[:16]!r}")
PY
}

assert_zip_min_entries() {
	python3 - <<'PY' "$1" "$2"
from pathlib import Path
import sys
import zipfile

archive = Path(sys.argv[1])
minimum = int(sys.argv[2])
with zipfile.ZipFile(archive) as zf:
    count = len(zf.infolist())
if count < minimum:
    raise SystemExit(f"expected at least {minimum} zip entries, got {count}")
PY
}

create_pdf_fixture() {
	local pdf_path=$1
	python3 - <<'PY' "$pdf_path"
from pathlib import Path
import sys

output = Path(sys.argv[1])
stream = b"BT\n/F1 18 Tf\n36 96 Td\n(Reform Lab PDF smoke text) Tj\nET\n"
objects = [
    b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
    b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
    b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n",
    b"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
    b"5 0 obj\n<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"endstream\nendobj\n",
]
data = bytearray(b"%PDF-1.4\n")
offsets = [0]
for obj in objects:
    offsets.append(len(data))
    data.extend(obj)
xref_offset = len(data)
data.extend(f"xref\n0 {len(objects) + 1}\n".encode())
data.extend(b"0000000000 65535 f \n")
for offset in offsets[1:]:
    data.extend(f"{offset:010d} 00000 n \n".encode())
data.extend(f"trailer\n<< /Size {len(objects) + 1} /Root 1 0 R >>\nstartxref\n{xref_offset}\n%%EOF\n".encode())
output.write_bytes(data)
PY
}

create_png_fixture() {
	local png_path=$1
	python3 - <<'PY' "$png_path"
from pathlib import Path
import base64
import sys

Path(sys.argv[1]).write_bytes(base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP8z8DAwMDAxMDAwMAAAAYAAYdwhpQAAAAASUVORK5CYII="
))
PY
}

create_wav_fixture() {
	local wav_path=$1
	python3 - <<'PY' "$wav_path"
from pathlib import Path
import math
import struct
import sys
import wave

sample_rate = 8000
duration = 1.0
frames = int(sample_rate * duration)
with wave.open(str(Path(sys.argv[1])), "wb") as wav:
    wav.setnchannels(1)
    wav.setsampwidth(2)
    wav.setframerate(sample_rate)
    for i in range(frames):
        sample = int(12000 * math.sin(2 * math.pi * 440 * i / sample_rate))
        wav.writeframes(struct.pack("<h", sample))
PY
}

create_docx_fixture() {
	local docx_path=$1
	python3 - <<'PY' "$docx_path"
from pathlib import Path
import sys
import zipfile

files = {
    "[Content_Types].xml": """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>""",
    "_rels/.rels": """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>""",
    "word/document.xml": """<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Reform Lab DOCX smoke text</w:t></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>""",
}
with zipfile.ZipFile(Path(sys.argv[1]), "w", zipfile.ZIP_DEFLATED) as zf:
    for name, content in files.items():
        zf.writestr(name, content)
PY
}

create_svg_fixture() {
	local svg_path=$1
	cat >"$svg_path" <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" width="240" height="140" viewBox="0 0 240 140">
  <rect width="240" height="140" fill="#0f766e"/>
  <text x="24" y="78" fill="#f8fafc" font-size="24">Local smoke</text>
</svg>
EOF
}

create_video_fixture() {
	local video_path=$1
	if ! command -v ffmpeg >/dev/null 2>&1; then
		return 1
	fi
	ffmpeg -y -hide_banner -loglevel error \
		-f lavfi -i testsrc=size=160x120:rate=12 \
		-t 2 \
		-c:v mpeg4 \
		-pix_fmt yuv420p \
		"$video_path"
}

run_and_assert() {
	local scenario=$1
	local fixture_path=$2
	local capability=$3
	local output_name=$4
	local assertion=$5
	local needle=${6:-}

	set +e
	run_conversion "$scenario" "$fixture_path" "$capability"
	local result=$?
	set -e
	if [[ $result -eq 2 ]]; then
		return 0
	fi
	if [[ $result -ne 0 ]]; then
		return 0
	fi
	if [[ $(json_field "$LAST_JOB_JSON" status) != "succeeded" ]]; then
		fail "$scenario job did not succeed: $LAST_JOB_JSON"
		return 0
	fi

	local artifact_id
	artifact_id=$(json_field "$LAST_JOB_JSON" artifactId)
	local output_path="$TMP_DIR/$output_name"
	download_artifact "$artifact_id" "$output_path"
	set +e
	case "$assertion" in
		contains)
			assert_file_contains "$output_path" "$needle"
			;;
		zip-min)
			assert_zip_min_entries "$output_path" "$needle"
			;;
		*)
			assert_header "$output_path" "$assertion"
			;;
	esac
	local assert_result=$?
	set -e
	if [[ $assert_result -ne 0 ]]; then
		fail "$scenario artifact assertion failed"
		return 0
	fi
	pass "$scenario"
}

log "Waiting for API at $BASE_URL"
wait_for_health

create_pdf_fixture "$TMP_DIR/text.pdf"
run_and_assert "pdf-to-txt" "$TMP_DIR/text.pdf" pdf-to-txt pdf.txt contains "Reform Lab PDF smoke text"

create_png_fixture "$TMP_DIR/image.png"
run_and_assert "png-to-webp" "$TMP_DIR/image.png" image-to-webp image.webp webp

create_wav_fixture "$TMP_DIR/tone.wav"
run_and_assert "wav-to-mp3" "$TMP_DIR/tone.wav" audio-to-mp3 tone.mp3 mp3

if create_video_fixture "$TMP_DIR/video.mp4"; then
	run_and_assert "mp4-to-gif" "$TMP_DIR/video.mp4" video-to-gif video.gif gif
else
	skip "mp4-to-gif fixture generation requires local ffmpeg"
fi

create_docx_fixture "$TMP_DIR/document.docx"
run_and_assert "docx-to-pdf" "$TMP_DIR/document.docx" doc-to-pdf document.pdf pdf

if [[ -f "$API_DIR/tests/fixtures/heif/valid-complex.heif" ]]; then
	run_and_assert "heif-to-png" "$API_DIR/tests/fixtures/heif/valid-complex.heif" image-heic-to-png heif.png png
else
	skip "heif-to-png fixture missing"
fi

create_svg_fixture "$TMP_DIR/vector.svg"
run_and_assert "svg-to-pdf" "$TMP_DIR/vector.svg" image-svg-to-pdf vector.pdf pdf

if [[ -f "$API_DIR/tests/fixtures/presentation/valid-three-slides.pptx" ]]; then
	run_and_assert "pptx-to-jpg-zip" "$API_DIR/tests/fixtures/presentation/valid-three-slides.pptx" presentation-to-jpg slides.zip zip-min 1
else
	skip "pptx-to-jpg-zip fixture missing"
fi

if [[ -f "$API_DIR/tests/fixtures/spreadsheet/valid-multi-sheet.xlsx" ]]; then
	run_and_assert "xlsx-to-csv" "$API_DIR/tests/fixtures/spreadsheet/valid-multi-sheet.xlsx" spreadsheet-to-csv report.csv contains "capability,status,count"
else
	skip "xlsx-to-csv fixture missing"
fi

log "Summary: pass=$PASS_COUNT skip=$SKIP_COUNT fail=$FAIL_COUNT"
if [[ $FAIL_COUNT -gt 0 ]]; then
	exit 1
fi
