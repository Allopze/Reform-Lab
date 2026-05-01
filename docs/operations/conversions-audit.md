# Auditoría de conversiones de archivos (Reform Lab)

Fecha: 2026-05-01  
Alcance: backend (apps/api), workers (apps/api/internal/workers), frontend (apps/web)  
Método: auditoría estática por lectura de código + revisión de tests existentes.

> Nota: este informe **no** ejecuta conversiones reales ni mide performance en runtime. Donde aplica, indico “no confirmado” y cómo verificarlo.

---

## Resumen ejecutivo

**Estado general**: el sistema está bien estructurado para conversión segura y mantenible. La “fuente de verdad” de capacidades existe en backend (catálogo + resolver), los workers ejecutan engines con **timeouts**, **reintentos**, **cancelación**, y hay validación de salida (incluye defensas contra ZIP traversal y checks mínimos de OOXML). En frontend, la UI consume capacidades **desde backend** tras upload (no decide reglas de conversión).

**Fortalezas principales**
- **Fuente de verdad backend** para capacidades: catálogo declarativo + resolver de elegibilidad.
- **Defensa SSRF/prácticas seguras** al procesar HTML/SVG/Markdown (sanitización de referencias remotas + stripping de scripts).
- **Validación post-conversión** sólida para artefactos (MIME allowlist, ZIP traversal, OOXML structure, JSON/CSV/text).
- **Robustez operacional**: timeouts/retries, cancelación, cuotas de subida/conversión, limpieza y retención, control de espacio en disco.

**Riesgos / oportunidades más relevantes**
- **Acoplamiento despliegue API↔worker**: la disponibilidad de engines se decide en el proceso que resuelve capabilities; en un despliegue “API liviana + workers pesados”, esto puede ocultar/mostrar capacidades de forma incorrecta.
- **UI pre-upload infiere categoría por MIME/extensión** (solo para UX). No afecta la conversión, pero puede generar expectativas erróneas y contradice parcialmente la regla “no confiar en extensión” (aunque aquí es solo “hint”).

**Hallazgos críticos**
- No se identificaron hallazgos de severidad “crítica” en lectura estática.

---

## Pasada de fixes aplicada (2026-05-01)

1) **UX para lotes sin capacidades**
- Estado: aplicado.
- Cambio:
  - `apps/web/src/components/conversion-card.tsx` ahora muestra un mensaje explícito cuando el backend devuelve `capabilities: []` para un lote ya subido.
  - `apps/web/messages/es.json` cambia el texto de “este archivo” a “este lote” para no prometer que el problema es individual.
- Tests:
  - `apps/web/src/components/conversion-card.test.tsx` cubre que no se renderiza selector, que el botón queda deshabilitado y que el mensaje aparece.
- Nota de frontera:
  - La UI no decide capacidades; solo explica el resultado devuelto por backend.

2) **UX para batches con familias detectadas mixtas**
- Estado: aplicado.
- Cambio:
  - `apps/web/src/components/conversion-card.tsx` calcula familias desde `detectedFamily` devuelto por la API tras ingestión.
  - Si el batch tiene más de una familia detectada y no hay capabilities comunes, muestra: “selecciona archivos del mismo tipo”.
- Tests:
  - `apps/web/src/components/conversion-card.test.tsx` cubre PDF + imagen con `capabilities: []`.
- Nota de seguridad:
  - No se usa extensión ni MIME del navegador; el mensaje se basa en `DetectedFormat.family` del backend.

3) **Contract test para intersección de capabilities**
- Estado: aplicado.
- Cambio:
  - Nuevo test en `apps/api/internal/api/handlers/capabilities_test.go`.
  - Cubre que la intersección conserva solo capabilities comunes y que familias mixtas devuelven lista vacía.
- Tests:
  - El test fuerza `PATH` vacío para no depender de binarios locales y dejar disponibles solo engines puros (`go-image`, etc.).

4) **Contrato operativo API/worker para engines**
- Estado: documentado.
- Cambio:
  - `docs/operations/runbooks.md` documenta que la API resuelve disponibilidad de engines en su propio proceso y que API/worker deben compartir set efectivo de binarios.
- Alcance:
  - No cambia arquitectura. El modelo reportado por workers queda como siguiente paso si se separan runtimes livianos/pesados.

