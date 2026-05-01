#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
API_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
BASE_URL=${BASE_URL:-http://127.0.0.1:8080}
JWT_SECRET=${JWT_SECRET:-reform-lab-local-dev-secret-change-me}
USER_UPLOADS_PER_MINUTE=${USER_UPLOADS_PER_MINUTE:-60}
USER_UPLOAD_BURST=${USER_UPLOAD_BURST:-10}
USER_CONVERSIONS_PER_MINUTE=${USER_CONVERSIONS_PER_MINUTE:-60}
USER_CONVERSION_BURST=${USER_CONVERSION_BURST:-10}
MAX_ACTIVE_JOBS_PER_USER=${MAX_ACTIVE_JOBS_PER_USER:-10}
TMP_DIR=$(mktemp -d)
COOKIE_JAR="$TMP_DIR/cookies.txt"
LAST_STATUS=""
LAST_BODY=""

cleanup() {
	(
		cd "$API_DIR"
		JWT_SECRET="$JWT_SECRET" docker compose -f docker-compose.yml down -v >/dev/null 2>&1 || true
	)
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

log() {
	printf '[smoke] %s\n' "$*"
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

assert_status() {
	local expected=$1
	if [[ "$LAST_STATUS" != "$expected" ]]; then
		printf 'expected HTTP %s, got %s: %s\n' "$expected" "$LAST_STATUS" "$LAST_BODY" >&2
		exit 1
	fi
}

json_field() {
	JSON_PAYLOAD="$1" FIELD_NAME="$2" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ['JSON_PAYLOAD'])
value = payload.get(os.environ['FIELD_NAME'])
print('' if value is None else value)
PY
}

capability_id() {
	JSON_PAYLOAD="$1" CAPABILITY_NAME="$2" python3 - <<'PY'
import json
import os
import sys

payload = json.loads(os.environ['JSON_PAYLOAD'])
target = os.environ['CAPABILITY_NAME']
for capability in payload.get('capabilities', []):
    if capability.get('id') == target:
        print(capability['id'])
        sys.exit(0)

raise SystemExit(f'capability {target} not found in {payload!r}')
PY
}

wait_for_health() {
	for _ in $(seq 1 90); do
		local response
		response=$(curl -sS "$BASE_URL/api/health" || true)
		if [[ $response == *'"status":"ok"'* ]]; then
			return 0
		fi
		python3 - <<'PY'
import time
time.sleep(1)
PY
	done

	printf 'API did not become healthy in time\n' >&2
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
		python3 - <<'PY'
import time
time.sleep(0.5)
PY
	done

	printf 'job %s did not reach a terminal state in time\n' "$job_id" >&2
	exit 1
}

register_user() {
	local suffix=$1
	request_json POST /api/auth/register "{\"email\":\"smoke-${suffix}@test.local\",\"password\":\"SmokePass123!\",\"name\":\"Docker Smoke ${suffix}\"}"
	assert_status 201
}

run_conversion() {
	local fixture_path=$1
	local capability_name=$2

	upload_file "$fixture_path"
	assert_status 201
	local file_id
	file_id=$(json_field "$LAST_BODY" id)

	request_json GET "/api/files/$file_id/capabilities"
	assert_status 200
	local capability_id_value
	capability_id_value=$(capability_id "$LAST_BODY" "$capability_name")

	request_json POST /api/conversions "{\"fileId\":\"$file_id\",\"capabilityId\":\"$capability_id_value\"}"
	assert_status 201
	local job_id
	job_id=$(json_field "$LAST_BODY" id)
	wait_for_job_terminal "$job_id"
}

assert_png() {
	python3 - <<'PY' "$1"
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_bytes()
expected = b'\x89PNG\r\n\x1a\n'
if not data.startswith(expected):
    raise SystemExit(f'unexpected PNG header: {data[:12]!r}')
PY
}

assert_pdf() {
	python3 - <<'PY' "$1"
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_bytes()
if not data.startswith(b'%PDF-'):
    raise SystemExit(f'unexpected PDF header: {data[:12]!r}')
PY
}

assert_webp() {
	python3 - <<'PY' "$1"
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_bytes()
if len(data) < 12 or data[:4] != b'RIFF' or data[8:12] != b'WEBP':
    raise SystemExit(f'unexpected WebP header: {data[:16]!r}')
PY
}

assert_mp3() {
	python3 - <<'PY' "$1"
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_bytes()
if not (data.startswith(b'ID3') or (len(data) >= 2 and data[0] == 0xff and (data[1] & 0xe0) == 0xe0)):
    raise SystemExit(f'unexpected MP3 header: {data[:16]!r}')
PY
}

assert_gif() {
	python3 - <<'PY' "$1"
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_bytes()
if not (data.startswith(b'GIF87a') or data.startswith(b'GIF89a')):
    raise SystemExit(f'unexpected GIF header: {data[:12]!r}')
PY
}

assert_zip_entries() {
	python3 - <<'PY' "$1" "$2"
from pathlib import Path
import sys
import zipfile

archive = Path(sys.argv[1])
expected = int(sys.argv[2])
with zipfile.ZipFile(archive) as zf:
    count = len(zf.infolist())
if count != expected:
    raise SystemExit(f'expected {expected} zip entries, got {count}')
PY
}

assert_file_contains() {
	python3 - <<'PY' "$1" "$2"
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding='utf-8')
needle = sys.argv[2]
if needle not in text:
    raise SystemExit(f'expected {needle!r} in output, got {text!r}')
PY
}

create_svg_fixture() {
	local svg_path=$1
	cat >"$svg_path" <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="240" height="140" viewBox="0 0 240 140">
  <rect width="240" height="140" fill="#0f766e" />
  <a href="https://example.com" xlink:href="https://example.com">
    <text x="24" y="78" fill="#f8fafc" font-size="24">Docker smoke</text>
  </a>
</svg>
EOF
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

create_video_fixture() {
	local video_path=$1
	local container_path="/tmp/reform-lab-smoke-video.mp4"
	(
		cd "$API_DIR"
		docker compose -f docker-compose.yml exec -T api \
			ffmpeg -y -hide_banner -loglevel error \
			-f lavfi -i testsrc=size=160x120:rate=12 \
			-t 2 \
			-c:v mpeg4 \
			-pix_fmt yuv420p \
			"$container_path"
		docker compose -f docker-compose.yml cp "api:$container_path" "$video_path" >/dev/null
	)
}

log 'Starting Docker Compose stack'
(
	cd "$API_DIR"
	JWT_SECRET="$JWT_SECRET" docker compose -f docker-compose.yml down -v >/dev/null 2>&1 || true
	JWT_SECRET="$JWT_SECRET" \
	USER_UPLOADS_PER_MINUTE="$USER_UPLOADS_PER_MINUTE" \
	USER_UPLOAD_BURST="$USER_UPLOAD_BURST" \
	USER_CONVERSIONS_PER_MINUTE="$USER_CONVERSIONS_PER_MINUTE" \
	USER_CONVERSION_BURST="$USER_CONVERSION_BURST" \
	MAX_ACTIVE_JOBS_PER_USER="$MAX_ACTIVE_JOBS_PER_USER" \
	docker compose -f docker-compose.yml up --build -d api
)

log 'Waiting for /api/health'
wait_for_health

log 'PDF -> TXT'
register_user pdf
create_pdf_fixture "$TMP_DIR/text.pdf"
job_json=$(run_conversion "$TMP_DIR/text.pdf" pdf-to-txt)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'PDF text extraction failed: %s\n' "$job_json" >&2
	exit 1
fi
pdf_txt_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$pdf_txt_artifact_id" "$TMP_DIR/pdf.txt"
assert_file_contains "$TMP_DIR/pdf.txt" 'Reform Lab PDF smoke text'

log 'PNG -> WebP'
register_user png
create_png_fixture "$TMP_DIR/image.png"
job_json=$(run_conversion "$TMP_DIR/image.png" image-to-webp)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'PNG to WebP conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
png_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$png_artifact_id" "$TMP_DIR/image.webp"
assert_webp "$TMP_DIR/image.webp"

log 'WAV -> MP3'
register_user wav
create_wav_fixture "$TMP_DIR/tone.wav"
job_json=$(run_conversion "$TMP_DIR/tone.wav" audio-to-mp3)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'WAV to MP3 conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
audio_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$audio_artifact_id" "$TMP_DIR/tone.mp3"
assert_mp3 "$TMP_DIR/tone.mp3"

log 'MP4 -> GIF'
register_user mp4
create_video_fixture "$TMP_DIR/video.mp4"
job_json=$(run_conversion "$TMP_DIR/video.mp4" video-to-gif)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'MP4 to GIF conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
video_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$video_artifact_id" "$TMP_DIR/video.gif"
assert_gif "$TMP_DIR/video.gif"

log 'DOCX -> PDF'
register_user docx
create_docx_fixture "$TMP_DIR/document.docx"
job_json=$(run_conversion "$TMP_DIR/document.docx" doc-to-pdf)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'DOCX to PDF conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
docx_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$docx_artifact_id" "$TMP_DIR/document.pdf"
assert_pdf "$TMP_DIR/document.pdf"

log 'HEIF -> PNG'
register_user heif
job_json=$(run_conversion "$API_DIR/tests/fixtures/heif/valid-complex.heif" image-heic-to-png)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'HEIF conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
heif_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$heif_artifact_id" "$TMP_DIR/heif.png"
assert_png "$TMP_DIR/heif.png"

log 'SVG -> PDF'
register_user svg
create_svg_fixture "$TMP_DIR/vector.svg"
job_json=$(run_conversion "$TMP_DIR/vector.svg" image-svg-to-pdf)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'SVG conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
svg_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$svg_artifact_id" "$TMP_DIR/vector.pdf"
assert_pdf "$TMP_DIR/vector.pdf"
	pdfinfo -url "$TMP_DIR/vector.pdf" | grep -q 'https://example.com'

log 'PPTX -> JPG ZIP'
register_user pptx
job_json=$(run_conversion "$API_DIR/tests/fixtures/presentation/valid-three-slides.pptx" presentation-to-jpg)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'Presentation conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
if [[ $(json_field "$job_json" artifactFileName) != 'slides.zip' ]]; then
	printf 'Unexpected presentation artifact name: %s\n' "$job_json" >&2
	exit 1
fi
pptx_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$pptx_artifact_id" "$TMP_DIR/slides.zip"
assert_zip_entries "$TMP_DIR/slides.zip" 3

log 'XLSX -> CSV'
register_user xlsx
job_json=$(run_conversion "$API_DIR/tests/fixtures/spreadsheet/valid-multi-sheet.xlsx" spreadsheet-to-csv)
if [[ $(json_field "$job_json" status) != 'succeeded' ]]; then
	printf 'Spreadsheet conversion failed: %s\n' "$job_json" >&2
	exit 1
fi
xlsx_artifact_id=$(json_field "$job_json" artifactId)
download_artifact "$xlsx_artifact_id" "$TMP_DIR/report.csv"
assert_file_contains "$TMP_DIR/report.csv" 'capability,status,count'
assert_file_contains "$TMP_DIR/report.csv" 'presentation-to-jpg'

log 'Docker E2E smoke passed'
