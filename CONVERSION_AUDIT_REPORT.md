# Auditoría de conversiones de archivos

## Resumen ejecutivo

El sistema de conversión de Reform Lab está sólidamente diseñado, con una arquitectura clara que respeta la separación entre detección, resolución de capacidades y ejecución de jobs. La fuente de verdad de capacidades (`catalog.go`) es exhaustiva (~90 capacidades en 5 familias) y el pipeline de validación (detección por contenido → validación de límites → metadatos → resolución) está bien implementado.

Se detectaron originalmente **2 hallazgos críticos** funcionales que impedían conversiones prometidas (HEIC >10MB bloqueados por el validador; .doc legacy sin capacidades), **11 hallazgos importantes** de robustez y UX, y **9 hallazgos menores** de consistencia o deuda técnica. Las pasadas 1-3 cierran los críticos y la mayoría de robustez inmediata sin cambiar la arquitectura base.

Riesgos principales ya mitigados: (1) imágenes HEIC/HEIF/SVG sin dimensiones verificables ahora tienen límite conservador explícito de 25MB y error claro; (2) archivos .doc legacy ya resuelven capacidades documentales; (3) documentos OOXML/ODF protegidos se detectan antes de la conversión cuando el contenedor expone metadatos de cifrado.

## Estado de implementación

### 2026-05-05 — Pasada 1: críticos y UX de descarga

Implementado:
- `application/msword` agregado a `doc-to-pdf`, `doc-to-txt` y `doc-to-docx`; la UI ahora lista DOC como formato de documento.
- El límite conservador para imágenes sin dimensiones verificables subió de 10MB a 25MB y devuelve un error específico cuando no puede verificar dimensiones de forma segura.
- Metadata detecta documentos OOXML/ODF protegidos cuando el contenedor expone `EncryptionInfo`, `EncryptedPackage` o `manifest:encryption-data`.
- `classifyError` devuelve un mensaje accionable para documentos protegidos por contraseña/encriptados.
- Descarga de artefactos incluye `Content-Length` usando el tamaño persistido del artifact.

Tests agregados/actualizados:
- Resolución de capacidades para `.doc` legacy.
- Validación de imágenes sin dimensiones hasta 25MB y rechazo específico por encima del límite.
- Detección de OOXML y ODF protegidos.
- Clasificación de error de documento protegido.
- E2E de descarga verifica `Content-Length`.

Pendiente tras esta pasada:
- Cuota acumulativa atómica en upload.
- Validación contextual del output menor al 1% del input.
- Anti ZIP-bomb en ZIPs de entrada.
- Validación estructural de PDF output.
- Sanitización defensiva del nombre de artifact emitido por engines.
- Métrica de duración de metadata extraction.
- Sincronización dinámica/manual completa de hints UI con el catálogo.

### 2026-05-05 — Pasada 2: robustez, observabilidad y cuota

Implementado:
- La cuota acumulativa de upload ahora se revalida de forma atómica en SQLite con `BEGIN IMMEDIATE` antes de insertar el registro del archivo.
- La validación de output PDF ahora exige MIME PDF, header `%PDF-` y marcador `%%EOF`.
- El check “output < 1% del input” se omite para operaciones `extract` y `compress`, evitando falsos rechazos de extracciones/compresiones legítimas.
- El detector OOXML rechaza ZIPs de entrada con demasiadas entradas, tamaño expandido acumulado excesivo o ratio de compresión extremo.
- La validación de ZIP output ahora inspecciona la primera entrada de preview y exige JPEG/PNG real.
- `outputArtifactFileName` sanitiza defensivamente el nombre que devuelve el engine antes de persistir el artifact.
- Se agregó la métrica `reform_metadata_extraction_duration_seconds` por familia de formato.

Tests agregados/actualizados:
- Repositorio: `CreateIfUnderQuota` rechaza exceso de cuota para usuario y guest.
- Detector: ZIP sospechoso por ratio extremo no se acepta como OOXML.
- Workers: PDF truncado se rechaza; extracción pequeña válida no falla por ratio; ZIP con imagen corrupta se rechaza; filename de artifact se sanitiza.

Pendiente tras esta pasada:
- Completar sincronización dinámica/manual de UI hints con el catálogo.
- Actualizar `docs/domain/capabilities-catalog.md` con el estado real.
- Revisar si conviene endpoint público `GET /api/catalog` u OpenAPI.
- Ampliar corpus real: `.doc`, protegidos reales, ZIP bombs controlados, corruptos por familia.

### 2026-05-05 — Pasada 3: catálogo público, docs y contrato de descarga

Implementado:
- Nuevo endpoint público `GET /api/catalog` que expone el catálogo backend agrupado por familia, con source formats, target, operación, límites y restricciones conocidas.
- Cliente frontend `getCatalog()` y tipos `CatalogFamily`/`CatalogCapability` para consumir el catálogo sin duplicar reglas.
- Test E2E de `GET /api/catalog` verificando que `.doc` legacy aparece en `doc-to-docx`.
- Test E2E de descarga con `artifact.file_name` malicioso (`../outside.txt`) para asegurar que no se lee contenido fuera del directorio del artifact.
- `docs/domain/capabilities-catalog.md` actualizado con `.doc` legacy y referencia al endpoint público de catálogo.

Pendiente tras esta pasada:
- Decidir si la pantalla inicial debe consumir `GET /api/catalog` en runtime o mantener hints estáticos como fallback visual.
- Ampliar corpus real: `.doc`, protegidos reales, ZIP bombs controlados, corruptos por familia.
- Valorar OpenAPI para compartir tipos completos entre API y frontend.

### 2026-05-05 — Pasada 4: UI híbrida con catálogo backend

Implementado:
- La pantalla inicial carga `GET /api/catalog` en runtime y usa el catálogo para hidratar `acceptedFormats`, `targetFormats` y `acceptedMimeTypes`.
- `categories.ts` conserva estructura visual, textos, iconos y fallback estático si el catálogo no está disponible.
- Se agregó `applyCatalogHints()` como función pura para evitar meter reglas de negocio en componentes.
- Tests frontend cubren hidratación desde catálogo, fallback sin catálogo y preservación de la categoría `auto`.