5) **Smoke Docker ampliado por familia crítica**
- Estado: aplicado en script; ejecución completa pendiente en entorno Docker/CI.
- Cambio:
  - `apps/api/scripts/docker-e2e-smoke.sh` añade escenarios `PDF -> TXT`, `PNG -> WebP`, `WAV -> MP3`, `MP4 -> GIF` y `DOCX -> PDF`.
  - El script mantiene los escenarios existentes `HEIF -> PNG`, `SVG -> PDF`, `PPTX -> JPG ZIP` y `XLSX -> CSV`.
  - Los fixtures simples de PDF, PNG, WAV, MP4 y DOCX se generan en runtime para no añadir binarios pesados al repo.
  - Se añadieron validadores mínimos de salida para WebP, MP3 y GIF, además de las validaciones existentes para PDF, PNG, ZIP y contenido textual.
- Tests / verificación:
  - `bash -n apps/api/scripts/docker-e2e-smoke.sh` pasa.
  - No se ejecutó el smoke completo en esta pasada para no bajar y recrear volúmenes del stack Docker local desde una tarea de edición.
- Nota operativa:
  - El propio script ejecuta `docker compose down -v` durante cleanup; conviene correrlo de forma deliberada en CI o en un entorno local preparado para perder ese volumen temporal.

---

## Stack y arquitectura (observado en repo)

### Componentes
- **Frontend**: Next.js (apps/web). UX principal: upload → capabilities → batch conversion → polling jobs → download artifact.
- **API/Backend**: Go + chi router (apps/api). Expone endpoints públicos de subida, capabilities, conversion, jobs y artifacts.
- **Jobs/Workers**: 
  - Redis + Asynq si existe `REDIS_URL`.
  - Fallback in-process (dev) con límites de concurrencia.
- **DB**: SQLite + migraciones.
- **Storage**: filesystem local (`originals/`, `artifacts/`, `temp/`) + retención/limpieza + mínimo espacio libre (500MB).

### Engines y dependencias del runtime
El catálogo declara un `Engine` lógico; la disponibilidad se determina por binarios en `$PATH`.
- Poppler: `pdftoppm`, `pdftotext`, `pdftohtml`
- Ghostscript: `gs`
- LibreOffice: `libreoffice` (+ calc/impress/writer)
- OCR: `tesseract` (incluye idioma `spa` en imágenes Docker)
- Audio/Video/Imagen avanzada: `ffmpeg`
- HEIF: `heif-convert` (+ ffmpeg para webp)
- SVG→PDF vector: `rsvg-convert`
- PDF→DOCX: `pdf2docx` (Python venv)

Evidencia de empaquetado de binarios: apps/api/Dockerfile y apps/api/Dockerfile.worker.

---

## Flujo end-to-end (paso a paso)

1) **Upload** (frontend → `POST /api/files`)
- UI arma `FormData` y sube con timeout extendido.
- API aplica rate limits/cuotas, guarda original en storage y persiste registro.
- Se detecta formato real (MIME/familia/extensión) y se extraen metadatos/validaciones.

2) **Resolución de capacidades**
- UI llama `POST /api/files/capabilities/batch` con todos los fileIds seleccionados.
- Backend calcula **intersección** de capabilities disponibles para todos los archivos del batch.
- El resolver filtra por: feature flags, source MIME, tamaño, protegido, engine disponible, y “no same-format” solo para `convert`.

3) **Crear conversión / jobs**
- UI llama `POST /api/conversions/batch` con `{ fileIds, capabilityId }`.
- Orchestrator crea jobs con `timeoutSeconds` y `maxRetries` desde el catálogo.
- Encola en Asynq o ejecuta in-process.

4) **Ejecución de worker**
- Worker obtiene archivo original desde storage, crea temp dir, ejecuta engine.
- Reporta progreso (granularidad depende del handler; típicamente por fases).
- Valida output (por formato esperado). Si falla: job “failed” con error.

5) **Persistir artefacto**
- Se guarda el artefacto en storage con nombre y MIME detectados.
- Se guarda metadata en DB y se marca expiración.

6) **Polling + descarga**
- UI hace polling `GET /api/jobs/{jobId}` hasta estado final.
- Si `succeeded`, habilita `GET /api/artifacts/{artifactId}/download`.

---

## Matriz de formatos y capacidades (backend)

Fuente: `apps/api/internal/capabilities/catalog.go` + `apps/api/internal/capabilities/resolver.go` + `apps/api/internal/workers/registry.go`.

Convenciones:
- **SourceFormats**: MIME types aceptados para esa capacidad (según formato detectado real).
- **TargetFormat**: formato esperado de salida (el artefacto real puede ser ZIP en outputs multi-archivo).
- **Límites**: `MaxInputBytes`, `TimeoutSeconds`, `MaxRetries`.

### PDF

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| pdf-to-jpg | application/pdf | convert | jpg | poppler | 100 | 60 | 1 | Multipágina ⇒ ZIP de imágenes |
| pdf-to-png | application/pdf | convert | png | poppler | 100 | 60 | 1 | Multipágina ⇒ ZIP de imágenes |
| pdf-to-docx | application/pdf | convert | docx | pdf2docx | 100 | 180 | 1 | Layout complejo/escaneados pierden fidelidad |
| pdf-to-txt | application/pdf | extract | txt | poppler | 100 | 30 | 1 | PDF solo imágenes puede devolver vacío |
| pdf-compress | application/pdf | compress | pdf | ghostscript | 100 | 120 | 1 | Puede degradar calidad en scans |
| pdf-to-html-preview | application/pdf | preview | html | poppler-html | 100 | 90 | 1 | Sanitiza HTML de salida |
| pdf-ocr-to-txt | application/pdf | extract | txt | ocr-pdf | 100 | 240 | 1 | Precisión depende de calidad/idioma/contraste |
| pdf-ocr-to-json | application/pdf | extract | json | ocr-pdf | 100 | 240 | 1 | JSON deriva de TSV OCR (bloques/líneas) |
| pdf-ocr-searchable-pdf | application/pdf | optimize | pdf | ocr-pdf | 100 | 300 | 1 | Reconstruye capa searchable; puede crecer |

### Imágenes (raster)

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| image-to-png | image/jpeg, image/webp, image/gif, image/bmp, image/tiff | convert | png | go-image | 100 | 30 | 1 | Lossless |
| image-to-jpg | image/png, image/webp, image/gif, image/bmp, image/tiff | convert | jpg | go-image | 100 | 30 | 1 | Lossy; alpha se aplana a blanco |
| image-to-webp | image/jpeg, image/png | convert | webp | ffmpeg | 100 | 45 | 1 | Animados se “flatten” a 1 frame |
| image-to-avif | image/jpeg, image/png | convert | avif | ffmpeg | 100 | 45 | 1 | Path still-image; orientado a previews |
| image-to-pdf | image/jpeg, image/png, image/webp, image/gif, image/bmp, image/tiff | convert | pdf | go-image | 100 | 30 | 1 | PDF single-page |
| image-compress-jpg | image/jpeg | compress | jpg | go-image | 100 | 30 | 1 | Lossy |
| image-compress-png | image/png | compress | png | go-image | 100 | 30 | 1 | Ganancia variable; preserva píxeles |
| image-thumbnail-jpg | image/jpeg | preview | jpg | go-image | 100 | 20 | 1 | Max 320px edge |
| image-thumbnail-png | image/png | preview | png | go-image | 100 | 20 | 1 | Max 320px edge |
| image-ocr-to-txt | image/jpeg, image/png, image/webp, image/gif, image/bmp, image/tiff | extract | txt | tesseract | 100 | 120 | 1 | Depende de contraste/tamaño |
| image-ocr-to-json | image/jpeg, image/png, image/webp, image/gif, image/bmp, image/tiff | extract | json | tesseract | 100 | 120 | 1 | Bloques/líneas/palabras por TSV |
| image-web-jpg-640 | (idem raster) | optimize | jpg | ffmpeg | 100 | 45 | 1 | Cap 640px; animados 1 frame |
| image-web-webp-640 | (idem raster) | optimize | webp | ffmpeg | 100 | 45 | 1 | Cap 640px; animados 1 frame |
| image-web-avif-640 | (idem raster) | optimize | avif | ffmpeg | 100 | 45 | 1 | Cap 640px; animados 1 frame |
| image-web-jpg-1600 | (idem raster) | optimize | jpg | ffmpeg | 100 | 45 | 1 | Cap 1600px; animados 1 frame |
| image-web-webp-1600 | (idem raster) | optimize | webp | ffmpeg | 100 | 45 | 1 | Cap 1600px; animados 1 frame |
| image-web-avif-1600 | (idem raster) | optimize | avif | ffmpeg | 100 | 45 | 1 | Cap 1600px; animados 1 frame |