Pendiente tras esta pasada:
- Ampliar corpus real: `.doc`, protegidos reales, ZIP bombs controlados, corruptos por familia.
- Valorar OpenAPI para compartir tipos completos entre API y frontend.

### 2026-05-05 — Pasada 5: corpus real de fixtures críticos

Implementado:
- Se agregó corpus persistido en `apps/api/tests/fixtures` para `.doc` legacy, documentos protegidos OOXML/ODF, ZIP bomb controlado y corruptos por familia: PDF, documento, imagen, audio y video.
- La detección ahora queda cubierta por fixtures reales/persistidos: `.doc` se reconoce como `application/msword`, el ZIP bomb controlado se rechaza y los corruptos preservan la frontera de MIME esperada cuando aplica.
- Metadata valida fixtures protegidos desde disco para OOXML (`EncryptionInfo`/`EncryptedPackage`) y ODF (`manifest:encryption-data`).
- Workers documentales convierten el `.doc` legacy real a PDF y DOCX con LibreOffice.
- E2E cubre que un `.doc` real subido resuelve `doc-to-pdf`, `doc-to-docx` y `doc-to-txt`, que el ZIP bomb controlado se rechaza en upload y que un ODF protegido devuelve un error accionable.
- `apps/api/tests/fixtures/README.md` documenta cada fixture y su propósito.

Pendiente tras esta pasada:
- Valorar OpenAPI para compartir tipos completos entre API y frontend.
- Seguir ampliando corpus con muestras reales adicionales cuando aparezcan bugs específicos de engines (por ejemplo PDFs cifrados reales, DOC legacy con macros o multimedia grande).

## Stack y flujo detectado

**Backend**: Go 1.25, chi router, SQLite (WAL), Asynq (Redis) / in-process queue, zerolog, Prometheus, OTel.
**Frontend**: Next.js 15, React 19, Tailwind CSS 4, Vitest, Playwright, next-intl.
**Workers/Engines**: LibreOffice, Poppler (pdftoppm/pdftotext/pdftohtml), FFmpeg, Ghostscript, Tesseract, libheif, librsvg, pdf2docx + pure Go engines (go-image, goldmark, go-html).
**Storage**: Filesystem local (`originals/`, `artifacts/`, `temp/`).

```
Upload → Detect (mimetype + heurísticas) → Extract metadata → Validate → Save originals
  → Resolve capabilities (source match, engine probe, size, protected)
  → Create conversion request → Atomic job creation (DB + queue)
  → Worker picks engine → Execute with cancel-aware context
  → Validate output (MIME, size, structure) → Persist artifact → Notify (webhook/email)
  → Download (authorization by file ownership, expiration check)
```

## Matriz de formatos

| Entrada (MIME) | Salida | UI | Backend | Validado | Estado | Observaciones |
|---|---|---|---|---|---|---|
| application/pdf | jpg | Si | Si | Si | Funciona | Multi-página produce ZIP |
| application/pdf | png | Si | Si | Si | Funciona | Multi-página produce ZIP |
| application/pdf | docx | Si | Si | Si | Funciona | |
| application/pdf | txt | Si | Si | Si | Funciona | |
| application/pdf | pdf (compressed) | Si | Si | Si | Funciona | Ghostscript |
| application/pdf | html | Si | Si | Si | Funciona | Preview |
| application/pdf | txt (OCR) | No | Si | No confirmado | Parcial | No visible en UI estática |
| application/pdf | json (OCR) | Si | Si | No confirmado | Funciona | |
| application/pdf | pdf (searchable OCR) | No | Si | No confirmado | Parcial | No visible en UI estática |
| image/jpeg | png | Si | Si | Si | Funciona | |
| image/jpeg | webp | Si | Si | Si | Funciona | |
| image/jpeg | avif | Si | Si | Si | Funciona | |
| image/jpeg | pdf | Si | Si | Si | Funciona | |
| image/jpeg | jpg (compressed) | No | Si | No confirmado | Funciona | |
| image/jpeg | jpg (thumbnail) | No | Si | No confirmado | Funciona | Solo JPG source |
| image/jpeg | jpg (640px/1600px) | No | Si | No confirmado | Funciona | Web variants |
| image/png | jpg | Si | Si | Si | Funciona | |
| image/png | webp | Si | Si | Si | Funciona | |
| image/png | avif | Si | Si | Si | Funciona | |
| image/png | pdf | Si | Si | Si | Funciona | |
| image/png | png (compressed) | No | Si | No confirmado | Funciona | |
| image/png | png (thumbnail) | No | Si | No confirmado | Funciona | Solo PNG source |
| image/heic, image/heif | jpg | Si | Si | Si | Funciona | |
| image/heic, image/heif | png | Si | Si | Si | Funciona | |
| image/heic, image/heif | webp | Si | Si | Si | Funciona | |
| image/svg+xml | png | Si | Si | Si | Funciona | |
| image/svg+xml | webp | Si | Si | Si | Funciona | |
| image/svg+xml | pdf | Si | Si | No confirmado | Funciona | Requiere librsvg |
| image/webp | png/jpg/pdf | Si | Si | No confirmado | Funciona | |
| image/gif | png/jpg/pdf | Si | Si | No confirmado | Funciona | |
| image/bmp | png/jpg/pdf | Si | Si | No confirmado | Funciona | |
| image/tiff | png/jpg/pdf | Si | Si | No confirmado | Funciona | |
| DOCX/ODT/RTF | pdf | Si | Si | Si | Funciona | |
| DOCX/ODT/RTF | txt | Si | Si | No confirmado | Funciona | |
| ODT/RTF | docx | Si | Si | No confirmado | Funciona | |
| DOCX | html | Si | Si | No confirmado | Funciona | |
| DOCX | md | Si | Si | No confirmado | Funciona | LibreOffice path |
| text/plain | pdf | Si | Si | No confirmado | Funciona | |
| text/html | pdf | Si | Si | No confirmado | Funciona | |
| text/html | txt | Si | Si | Si | Funciona | Pure Go |
| text/markdown | html | Si | Si | Si | Funciona | Goldmark |
| text/markdown | pdf | Si | Si | No confirmado | Funciona | |
| text/markdown | docx | Si | Si | No confirmado | Funciona | |
| text/csv | pdf/xlsx/html | Si | Si | No confirmado | Funciona | |
| XLSX/ODS | pdf/csv/html | Si | Si | No confirmado | Funciona | |
| ODS/CSV | xlsx | Si | Si | No confirmado | Funciona | |
| PPTX/ODP | pdf/jpg/png | Si | Si | No confirmado | Funciona | ZIP multi-slide |
| application/msword | pdf/txt/docx | Si | Si | Si | Funciona | Cubierto con fixture DOC legacy real |
| Audio MP3/WAV/OGG/FLAC/AAC/M4A/Opus | cross-format | Si | Si | Si | Funciona | ffmpeg |
| Audio todos | waveform PNG | Si | Si | Si | Funciona | |
| Video MP4/MOV/WEBM/AVI | mp4/webm | Si | Si | Si | Funciona | ffmpeg |
| Video MP4/MOV/WEBM/AVI | gif | Si | Si | Si | Funciona | 30s, 480px, 10fps |
| Video MP4/MOV/WEBM/AVI | audio MP3/WAV/AAC/M4A/FLAC/Opus | Si | Si | Si | Funciona | ffmpeg |
| Video MP4/MOV/WEBM/AVI | preview mp4/webm | Si | Si | Si | Funciona | 8s clip |
| Video MP4/MOV/WEBM/AVI | thumbnails ZIP | Si | Si | No confirmado | Funciona | 6 frames |
| Video MP4/MOV/WEBM/AVI | contact sheet JPG | Si | Si | No confirmado | Funciona | 3x2 grid |
| Video MP4/MOV/WEBM/AVI | waveform PNG | Si | Si | No confirmado | Funciona | |

## Hallazgos críticos

### [CRÍTICO] HEIC/HEIF y SVG > 10MB bloqueados por el validador de upload aunque los engines soportan hasta 100MB

**Descripción:** En `validator.go:46-52`, cuando una imagen no tiene dimensiones parseables (Width/Height == nil, común en HEIC/HEIF con libheif o SVG con viewBox no detectado), el validador impone un límite conservador de 10MB para prevenir decompression bombs. Sin embargo, las capacidades del catálogo (`image-heic-to-jpg`, `image-heic-to-png`, `image-heic-to-webp`, `image-svg-to-png`, `image-svg-to-webp`, `image-svg-to-pdf`) declaran `MaxInputBytes: 100MB`. Esto crea una inconsistencia: archivos HEIC/SVG de entre 10MB y 100MB se rechazan en upload aunque el engine podría procesarlos.

**Impacto:** Usuarios con imágenes HEIC/HEIF o SVG de alta resolución (comunes en fotografía profesional y diseño) no pueden subirlas. El backend promete soporte hasta 100MB pero el validador rechaza a 10MB.

**Ubicación:**
- `apps/api/internal/ingestion/validator.go:46-52` — límite 10MB para imágenes sin dimensiones
- `apps/api/internal/capabilities/catalog.go:611-669` — capacidades HEIC con MaxInputBytes=100MB
- `apps/api/internal/capabilities/catalog.go:671-723` — capacidades SVG con MaxInputBytes=100MB

**Cómo verificarlo:** Intentar subir un HEIC >10MB con ffprobe ausente o fallando. Alternativamente, subir un SVG >10MB sin viewBox explícito.

**Recomendación:** Separar el límite de seguridad anti-bomba del límite funcional. Para HEIC: usar ffprobe como fallback para extraer dimensiones reales antes de aplicar el límite de píxeles. Si ffprobe también falla, rechazar con `"could not verify image dimensions — unsupported variant"`. Para SVG: el parser de viewBox ya cubre la mayoría de casos. Aumentar el límite conservador a 25MB o rechazar explícitamente con mensaje claro en lugar de un límite silencioso.

**Prioridad:** Alta.

### [CRÍTICO] Archivos legacy .doc (application/msword) detectados correctamente pero sin capacidades disponibles

**Descripción:** El detector mapea `application/msword` → `FamilyDocument`, ext `doc` en `mimeToFamily` y `mimeToExtension`. Sin embargo, **ninguna capacidad del catálogo** lista `application/msword` en sus SourceFormats. Las capacidades de documentos solo aceptan `application/vnd.openxmlformats-officedocument.wordprocessingml.document` (DOCX), `application/vnd.oasis.opendocument.text` (ODT), `application/rtf` y `text/rtf`. Esto significa que un archivo .doc se sube exitosamente pero recibe **cero capacidades disponibles**, mostrando una UI vacía. LibreOffice sí puede procesar .doc.

**Impacto:** Los usuarios de formatos antiguos de Word (todavía muy comunes en entornos corporativos y legales) no pueden hacer ninguna conversión, y el sistema no les explica por qué.

**Ubicación:**
- `apps/api/internal/ingestion/detector.go:44` — `application/msword` mapeado a FamilyDocument
- `apps/api/internal/capabilities/catalog.go:727-805` — capacidades de documento sin `application/msword`

**Cómo verificarlo:** Subir un archivo .doc y observar que la respuesta de capabilities es un array vacío.

**Recomendación:** Agregar `application/msword` a los SourceFormats de `doc-to-pdf`, `doc-to-txt`, y `doc-to-docx`. Crear capacidad `doc-to-docx` con fuente `application/msword` para migración a formato moderno.

**Prioridad:** Alta.

## Hallazgos importantes

### [IMPORTANTE] Documentos Office protegidos por contraseña no detectados antes de la conversión

**Descripción:** El detector de metadata (`ingestion/metadata.go`) solo verifica protección en PDFs (vía `pdfinfo Encrypted: yes`). Documentos DOCX, XLSX, PPTX y ODF protegidos por contraseña no se detectan. Estos archivos se suben exitosamente, pero fallan durante la conversión (LibreOffice no puede abrirlos), produciendo un error genérico `"El motor de conversión no pudo procesar este archivo"` que no informa al usuario del verdadero motivo.