### Imágenes (HEIC/HEIF)

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| image-heic-to-jpg | image/heic, image/heif | convert | jpg | libheif | 100 | 45 | 1 | Decodifica still primario; metadata secuencia no se preserva |
| image-heic-to-png | image/heic, image/heif | convert | png | libheif | 100 | 45 | 1 | (idem) |
| image-heic-to-webp | image/heic, image/heif | convert | webp | libheif | 100 | 60 | 1 | Decode → PNG tmp → encode WebP |

### Imágenes (SVG)

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| image-svg-to-png | image/svg+xml | convert | png | ffmpeg | 100 | 45 | 1 | Sanitiza/extrae `<svg>`; rasteriza |
| image-svg-to-webp | image/svg+xml | convert | webp | ffmpeg | 100 | 60 | 1 | Sanitiza/extrae `<svg>`; rasteriza |
| image-svg-to-pdf | image/svg+xml | convert | pdf | librsvg | 100 | 60 | 1 | Export vectorial; depende de fonts/soporte librsvg |

### Documentos (office/text)

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| doc-to-pdf | docx, odt, rtf | convert | pdf | libreoffice | 100 | 120 | 1 | Formato complejo puede “shift” |
| doc-to-txt | docx, odt, rtf | extract | txt | libreoffice | 100 | 60 | 1 | Texto plano |
| doc-to-docx | odt, rtf | convert | docx | libreoffice | 100 | 120 | 1 | |
| doc-to-html | docx | convert | html | libreoffice | 100 | 90 | 1 | Sanitiza HTML de salida |
| docx-to-markdown | docx | convert | md | libreoffice | 100 | 90 | 1 | Simplifica layout |
| txt-to-pdf | text/plain | convert | pdf | libreoffice | 100 | 45 | 1 | Layout simple |
| html-to-pdf | text/html | convert | pdf | libreoffice | 100 | 90 | 1 | Sanitiza input HTML antes de LO |
| html-to-txt | text/html | extract | txt | go-html | 100 | 15 | 1 | Texto legible; descarta scripts/layout avanzado |
| markdown-to-html | text/markdown | convert | html | goldmark | 100 | 15 | 1 | Elegible solo si detección por contenido es confiable |
| markdown-to-pdf | text/markdown | convert | pdf | libreoffice | 100 | 60 | 1 | Render MD→HTML (sanitiza)→PDF |
| markdown-to-docx | text/markdown | convert | docx | libreoffice | 100 | 90 | 1 | Render MD→HTML (sanitiza)→DOCX |

### Presentaciones

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| presentation-to-pdf | pptx, odp | convert | pdf | libreoffice | 100 | 180 | 1 | Transiciones/video/fonts pueden aplanarse |
| presentation-to-jpg | pptx, odp | convert | jpg | libreoffice-poppler | 100 | 240 | 1 | 1 JPG por slide; puede ZIP |
| presentation-to-png | pptx, odp | convert | png | libreoffice-poppler | 100 | 240 | 1 | 1 PNG por slide; puede ZIP |

### Hojas de cálculo

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| spreadsheet-to-pdf | xlsx, ods, csv | convert | pdf | libreoffice | 100 | 180 | 1 | Ranges/escala según LO |
| spreadsheet-to-csv | xlsx, ods | convert | csv | libreoffice | 100 | 120 | 1 | Exporta sheet activo; fórmulas → valores |
| spreadsheet-to-xlsx | ods, csv | convert | xlsx | libreoffice | 100 | 120 | 1 | CSV→1 sheet; type inference LO |
| spreadsheet-to-html | xlsx, ods, csv | convert | html | libreoffice | 100 | 120 | 1 | Puede emitir assets adicionales |

### Audio

| Capability ID | Source MIME (resumen) | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| audio-to-mp3 | wav/ogg/opus/flac/aac/m4a | convert | mp3 | ffmpeg | 250 | 180 | 1 | |
| audio-to-wav | mp3/ogg/opus/flac/aac/m4a | convert | wav | ffmpeg | 250 | 180 | 1 | Lossless |
| audio-to-ogg | mp3/wav/opus/flac/aac/m4a | convert | ogg | ffmpeg | 250 | 180 | 1 | |
| audio-to-aac | mp3/wav/ogg/opus/flac/m4a | convert | aac | ffmpeg | 250 | 180 | 1 | ADTS |
| audio-to-m4a | mp3/wav/ogg/opus/flac/aac/m4a | convert | m4a | ffmpeg | 250 | 180 | 1 | AAC en contenedor M4A |
| audio-to-flac | mp3/wav/ogg/aac/m4a | convert | flac | ffmpeg | 250 | 180 | 1 | Lossless |
| audio-to-opus | mp3/wav/ogg/opus/flac/aac/m4a | convert | opus | ffmpeg | 250 | 180 | 1 | Ogg/Opus stream |
| audio-waveform-png | (idem) | preview | png | ffmpeg | 250 | 90 | 1 | Waveform estática |