**Impacto:** Mala experiencia de usuario — el usuario no sabe que su archivo está protegido. El error no es accionable (no sugiere quitar la contraseña).

**Ubicación:**
- `apps/api/internal/ingestion/metadata.go:44-73` — solo detecta protección en PDF
- `apps/api/internal/workers/handler.go:248-266` — classifyError genérico para exit status

**Cómo verificarlo:** Subir un DOCX protegido por contraseña e intentar convertir a PDF.

**Recomendación:** Agregar detección de protección para OOXML (verificar si `EncryptedPackage` o `EncryptionInfo` existe en el ZIP) y ODF (verificar `META-INF/manifest.xml` con `manifest:encryption-data`). Clasificar el error como `"Archivo protegido por contraseña — quita la protección y vuelve a intentar"`.

**Prioridad:** Alta.

### [IMPORTANTE] Race condition en verificación de cuota acumulativa de upload

**Descripción:** En `upload.go`, `enforceCumulativeQuota` lee el uso actual de bytes y verifica que `used + fileSize <= quota`. Sin embargo, entre esta lectura y la inserción del registro del archivo en BD (`h.Files.Create`), otra subida concurrente del mismo usuario/guest podría también pasar la verificación y exceder la cuota. No hay una transacción atómica que cubra check + insert.

**Impacto:** Los usuarios (especialmente guests con scripts) pueden exceder su cuota acumulativa mediante subidas concurrentes, ocupando más espacio en disco del permitido.

**Ubicación:** `apps/api/internal/api/handlers/upload.go:107,255-279`

**Cómo verificarlo:** Enviar múltiples uploads concurrentes desde la misma sesión guest justo debajo del límite de cuota. Verificar que el uso total puede exceder el límite.

**Recomendación:** Mover la verificación de cuota dentro de `Files.Create` o `CreateIfUnderQuota` usando una transacción SQLite con `SELECT ... FOR UPDATE` o un enfoque de `INSERT ... WHERE` condicional. Alternativamente, aceptar el riesgo para guests pero cerrar la brecha para usuarios registrados.

**Prioridad:** Media.

### [IMPORTANTE] Falta Content-Length en descarga de artefactos

**Descripción:** En `artifact.go:66`, el handler copia el archivo al response writer con `io.Copy(w, reader)`, pero nunca establece el header `Content-Length`. Esto impide que los navegadores muestren barra de progreso durante la descarga y puede causar problemas con proxies y CDNs.

**Impacto:** Descargas de archivos grandes (videos, PDFs, ZIPs multi-página) no muestran progreso al usuario.

**Ubicación:** `apps/api/internal/api/handlers/artifact.go:53-66`

**Recomendación:** Leer el tamaño del archivo del sistema (via `info.Size()` de `os.Stat`) y establecer `w.Header().Set("Content-Length", ...)` antes de copiar.

**Prioridad:** Media.

### [IMPORTANTE] Sin timeout de lectura del cuerpo multipart en upload

**Descripción:** `upload.go` establece `r.Body = http.MaxBytesReader(w, r.Body, limit)` que limita el tamaño total del body. Sin embargo, no hay un `http.TimeoutHandler` ni `ReadTimeout` específico para la lectura del stream multipart. Un cliente malicioso podría mantener una conexión abierta indefinidamente enviando datos muy lentamente (slowloris en upload).

**Impacto:** Potencial DoS por agotamiento de conexiones o goroutines bloqueadas en reads de red.

**Ubicación:** `apps/api/internal/api/handlers/upload.go:39-56`

**Recomendación:** Configurar `ReadTimeout` en el servidor HTTP (`http.Server.ReadTimeout`) o usar `context.WithTimeout` alrededor de `io.Copy(tempFile, file)`.

**Prioridad:** Media.

### [IMPORTANTE] Validación 1% del tamaño del output puede rechazar falsamente compresiones legítimas

**Descripción:** `output_validation.go:311` rechaza outputs cuyo tamaño sea menor al 1% del input. Esto protege contra conversiones que producen basura, pero **rechazará falsamente compresiones legítimas** que reduzcan el archivo a menos del 1% (ej. PDF de 100MB de puros escaneos a TXT extraído de 50KB, o imagen PNG de 50MB a JPG de 200KB).

**Impacto:** Conversiones exitosas pero con outputs muy pequeños se marcan como fallidas. El caso más probable es `pdf-to-txt` de un PDF enorme con poco texto o `image-to-jpg` de un PNG enorme.

**Ubicación:** `apps/api/internal/workers/output_validation.go:310-313`

**Recomendación:** Excluir explícitamente capacidades de extracción (OpExtract: `pdf-to-txt`, `html-to-txt`, `image-ocr-to-txt`) y compresión (OpCompress) del check 1%. Para OpConvert, mantener el check pero con umbral de 0.1% en lugar de 1%. Documentar esta decisión.

**Prioridad:** Media.

### [IMPORTANTE] Thumbnails de imagen solo disponibles para JPEG y PNG como fuente

**Descripción:** Las capacidades `image-thumbnail-jpg` (solo `image/jpeg`) y `image-thumbnail-png` (solo `image/png`) no aceptan WebP, GIF, BMP ni TIFF como fuente. Un usuario que sube un WebP no puede generar thumbnail directamente — debe convertirlo a JPG/PNG primero.

**Impacto:** Flujo innecesariamente complejo para usuarios con imágenes WebP/GIF/TIFF.

**Ubicación:** `apps/api/internal/capabilities/catalog.go:408-441`

**Recomendación:** Ampliar `image-thumbnail-jpg` a `image/png`, `image/webp`, `image/gif`, `image/bmp`, `image/tiff`. El engine go-image puede decodificar todos estos formatos.

**Prioridad:** Media.

### [IMPORTANTE] UI estática (categories.ts) desincronizada con capacidades reales del backend

**Descripción:** `categories.ts` define `acceptedFormats` y `targetFormats` como hints visuales. Sin embargo, hay capacidades del backend no visibles en la UI estática (ej. `image-compress-jpg`, `image-web-*`, `image-thumbnail-*`, `pdf-ocr-to-txt`, `pdf-ocr-searchable-pdf`). Esto no es un bug funcional porque tras upload la UI muestra capacidades reales desde el backend, pero puede confundir a usuarios que exploran la página antes de subir archivos.

**Impacto:** Usuarios no descubren todas las funcionalidades disponibles. Marketing pierde oportunidades de comunicar el valor completo del producto.

**Ubicación:** `apps/web/src/config/categories.ts`

**Recomendación:** Agregar un endpoint `GET /api/catalog` que devuelva todas las capacidades disponibles agrupadas por familia, y usarlo para poblar dinámicamente los hints de la UI. Mantener `categories.ts` solo para la estructura visual (íconos, labels, hints). O al menos actualizar manualmente para reflejar todas las capacidades.

**Prioridad:** Baja.

### [IMPORTANTE] ZIP de thumbnails de video no valida tipo MIME de su contenido interno — RESUELTO 2026-05-05

**Descripción:** `validateZipOutput` verifica estructura del ZIP (paths seguros, files no vacíos, máximo 2000 entradas) pero **no verifica que los archivos dentro del ZIP sean imágenes válidas** (JPG en este caso). Un ffmpeg corrupto o error silencioso podría producir un ZIP con entradas válidas pero contenido inválido.

**Estado:** Resuelto en Pasada 2. La validación ahora inspecciona la primera entrada no vacía del ZIP y exige MIME `image/jpeg` o `image/png`; se agregó test para entrada JPG corrupta.

**Impacto:** Usuario descarga un ZIP que parece correcto pero contiene imágenes corruptas. No se detecta hasta que el usuario intenta abrirlas.

**Ubicación:** `apps/api/internal/workers/output_validation.go:176-203`

**Recomendación:** Para ZIPs de thumbnails, muestrear el primer archivo y validar que sea un JPEG/PNG válido. Esto añade confianza sin penalizar rendimiento.

**Prioridad:** Baja.

### [IMPORTANTE] Falta validación anti-bomba ZIP en archivos subidos

**Descripción:** El detector (`detector.go:280-322`) inspecciona ZIPs entrantes para detectar OOXML, pero no verifica profundidad de anidamiento, ratio de compresión extremo, ni tamaño total expandido (zip bomb). Un atacante podría subir un ZIP malicioso (ej. `42.zip`) que el sistema intente inspeccionar o convertir, potencialmente agotando memoria o disco.

**Impacto:** Potencial DoS vía zip bomb si se procesa como documento (vía LibreOffice) o como ZIP genérico.

**Ubicación:** `apps/api/internal/ingestion/detector.go:281-322` — OOXML detection sin límites de descompresión

**Recomendación:** Verificar `compressed/uncompressed ratio` antes de abrir cualquier ZIP de entrada. Si `uncompressed/compressed > 100` o `total uncompressed > 500MB`, rechazar con `"file compression ratio too extreme"`. Aplicar en `detectOOXMLMimeFromZip`.

**Prioridad:** Media.

### [IMPORTANTE] Conversiones con cancelación durante la ejecución pueden dejar temporales huérfanos si el worker crashea

**Descripción:** El handler (`handler.go:130`) usa `defer h.Store.CleanupTemp` para limpiar temporales. Si el proceso worker crashea (panic, OOM kill, kill -9), este defer no se ejecuta. Los temporales quedan en disco hasta que el `CleanupService` los purgue en su siguiente ciclo.

**Impacto:** Acumulación de archivos temporales en disco entre ciclos de limpieza. En entornos con muchos workers crasheando, puede agotar el disco.

**Ubicación:**
- `apps/api/internal/workers/handler.go:126-130`
- `apps/api/internal/storage/cleanup.go`

**Recomendación:** Verificar que `CleanupService` tenga un intervalo razonable (ej. cada 5 minutos en producción). Agregar métrica `reform_temp_dir_count` para monitorear acumulación. Considerar usar `/tmp` del SO para temporales pequeños y el storage para archivos intermedios grandes.

**Prioridad:** Baja.

## Hallazgos menores

### [MENOR] PresentationOrder duplicado (600) para doc-to-pdf, txt-to-pdf, html-to-pdf

**Descripción:** Las capacidades `doc-to-pdf`, `txt-to-pdf` y `html-to-pdf` comparten `PresentationOrder: 600`. Como tienen SourceFormats mutuamente excluyentes, nunca aparecen juntas en la misma lista de capacidades (cada archivo solo matchea una), por lo que no hay impacto funcional. Pero viola la intención de tener órdenes únicos y estables.

**Ubicación:** `apps/api/internal/capabilities/catalog.go:48-51`

**Recomendación:** Asignar PresentationOrder distintos: 600 (doc-to-pdf), 601 (txt-to-pdf), 602 (html-to-pdf).

**Prioridad:** Baja.

### [MENOR] SVG a PDF no valida estructura PDF interna del output

**Descripción:** `validateBinaryOutputFormat` para SVG→PDF solo verifica que el MIME detectado del output sea `application/pdf`. No verifica estructura interna (header PDF, trailer, xref table) como sí se hace para DOCX/XLSX (validación OOXML). Un `rsvg-convert` que produce PDF corrupto pasaría la validación.

**Ubicación:** `apps/api/internal/workers/output_validation.go:83-87,92-106`

**Recomendación:** Agregar validación de header PDF para formato `pdf` en el `switch` de `validateOutputArtifact`: verificar que los primeros bytes sean `%PDF-`.

**Prioridad:** Baja.

### [MENOR] Fallback inseguro en sort cuando una capacidad no tiene PresentationOrder

**Descripción:** `withPresentationOrders` hace `panic` si un capability ID no está en `capabilityPresentationOrders`. Esto significa que si una nueva capacidad se agrega al slice `Catalog` sin su correspondiente entry en el map, el servidor crashea en startup. Es un fail-fast intencional pero frágil.

**Ubicación:** `apps/api/internal/capabilities/catalog.go:94-104`

**Recomendación:** Transformar en un test que recorra `Catalog` y verifique que todo ID tenga presentation order, en lugar de hacer panic en runtime. O usar `default` order de 9999 con un log warning.

**Prioridad:** Baja.

### [MENOR] Markdown detection puede producir falsos positivos con texto que contiene bullets