### Video

| Capability ID | Source MIME | Op | Target | Engine | Max (MB) | Timeout (s) | Retries | Notas |
|---|---|---:|---:|---|---:|---:|---:|---|
| video-to-mp4 | quicktime, webm, avi | convert | mp4 | ffmpeg | 500 | 600 | 1 | |
| video-to-webm | mp4, quicktime, avi | convert | webm | ffmpeg | 500 | 600 | 1 | |
| video-to-gif | mp4, quicktime, webm, avi | convert | gif | ffmpeg | 500 | 300 | 1 | 30s / 480px / 10fps |
| video-to-mp3 | mp4, quicktime, webm, avi | extract | mp3 | ffmpeg | 500 | 600 | 1 | Audio principal |
| video-to-wav | mp4, quicktime, webm, avi | extract | wav | ffmpeg | 500 | 600 | 1 | Audio principal |
| video-to-aac | mp4, quicktime, webm, avi | extract | aac | ffmpeg | 500 | 600 | 1 | ADTS |
| video-to-m4a | mp4, quicktime, webm, avi | extract | m4a | ffmpeg | 500 | 600 | 1 | AAC en contenedor |
| video-to-flac | mp4, quicktime, webm, avi | extract | flac | ffmpeg | 500 | 600 | 1 | Audio principal |
| video-to-opus | mp4, quicktime, webm, avi | extract | opus | ffmpeg | 500 | 600 | 1 | Encode Opus |
| video-to-thumbnails | mp4, quicktime, webm, avi | preview | zip | ffmpeg | 500 | 180 | 1 | ZIP con hasta 6 JPG |
| video-contact-sheet | mp4, quicktime, webm, avi | preview | jpg | ffmpeg | 500 | 180 | 1 | 3x2 grid |
| video-preview-mp4 | mp4, quicktime, webm, avi | preview | mp4 | ffmpeg | 500 | 180 | 1 | 8s, rescaled |
| video-preview-webm | mp4, quicktime, webm, avi | preview | webm | ffmpeg | 500 | 180 | 1 | 8s, rescaled |
| video-waveform-png | mp4, quicktime, webm, avi | preview | png | ffmpeg | 500 | 120 | 1 | Falla si no hay audio |

---

## Formatos “declarados en UI” (frontend)

Fuente principal: `apps/web/src/config/categories.ts` + flujo dinámico desde backend.

### Qué es hardcode y qué no
- **No hardcodea conversiones**: tras upload, la UI renderiza opciones reales desde backend (`/api/files/capabilities/batch`).
- **Sí hay “hints” hardcodeados**:
  - `targetFormats` por categoría: solo informativo antes de subir.
  - `acceptedMimeTypes`: restringe el “accept” del picker/dropzone por categoría.
  - `detectCategoryIdFromFile(file)`: elige tab por `file.type` y extensión como heurística pre-upload.

Implicación: el producto respeta la regla “no confiar en extensiones” **para decisiones de conversión**, pero usa extensión/MIME como **UX hint**. Esto es razonable, pero puede causar expectativas inconsistentes.

---

## Hallazgos

Formato por hallazgo:
- **Severidad**: Crítica / Importante / Menor
- **Ubicación**: archivos/símbolos
- **Verificación**: cómo confirmarlo
- **Recomendación**: acción propuesta

### Críticos

- (Ninguno identificado en lectura estática)

### Importantes

1) **Acoplamiento de “engine availability” al proceso que resuelve capabilities**
- Severidad: Importante
- Ubicación:
  - `apps/api/internal/capabilities/engines.go` (`EngineProber`)
  - `apps/api/internal/capabilities/resolver.go` (`Resolve` / `IsEligible`)
  - `apps/api/Dockerfile` y `apps/api/Dockerfile.worker` (ambos incluyen binarios)