**Descripción:** La heurística `looksLikeMarkdown` requiere 2+ señales fuertes o 1 fuerte + 1 débil. Texto plano con bullets (`- item`) y un heading (`# title`) se detectaría como Markdown. Esto puede sorprender a usuarios que suben listas de texto plano.

**Ubicación:** `apps/api/internal/ingestion/detector.go:181-214`

**Recomendación:** Documentar en la UI que archivos con formato Markdown-like se tratarán como Markdown. Agregar validación de que el archivo tenga extensión `.md` o `.markdown` para confirmar la intención del usuario antes de ofrecer capacidades Markdown.

**Prioridad:** Baja.

### [MENOR] Conversión de CSV a CSV técnicamente posible pero filtrada por lógica de negocio

**Descripción:** CSV → XLSX funciona correctamente. CSV → CSV está bloqueado por `rejectsSameFormat` (OpConvert + csv==csv). Esto es correcto, pero no hay un mensaje claro al usuario explicando que "exportar a CSV" no aparece porque el archivo ya es CSV. La UI simplemente no muestra la opción sin explicación.

**Ubicación:** `apps/api/internal/capabilities/resolver.go:85-96`

**Recomendación:** Agregar a `CapabilityResponse` un campo `ineligibleReason` para que la UI pueda mostrar capacidades en gris con tooltip explicativo. Para V1, esto es nice-to-have.

**Prioridad:** Baja.

### [MENOR] El engine classification en el handler usa un string fijo en español

**Descripción:** `classifyError` en `handler.go` retorna strings en español hardcodeados. Para internacionalización futura, estos deberían ser claves de traducción o al menos constantes.

**Ubicación:** `apps/api/internal/workers/handler.go:248-266`

**Recomendación:** Extraer los mensajes de error a constantes o a un mapa de traducciones. Para V1, esto es aceptable ya que el producto es español-first.

**Prioridad:** Baja.

### [MENOR] El nombre del archivo de salida usa `filepath.Base` del output del engine

**Descripción:** `outputArtifactFileName` usa `filepath.Base(outputPath)`. Engines como LibreOffice o ffmpeg pueden producir nombres con caracteres especiales o paths relativos. Aunque `SaveArtifact` luego valida el fileName con `validateArtifactFileName`, sería más seguro sanitizar también en el handler.

**Ubicación:** `apps/api/internal/workers/handler.go:364-369`

**Recomendación:** Pasar el nombre por `security.SanitizeFileName` antes de usarlo en el artifact.

**Prioridad:** Baja.

### [MENOR] No hay métrica de duración de metadata extraction

**Descripción:** El handler de upload tiene métricas de subidas totales pero no mide el tiempo de extracción de metadatos (`ExtractMetadata`), que involucra llamadas a procesos externos (`pdfinfo`, `ffprobe`) y puede ser lento.

**Ubicación:** `apps/api/internal/api/handlers/upload.go:139`

**Recomendación:** Agregar `reform_metadata_extraction_duration_seconds` histogram en el handler de upload.

**Prioridad:** Baja.

## Inconsistencias detectadas

1. **ui-capabilities**: `categories.ts` muestra "JSON OCR" como target de PDF e imágenes, pero `pdf-ocr-to-txt` y `pdf-ocr-searchable-pdf` no están en los hints visuales. Solo `pdf-ocr-to-json` está expuesto como "JSON OCR".
2. **ui-capabilities**: `categories.ts` muestra "Word" (docx) como target de documentos, que en backend se llama "Convertir a Word" (`doc-to-docx`). OK.
3. **validator-vs-catalog**: HEIC/HEIF images con dimensiones no parseables tienen límite 10MB en validator pero 100MB en catalog (ver hallazgo crítico).
4. **engine-names**: El catalog usa `Engine: "libheif"` pero `engines.go` registra el engine como `"libheif"` con binarios `heif-convert` + `ffmpeg`. Consistente.
5. **doc-vs-document**: En la UI la categoría se llama "Documentos" (`id: "documents"`) pero las capacidades usan IDs con prefijo `doc-to-*` y `spreadsheet-to-*` y `presentation-to-*`. La resolución backend usa `FormatFamily == "document"`. Consistente con la UI vía `family: "document"`.
6. **presentation-order**: Varias capacidades comparten orden (600, 900, 1000) pero para diferentes source formats. Sin impacto funcional pero inconsistente con la intención de unicidad.
7. **batch-capabilities**: El endpoint batch retorna intersección de capacidades entre archivos. Si un archivo no comparte capacidades con otro, el resultado es vacío. La UI no explica esto al usuario.
8. **error-messages**: `respondError` en los handlers envía mensajes en español, pero `classifyError` en el worker también. El cliente `api.ts` espera `data.error` en inglés (`"Upload failed"`). Esta mezcla es funcional pero inconsistente.

## Casos borde y bugs potenciales

- Archivo vacío: Cubierto (validator.go:24, ErrInvalidCorrupted)
- Archivo corrupto: Parcial (detector puede fallar con MIME desconocido → ErrFormatUnsupported)
- Extensión falsa: Cubierto (detección por contenido, nunca confía en extensión)
- MIME type incorrecto: Cubierto (detección por magic bytes)
- Archivo muy grande: Cubierto (MaxBytesReader + validator por familia)
- Archivo protegido por contraseña PDF: Cubierto (pdfinfo Encrypted detection)
- Archivo protegido por contraseña Office: No cubierto
- Formato no soportado: Cubierto (ErrFormatUnsupported)
- Input y output iguales: Cubierto (rejectsSameFormat excepto video)
- Conversión simultánea: Cubierto (atomic job creation con active-job-limit por usuario/guest)
- Timeout: Cubierto (context.WithTimeout + engine-specific timeouts via capabilities)
- Error del conversor: Cubierto (classifyError con mensajes clasificados)
- Usuario cerrando la página: No aplica (jobs son asíncronos, estado persiste en BD)
- Descarga de archivo ajeno: Cubierto (canAccessResource verifica propiedad)
- Archivo expirado: Cubierto (TTL check + retention cleanup)
- Usuario superando límites del plan: Cubierto (cumulative quota + active job limits)
- Zip bomb: No cubierto (ver hallazgo importante)
- Slowloris en upload: No cubierto (ver hallazgo importante)
- Worker crash deja temporales: Parcial (defer + CleanupService)
- Video sin audio: Parcial (video-waveform-png documenta que fallará, pero otras capacidades de audio extraen stream primario silenciosamente)
- Archivo con nombre unicode/bidi: Parcial (sanitize elimina no-printables pero permite unicode)