- Qué pasa:
  - La elegibilidad se filtra por `DefaultProber.IsAvailable(cap.Engine)`.
  - Si en el futuro API y worker no comparten el mismo set de binarios (o PATH), la API puede **ocultar** capacidades que el worker sí ejecuta, o **mostrar** capacidades que el worker no ejecuta.
- Verificación:
  - Ejecutar API sin binarios (p.ej. sin `ffmpeg`) pero worker con binarios; subir archivo y observar que la UI no ve la capability aunque worker podría.
- Recomendación:
  - Documentar explícitamente el contrato: “API y worker deben tener el mismo set de engines”, o
  - Cambiar el modelo: capabilities basadas en catálogo + flags, y “engine availability” vendría de un health/registry reportado por los workers (o config explícita por entorno).

2) **Batch capabilities = intersección: UX en mixed batches**
- Severidad: Importante
- Ubicación:
  - `apps/api/internal/api/handlers/capabilities.go` (`intersectCapabilities`)
  - `apps/web/src/components/hooks/use-upload.ts` (`getBatchCapabilities` + aplica 1 capability a todos)
- Qué pasa:
  - Si el usuario sube archivos de familias distintas, la intersección puede ser vacía y la UI queda sin opciones.
- Verificación:
  - Subir PDF + JPG en un mismo batch y confirmar que `capabilities` sea `[]`.
- Recomendación:
  - Mantenerlo (simple), pero mostrar un mensaje explícito: “Este batch mezcla tipos; selecciona archivos del mismo tipo” (idealmente basado en `detectedFamily`).

3) **“No same-format conversions” depende de `DetectedFormat.Extension`**
- Severidad: Importante (correctitud/eficiencia)
- Ubicación:
  - `apps/api/internal/capabilities/resolver.go` (`rejectsSameFormat`)
  - `apps/api/internal/ingestion/*` (donde se asigna extension detectada)
- Qué pasa:
  - Se bloquea convert cuando `cap.TargetFormat == file.DetectedFormat.Extension`.
  - Si la extensión detectada no está normalizada (ej. `jpeg` vs `jpg`), puede permitir conversiones “no-op” o bloquear incorrectamente.
- Verificación:
  - Asegurar que el detector normaliza extensiones canónicas (tests en ingestion ya cubren varios casos).
- Recomendación:
  - Normalizar extensiones detectadas a un set canónico (si aún no lo hace) o comparar por MIME/alias.

4) **Export HTML de LibreOffice puede generar assets companion que no se preservan**
- Severidad: Importante (correctitud de salida)
- Ubicación:
  - `apps/api/internal/workers/document/to_html.go` (`ToHTMLEngine`)
  - `apps/api/internal/workers/document/libreoffice.go` (`runLibreOfficeConvert`, `ensureOutputFile`)
- Qué pasa:
  - LibreOffice, al exportar a HTML, con frecuencia genera un archivo principal (`base.html`) y un directorio companion (`base_files/` o similar) para imágenes/CSS.
  - El engine actual solo valida/persiste `base.html` y sanitiza su contenido, pero **no empaqueta ni guarda** los assets companion.
  - Resultado probable: HTML descargado con referencias rotas (imágenes faltantes) en documentos no triviales.
- Verificación:
  - Convertir un `.docx`/`.xlsx` con imágenes a HTML y revisar si se crea un directorio companion en el temp dir del job.
  - Abrir el HTML descargado y confirmar si faltan recursos.
- Recomendación:
  - Si se detecta directorio companion, empaquetar `base.html` + companion dir como ZIP y entregar `artifactMimeType=application/zip`, o
  - Alternativa (más compleja): reescribir HTML para inline de assets (data URIs) tras sanitización.

### Menores

1) **UI pre-upload usa extensión/MIME para sugerir categoría**
- Severidad: Menor
- Ubicación: `apps/web/src/config/categories.ts` (`detectCategoryIdFromFile`)
- Impacto:
  - Es solo UX; la conversión real se decide por detección backend.
  - Puede sugerir una categoría errónea si el navegador no conoce el MIME o si la extensión es engañosa.
- Recomendación:
  - Favorecer siempre la categoría “Auto” (ya existe) o basar el switch solo en MIME cuando esté presente.

2) **Hints `acceptedMimeTypes` pueden “ocultar” archivos válidos**
- Severidad: Menor
- Ubicación: `apps/web/src/config/categories.ts`
- Impacto:
  - `accept` del file picker es una guía; algunos OS/navegadores asignan `file.type` vacío para ciertos formatos.