## Robustez

- **Timeouts**: Bien implementados por capacidad (`ExecutionLimits.TimeoutSeconds`), por metadata extraction (5-8s), y por worker task (vía queue.TaskOptions.Timeout). La cancelación se propaga correctamente con `exec.CommandContext`.
- **Reintentos**: Configurados a nivel de capacidad (`MaxRetries: 1` en todas). El reintento manual (`RetryFailedJob`) solo permite jobs en estado `failed`. Sin backoff exponencial (el reintento es inmediato).
- **Cancelación**: Implementada con polling cada 500ms (`newExecutionContext`) que verifica si el job fue cancelado en BD. Los recursos (temp dirs, artifact parcial) se limpian al detectar cancelación.
- **Concurrencia**: Protegida vía límite de jobs activos por usuario/guest con creación atómica en BD (`CreateIfUnderLimit`). Sin embargo, la cuota acumulativa tiene race condition (ver hallazgo importante).
- **Limpieza de temporales**: `defer CleanupTemp` en handler + `CleanupService` periódico para huérfanos. Intervalo configurable.
- **Idempotencia**: No implementada explícitamente. Si un job se reintenta, se crea un nuevo artifact. El original se reusa.
- **Logs**: Estructurados con zerolog. Incluyen job_id, capability_id, file_id, duration_sec. No se loguea contenido sensible.
- **Métricas**: Cobertura sólida — uploads, jobs (total/duration), artifacts, active_jobs, rate_limits, http_requests, errors, disk_space. Falta métrica de metadata extraction duration.
- **Trazas**: OpenTelemetry con spans en el worker. Span attributes incluyen job.id, capability.id, file.id.

## UX de conversiones

- **Progreso**: El handler reporta progreso (20, 30, 40, 70, 80, 95) al orchestrator. El frontend hace polling de `GET /api/jobs/{jobId}` para updates. Bueno para el tamaño actual del producto.
- **Mensajes de error**: Clasificados y en español. Cubren los casos principales (timeout, engine error, output inválido, storage lleno, límite excedido, archivo protegido). El mensaje para "archivo protegido Office" es genérico (ver hallazgo).
- **Botón de reintento**: Implementado vía `POST /api/jobs/{jobId}/retry`. Solo jobs failed pueden reintentarse. UI muestra botón condicionalmente. OK.
- **Explicación de formatos**: La UI estática (categories.ts) muestra formatos aceptados y targets esperados. Tras upload, las capacidades reales vienen del backend. OK.
- **Historial**: Dashboard muestra archivos subidos y jobs. Admin dashboard tiene vista completa con filtros. OK.
- **Mobile**: No evaluado en detalle. Tailwind responsive classes detectadas en componentes.
- **Claridad de límites**: `GET /api/upload-policy` expone límites por tipo de usuario. La UI puede mostrar esta info. OK.

## Tests recomendados

### Unitarios nuevos
- `detector_test.go`: Caso .doc (application/msword) detectado correctamente como FamilyDocument
- `detector_test.go`: Caso ZIP bomb (ratio > 100) rechazado
- `validator_test.go`: Caso HEIC >10MB sin dimensiones rechazado con mensaje específico
- `validator_test.go`: Caso DOCX protegido por contraseña detectado como IsProtected
- `validator_test.go`: Caso XLSX/PPTX protegido por contraseña detectado como IsProtected
- `resolver_test.go`: Caso application/msword resuelve a capacidades de documento
- `resolver_test.go`: Caso text/csv no resuelve spreadsheet-to-csv
- `output_validation_test.go`: Caso output PDF truncado (sin trailer) detectado
- `output_validation_test.go`: Caso ZIP con contenido corrupto (entrada JPG inválida) detectado
- `output_validation_test.go`: Caso compresión legítima <1% (ej. PNG 50MB → JPG 200KB) no rechazado
- `handler_test.go`: Caso cancelación durante engine execution limpia temporales
- `handler_test.go`: Caso panic en engine → worker recovery → job marked failed
- `sanitize_test.go`: Caso nombre de archivo con caracteres bidi/RTLO

### Integración
- `upload_quota_test.go`: Caso concurrencia de uploads no excede cuota
- `capabilities_test.go`: Caso .doc legacy devuelve capacidades (no array vacío)

### E2E
- `api_e2e_test.go`: Flujo completo con archivo .doc → upload → 0 capacidades → error claro
- `api_e2e_test.go`: Flujo HEIC >10MB → upload → error claro (no límite genérico)
- `api_e2e_test.go`: Cancelación de job durante ejecución → estado cancelled → limpieza

### Seguridad
- `artifact_test.go`: Test de path traversal en artifact fileName
- `artifact_test.go`: Test de acceso a artifact de otro usuario/guest

## Mejoras técnicas

- Matriz centralizada de formatos: Ya implementada en `catalog.go`. OK.
- Tipos compartidos frontend/backend: Parcial. El frontend usa `CapabilityResponse` manualmente tipado. Se podría generar desde OpenAPI.
- Validación real de contenido: Implementada en `detector.go` (mimetype + heurísticas SVG/MD/CSV/Opus/OOXML). OK.
- Validación del output: Implementada en `output_validation.go` con checks de MIME, tamaño, estructura. Mejorable (ver hallazgos). OK.
- Jobs asíncronos: Implementado con Asynq/in-process queue. OK.
- Timeouts por formato: Implementado en `ExecutionLimits.TimeoutSeconds` por capacidad. OK.
- Logs estructurados: Implementados con zerolog. OK.
- Métricas por conversión: Implementadas con label `capability_id`. OK.
- Limpieza automática de temporales: Implementada (defer + CleanupService). OK.
- Fixtures reales para tests: Existen en `tests/fixtures/` (heif, presentation, spreadsheet) pero insuficientes. Se necesitan .doc, archivos protegidos, ZIP bombs, y archivos corruptos para cada familia.