- Recomendación:
  - Asegurar que “Auto” sea la experiencia por defecto (o que el picker siempre permita `*/*`).

---

## Inconsistencias / diferencias (backend vs UI)

- Backend decide capabilities por **MIME detectado real**; UI muestra “target formats” por categoría como **hints**. Esto puede producir discrepancias en expectativas:
  - Ej.: en Video, la UI sugiere MP4 como output, pero `video-to-mp4` no aplica a `video/mp4` (solo a quicktime/webm/avi). Un MP4 subido no verá “Convertir a MP4” porque sería same-format.
- Recomendación: en textos/hints, usar wording tipo “Opciones típicas (según archivo)” para evitar promesas.

---

## Casos borde (cubierto / no cubierto / no confirmado)

| Caso borde | Estado | Evidencia / notas |
|---|---|---|
| Archivo demasiado grande (por capability) | Cubierto | Resolver filtra por `MaxInputBytes`; ingestion + tests en `apps/api/internal/ingestion/*_test.go` |
| PDFs protegidos/encriptados | Cubierto | Resolver excluye `Metadata.IsProtected`; validator/metadata en ingestion |
| Cancelación de job en ejecución | Cubierto | Handler de worker hace polling de cancel; tests en `apps/api/internal/workers/handler_cancel_test.go` |
| Output ZIP con traversal (`../`, abs, `\`) | Cubierto | `validateZipOutput` + tests en `apps/api/internal/workers/output_validation_test.go` |
| Output OOXML inválido (docx/xlsx) | Cubierto | `validateOOXMLOutput` |
| HTML/SVG con referencias remotas (SSRF) | Cubierto | Sanitización en `apps/api/internal/workers/document/html_sanitize.go` + tests |
| Video sin stream de audio para waveform | Cubierto | Cap tiene known limitation; engine puede fallar; UX mostrará error |
| FFmpeg/LibreOffice colgados o muy lentos | No confirmado | Hay `TimeoutSeconds`, pero falta ver comportamiento real bajo carga |
| Discos casi llenos | Cubierto | `minFreeDiskBytes` (500MB) bloquea save original/artifact |
| Output HTML con assets sidecar (LO) | Parcialmente cubierto | Sanitiza HTML, pero no preserva directorio companion; en docs con imágenes es probable que queden referencias rotas |

---

## Robustez (operacional)

- **Timeouts + retries** por capability: definidos en catálogo y aplicados por orchestrator/queue.
- **Cancelación**: worker revisa cancel cada ~500ms y aborta.
- **Cuotas**:
  - Upload policy expuesta a UI y validada en backend.
  - Rate limiting por usuario o IP.
- **Limpieza**:
  - Temp/original TTL + purga de DB.
  - Retención y purga de artifacts vencidos.
- **Validación de salida**: detecta corrupción, formato inesperado, ZIP traversal.

---

## UX (frontend)

Observado en `apps/web/src/components/conversion-card.tsx` + hooks:
- 1 dropzone por categoría + “Auto”.
- Tras upload, opciones de conversión = capabilities backend.
- Batch conversion: una capability común a todos (intersección).
- Progress: barra simple por polling de job.
- Descarga: detecta ZIP por `artifactMimeType` o `.zip` y ajusta label.

Oportunidades UX (sin cambiar reglas de negocio)
- Aplicado: mensaje explícito cuando `capabilities.length === 0`.
- Aplicado: mensaje específico si batch mezcla familias (`detectedFamily` distintos).

---

## Tests recomendados (incrementales)

Ya hay buena cobertura unitaria de engines y validación. Recomendaciones puntuales:

1) **Contract/API tests**
- `POST /api/files/capabilities/batch`:
  - Aplicado en test unitario de handler: mixed families ⇒ `capabilities: []`.
  - Aplicado en test unitario de handler: intersección estable (orden).

2) **E2E (docker) de conversión real**
- Aplicado en `apps/api/scripts/docker-e2e-smoke.sh`:
  - PDF→TXT, Imagen→WebP, Audio→MP3, Video→GIF, Doc→PDF.
- Pendiente:
  - Ejecutar el smoke completo en CI o en un entorno Docker local dedicado.

3) **Frontend e2e**
- Extender `apps/web/e2e/smoke.spec.ts` o el spec de batch conversion para verificar:
  - estado “sin capabilities” muestra mensaje.
  - descarga de ZIP vs archivo simple cambia label.

---

## Mejoras técnicas (sin cambiar UX)

- Aplicado: documentar el contrato “API y worker comparten engines”.
- Pendiente: añadir check explícito en health/admin si se quiere validar divergencia entre API y workers.
- Añadir métrica por capability: latencia, tasa de éxito, razón de fallo (ya hay base de observabilidad).
- Mejorar progresos en workers por fases (decode → convert → validate → persist) para feedback más consistente.

---

## Nuevas funcionalidades candidatas (tabla)

| Idea | Valor usuario | Esfuerzo | Riesgo | Dependencias |
|---|---|---:|---:|---|
| Audio/Video → transcripción TXT/JSON | Alto | Medio | Medio | Motor STT + costes |
| Video → subtítulos SRT/VTT | Alto | Medio | Medio | STT + alineación |
| PDF → dividir/extraer páginas | Medio | Bajo-Medio | Medio | Poppler/gs + UX |
| Imagen → remover EXIF/metadata | Medio | Bajo | Bajo | Go/ffmpeg |
| PDF → “optimize for web” (faststart, downsample) | Medio | Medio | Medio | Ghostscript tuning |

---

## Quick wins (tabla)

| Acción | Impacto | Esfuerzo | Ubicación sugerida |
|---|---|---:|---|
| Mensaje UX cuando no hay capabilities | Alto | Bajo | Aplicado en `apps/web/src/components/conversion-card.tsx` |
| Mensaje UX para batches mixtos | Medio | Bajo | Aplicado en `apps/web/src/components/conversion-card.tsx` |
| Documentar contrato engines API/worker | Medio | Bajo | Aplicado en `docs/operations/runbooks.md` |
| Añadir test contract de `intersectCapabilities` | Medio | Bajo | Aplicado en `apps/api/internal/api/handlers/capabilities_test.go` |
| Ampliar smoke docker por familia | Alto | Bajo-Medio | Aplicado en `apps/api/scripts/docker-e2e-smoke.sh` |

---

## Roadmap (3 fases)

### Fase 1 — Estabilización y claridad (1–2 semanas)
- Aplicado: UX con mensajes claros para “sin capabilities” y “batch mixto”.
- Aplicado: contrato de engines en runbook operativo.
- Aplicado: tests de intersección de batch capabilities.
- Pendiente: checklist operativo verificable en despliegue real.

### Fase 2 — Observabilidad y fiabilidad (2–4 semanas)
- Métricas por capability (p95 duración, error rate).
- Mejoras de progreso por fases.
- Aplicado en script: smoke docker por familia crítica.
- Pendiente: ejecución recurrente en CI o release gate.

### Fase 3 — Nuevas capacidades de alto valor (4–8 semanas)
- STT (audio/video) + subtítulos.
- PDF split/extract pages (si el producto lo necesita).

---

## Checklist accionable

- [x] Documentar contrato de despliegue: API y worker comparten binarios o se redefine el modelo de disponibilidad.
- [x] Añadir UX para `capabilities.length === 0`.
- [x] Añadir UX para batch con `detectedFamily` mixto.
- [x] Añadir contract tests de la intersección usada por `POST /api/files/capabilities/batch`.
- [x] Añadir un smoke docker por familia crítica.
- [ ] Ejecutar el smoke Docker ampliado en CI o entorno local dedicado.

## Pendiente tras esta pasada

- Verificar en un despliegue real que `/api/admin/engines` refleja el set esperado de engines en API y worker.
- Ejecutar el smoke Docker ampliado completo y, si tarda demasiado, decidir si se divide por perfiles (`core`, `media`, `office`).
- Decidir si el hallazgo de HTML LibreOffice con assets companion se resuelve empaquetando HTML + assets como ZIP o haciendo inline de recursos.
- Si se planea API liviana con workers pesados, reemplazar el probing local por configuración explícita o health reportado por workers.

---

## Conclusión

El sistema está **bien planteado**: capacidades declarativas, resolución en backend, ejecución asíncrona con límites, sanitización defensiva y validación fuerte de outputs. Las pasadas cerraron los quick wins de claridad UX, el test de intersección de capabilities, la documentación del contrato de engines y la ampliación del smoke Docker por familia crítica. Lo siguiente de mayor retorno es ejecutar ese smoke ampliado en CI/local dedicado y decidir el tratamiento de HTML LibreOffice con assets companion.