## Nuevas funcionalidades sugeridas

| Funcionalidad | Valor | Complejidad | Prioridad |
|---|---|---|---|
| Soporte para .doc (application/msword) | Alto | Baja | Alta |
| Detección de documentos Office protegidos | Alto | Media | Alta |
| Conversión por lotes (batch) | Alto | Ya implementado | — |
| Historial de conversiones del usuario | Medio | Ya implementado | — |
| Preview inline del resultado (PDF, imágenes) | Medio | Media | Media |
| OCR multilingüe (selección de idioma Tesseract) | Medio | Baja | Media |
| Presets de calidad (baja/media/alta para imágenes) | Medio | Media | Baja |
| Integración con Google Drive / Dropbox | Medio | Alta | Baja |
| API pública documentada (OpenAPI) | Alto | Media | Alta |
| Webhooks custom (payload configurable) | Medio | Media | Baja |
| Dashboard interno de fallos por capability | Alto | Media | Alta |
| Transcripción audio/video (STT) | Alto | Alta | Media |
| Subtítulos automáticos (VTT/SRT) | Alto | Alta | Media |
| Conversión de email (.eml, .msg) a PDF | Bajo | Media | Baja |
| Comparación de documentos (diff visual) | Bajo | Alta | Baja |

## Quick wins

| Prioridad | Mejora | Impacto | Archivo/Ruta |
|---|---|---|---|
| Alta | Agregar `application/msword` como source a `doc-to-pdf`, `doc-to-txt`, `doc-to-docx` | .doc legacy usable | `catalog.go:727-784` |
| Alta | Detectar DOCX/XLSX/PPTX protegidos en metadata | Mejor UX de error | `metadata.go` |
| Alta | Mensaje de error específico para protected Office | Error accionable | `handler.go:248-266` |
| Alta | Content-Length en artifact download | Progreso de descarga | `artifact.go:61` |
| Media | Aumentar límite imágenes sin dimensiones de 10MB a 25MB | HEIC grandes subibles | `validator.go:48` |
| Media | Excluir OpExtract y OpCompress del check 1% output | Menos falsos rechazos | `output_validation.go:311` |
| Media | Header `%PDF-` validation para outputs pdf | Mejor detección PDF corrupto | `output_validation.go:58` |
| Baja | Separar PresentationOrders duplicados | Consistencia | `catalog.go:48-51` |
| Baja | Agregar métrica `metadata_extraction_duration` | Observabilidad | `upload.go:139` |
| Baja | Sanitizar `outputArtifactFileName` del engine | Seguridad defensiva | `handler.go:364-369` |

## Roadmap recomendado

### Fase 1: Correcciones críticas
1. Agregar `application/msword` a SourceFormats de `doc-to-pdf`, `doc-to-txt`, `doc-to-docx` en catalog.go
2. Corregir límite HEIC/SVG >10MB en validator (25MB o extracción de dimensiones vía ffprobe)
3. Agregar Content-Length header en descarga de artefactos
4. Detección de documentos Office protegidos en metadata.go
5. Mensaje de error específico para archivos protegidos en classifyError

### Fase 2: Robustez y observabilidad
6. Excluir OpExtract y OpCompress del check 1% output
7. Header PDF validation en output validator
8. Validación de ratio de compresión ZIP en detector (anti-bomb)
9. Pipeline E2E test para .doc y HEIC grande
10. Métrica metadata extraction duration
11. Sanitizar outputArtifactFileName

### Fase 3: Producto y nuevas funcionalidades
12. Dashboard de fallos por capability (admin)
13. API pública con OpenAPI spec
14. OCR multilingüe (Tesseract language selection)
15. Transcripción y subtitulado (STT pipeline)

## Checklist accionable

- [x] Agregar `application/msword` a SourceFormats en catalog.go para 3 capacidades
- [x] Aumentar límite imágenes sin dimensiones a 25MB en validator.go línea 48
- [x] Validar header `%PDF-` en output_validation.go para formato pdf
- [x] Detectar protección en OOXML y ODF en metadata.go
- [x] Agregar Content-Length header en artifact.go línea 61
- [x] Excluir OpExtract y OpCompress del check 1% en output_validation.go:311
- [x] Agregar ZIP bomb ratio check en detector.go:281
- [x] Agregar tests para .doc detection con capacidades esperadas
- [x] Agregar tests para HEIC >10MB
- [x] Agregar tests para documentos Office protegidos
- [x] Agregar tests para path traversal en artifact download
- [x] Agregar tests para permiso de descarga de artifact ajeno
- [x] Sanitizar outputArtifactFileName en handler.go:374
- [x] Agregar métrica `metadata_extraction_duration_seconds` en upload.go
- [x] Documentar todos los formatos en UI hints (categories.ts o endpoint /api/catalog)
- [x] Actualizar capabilities-catalog.md con estado real de cada capacidad

## Conclusión

El sistema de conversión de Reform Lab tiene una arquitectura sólida y bien pensada. La detección por contenido, la resolución centralizada de capacidades y la validación de outputs son fortalezas claras. Las correcciones más urgentes son de baja complejidad y alto impacto: habilitar .doc legacy (agregar un MIME type a 3 capabilities) y ajustar el límite HEIC en el validador (cambiar una constante). El resto de hallazgos son mejoras de robustez y UX que elevarán la calidad del producto sin requerir cambios arquitectónicos.

Las 5 acciones más importantes:

1. **Agregar `application/msword`** a `doc-to-pdf`, `doc-to-txt`, `doc-to-docx` — 3 líneas, impacto alto.
2. **Aumentar límite HEIC/SVG** de 10MB a 25MB en el validador — 1 línea, impacto alto.
3. **Detectar protección en documentos Office** (DOCX/XLSX/PPTX/ODF) — ~30 líneas, impacto alto en UX.
4. **Agregar Content-Length** en descarga de artefactos — 3 líneas, impacto medio en UX.
5. **Validar header PDF** en output validation — 5 líneas, previene falsos positivos.
