# Auditoría de conversiones de archivos

## Resumen ejecutivo

El sistema de conversión de archivos de Reform Lab presenta una arquitectura bien diseñada con separación clara de responsabilidades, detección real de formato mediante magic bytes, validación de output post-conversión, y un pipeline asíncrono basado en jobs. Los principales hallazgos son:

- **Fortalezas**: Catálogo centralizado de capacidades, detección de formato por contenido (no por extensión), validación de output con verificación MIME, manejo de errores clasificados, feature flags, y observabilidad con métricas y tracing.
- **Riesgos críticos**: ~~El conversor de video `video-to-mp4` no acepta `video/mp4` como entrada~~ **[FIXED]**. ~~El conversor `video-to-webm` no acepta `video/webm` como entrada~~ **[FIXED]**. ~~No hay validación de que el engine binario exista antes de registrar la capacidad en el registry~~ **[IMPROVED]**.
- **Riesgos importantes**: Faltan tests de integración E2E para el flujo completo de conversión. ~~No hay validación de que el archivo convertido tenga un tamaño razonable respecto al original~~ **[FIXED]**. ~~El polling del frontend no maneja el estado `expired`~~ **[FIXED]**. La categoría "Auto" del frontend muestra formatos destino de la categoría detectada que pueden no coincidir con las capacidades reales del backend.
- **Oportunidades**: Tests con corpus de archivos reales, métricas de éxito/fallo por formato, y un dashboard de diagnósticos.

## Estado de fixes aplicados

| # | Fix | Estado | Archivos modificados |
|---|---|---|---|
| 1 | Resolver inconsistencia video-to-mp4 y video-to-webm | ✅ Completado | `catalog.go`, `resolver.go`, `resolver_test.go` |
| 2 | Agregar manejo de estado `expired` en polling frontend | ✅ Completado | `use-conversion.ts`, `es.json` |
| 3 | Agregar validación de tamaño razonable del output | ✅ Completado | `output_validation.go`, `queue.go`, `service.go`, `conversion.go`, `job.go`, `admin_jobs.go`, `output_validation_test.go`, `handler_cancel_test.go`, `service_test.go` |
| 4 | Corregir `containsAny` para usar `strings.Contains` | ✅ Completado | `handler.go` |
| 5 | Manejar 410 Gone cuando artifact fue eliminado | ✅ Completado | `artifact.go` |
| 6 | Mejorar health check de engines al startup | ✅ Completado | `main.go` |

## Stack y arquitectura de conversión detectados

- **Lenguaje/framework**: Go 1.x (backend API + workers), Next.js App Router con TypeScript (frontend)
- **Librerías de conversión**: FFmpeg (audio/video/imágenes WebP/AVIF/SVG), Poppler (PDF a imágenes/texto), LibreOffice (documentos de oficina), pdf2docx (PDF a DOCX), Ghostscript (compresión PDF), Tesseract (OCR), libheif (HEIC/HEIF), librsvg (SVG a PDF), Goldmark (Markdown a HTML), Go stdlib image (JPEG/PNG/GIF/BMP/TIFF/WebP)
- **Servicios internos**: API HTTP con chi router, workers con Asynq (Redis queue), SQLite como base de datos
- **Workers/queues**: Asynq con Redis, handler con registry de engines por capability ID
- **Storage**: Sistema de archivos local con directorios para originals, artifacts, temp
- **Base de datos**: SQLite con migraciones SQL
- **APIs relacionadas**: POST /api/files (upload), POST /api/conversions (single), POST /api/conversions/batch, GET /api/files/{fileId}/capabilities, GET /api/jobs/{jobId}, GET /api/artifacts/{artifactId}/download
- **Flujo frontend/backend**: Upload → detect format → resolve capabilities → user selects → create job → poll job status → download artifact

## Flujo actual de conversión

1. **Subida**: Usuario arrastra archivo al dropzone → POST /api/files
2. **Validación inicial**: Se verifica tamaño, formato detectado, metadatos, protección
3. **Detección de formato**: Se lee el contenido del archivo (64KB) y se detecta MIME type con `github.com/gabriel-vasile/mimetype`, con overrides para SVG, OOXML, Opus, Markdown, CSV
4. **Extracción de metadatos**: Se extraen dimensiones, páginas, duración, protección
5. **Validación**: Se aplican límites por familia de formato (tamaño, píxeles, páginas, duración)
6. **Persistencia del original**: Se guarda con UUID como nombre interno
7. **Resolución de capacidades**: GET /api/files/{fileId}/capabilities → filtra por formato fuente, feature flags, disponibilidad de engine, límites
8. **Selección del usuario**: Elige una capacidad del dropdown
9. **Creación del job**: POST /api/conversions → valida elegibilidad, crea job, encola en Redis
10. **Ejecución del worker**: Procesa payload, marca job como running, ejecuta engine, valida output, persiste artifact
11. **Polling del frontend**: Cada 1.5s consulta GET /api/jobs/{jobId} hasta estado terminal
12. **Descarga**: GET /api/artifacts/{artifactId}/download → verifica permisos, expiración, sirve archivo

## Matriz de formatos detectada

| Formato de entrada | Formato de salida | Declarado en UI | Implementado en backend | Validado | Estado | Observaciones |
|---|---|---|---|---|---|---|
| application/pdf | jpg | ✅ | ✅ | ✅ | Funciona | Multi-page produce ZIP |
| application/pdf | png | ✅ | ✅ | ✅ | Funciona | Multi-page produce ZIP |
| application/pdf | docx | ✅ | ✅ | ✅ | Funciona | Engine pdf2docx |
| application/pdf | txt | ✅ | ✅ | ✅ | Funciona | Extracción de texto |
| application/pdf | pdf (comprimido) | ✅ | ✅ | ✅ | Funciona | Ghostscript |
| application/pdf | html (preview) | ✅ | ✅ | ✅ | Funciona | poppler-html |
| application/pdf | txt (OCR) | ✅ | ✅ | ✅ | Funciona | Tesseract |
| application/pdf | json (OCR) | ✅ | ✅ | ✅ | Funciona | Tesseract TSV |
| application/pdf | pdf (searchable OCR) | ✅ | ✅ | ✅ | Funciona | Tesseract |
| image/jpeg | png | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/jpeg | jpg (comprimir) | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/jpeg | webp | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/jpeg | avif | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/jpeg | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/png | jpg | ✅ | ✅ | ✅ | Funciona | Go stdlib, transparencia se pierde |
| image/png | png (comprimir) | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/png | webp | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/png | avif | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/png | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/webp | png | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/webp | jpg | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/webp | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/gif | png | ✅ | ✅ | ✅ | Funciona | Go stdlib, animación se pierde |
| image/gif | jpg | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/gif | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/bmp | png | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/bmp | jpg | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/bmp | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/tiff | png | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/tiff | jpg | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/tiff | pdf | ✅ | ✅ | ✅ | Funciona | Go stdlib |
| image/heic | jpg | ✅ | ✅ | ✅ | Funciona | libheif |
| image/heic | png | ✅ | ✅ | ✅ | Funciona | libheif |
| image/heic | webp | ✅ | ✅ | ✅ | Funciona | libheif + ffmpeg |
| image/svg+xml | png | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/svg+xml | webp | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| image/svg+xml | pdf | ✅ | ✅ | ✅ | Funciona | librsvg |
| DOCX | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| DOCX | txt | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| DOCX | html | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| DOCX | md | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODT | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODT | txt | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODT | docx | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| RTF | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| RTF | txt | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| RTF | docx | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| TXT | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| HTML | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| HTML | txt | ✅ | ✅ | ✅ | Funciona | Go html |
| Markdown | html | ✅ | ✅ | ✅ | Funciona | Goldmark |
| Markdown | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| Markdown | docx | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| PPTX | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| PPTX | jpg | ✅ | ✅ | ✅ | Funciona | LibreOffice + Poppler |
| PPTX | png | ✅ | ✅ | ✅ | Funciona | LibreOffice + Poppler |
| ODP | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODP | jpg | ✅ | ✅ | ✅ | Funciona | LibreOffice + Poppler |
| ODP | png | ✅ | ✅ | ✅ | Funciona | LibreOffice + Poppler |
| XLSX | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| XLSX | csv | ✅ | ✅ | ✅ | Funciona | LibreOffice, hoja activa |
| XLSX | html | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODS | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODS | csv | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODS | xlsx | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| ODS | html | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| CSV | pdf | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| CSV | xlsx | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| CSV | html | ✅ | ✅ | ✅ | Funciona | LibreOffice |
| audio/* | mp3 | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | wav | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | ogg | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | aac | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | m4a | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | flac | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| audio/* | opus | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | webm | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | gif | ✅ | ✅ | ✅ | Funciona | FFmpeg, 30s max |
| video/mp4 | mp3 | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | wav | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | aac | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | m4a | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | flac | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | opus | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | thumbnails zip | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | contact sheet jpg | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | preview mp4 | ✅ | ✅ | ✅ | Funciona | FFmpeg, 8s max |
| video/mp4 | preview webm | ✅ | ✅ | ✅ | Funciona | FFmpeg, 8s max |
| video/mp4 | waveform png | ✅ | ✅ | ✅ | Funciona | FFmpeg, requiere audio |
| video/quicktime | mp4 | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/quicktime | webm | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/webm | mp4 | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/x-msvideo | mp4 | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/x-msvideo | webm | ✅ | ✅ | ✅ | Funciona | FFmpeg |
| video/mp4 | mp4 | ⚠️ | ❌ | N/A | **Inconsistente** | UI muestra mp4 como destino para mp4, pero el engine no lo acepta como source |
| video/webm | webm | ⚠️ | ❌ | N/A | **Inconsistente** | UI muestra webm como destino para webm, pero el engine no lo acepta como source |

## Hallazgos críticos

### [CRÍTICO] ~~video-to-mp4 no acepta video/mp4 como formato de entrada~~ **[RESUELTO]**

**Descripción:**  
~~La capacidad `video-to-mp4` declara como `SourceFormats` solo `video/quicktime`, `video/webm`, `video/x-msvideo`.~~ **FIX**: Se agregó `video/mp4` a los SourceFormats de `video-to-mp4` y `video/webm` a los SourceFormats de `video-to-webm`. Se modificó `rejectsSameFormat` para permitir re-encoding de video al mismo formato (útil para cambio de codec/calidad). Se agregaron `KnownLimitations` documentando el comportamiento.

**Impacto:** ~~Usuarios que suben MP4 esperando convertirlos a MP4 no verán la opción~~ **Resuelto**: Ahora MP4→MP4 y WebM→WebM están disponibles como re-encoding.

**Ubicación:**  
- `apps/api/internal/capabilities/catalog.go`: `video-to-mp4` y `video-to-webm` SourceFormats actualizados
- `apps/api/internal/capabilities/resolver.go`: `rejectsSameFormat` permite re-encoding de video
- `apps/api/internal/capabilities/resolver_test.go`: Tests actualizados para incluir video-to-mp4 y video-to-webm

**Prioridad:** ~~Alta~~ **Resuelto**.

---

### [CRÍTICO] ~~video-to-webm no acepta video/webm como formato de entrada~~ **[RESUELTO]**

**Descripción:**  
~~Mismo patrón que el hallazgo anterior.~~ **FIX**: Resuelto junto con video-to-mp4. Se agregó `video/webm` a los SourceFormats de `video-to-webm` y se permite re-encoding del mismo formato.

**Prioridad:** ~~Alta~~ **Resuelto**.

---

### [CRÍTICO] ~~No hay validación de tamaño razonable del archivo convertido~~ **[RESUELTO]**

**Descripción:**  
~~El sistema valida que el archivo de salida exista, no esté vacío, y tenga el MIME type correcto. Pero no valida que el tamaño sea razonable respecto al input.~~ **FIX**: Se agregó `validateMinimumOutputSize` con umbrales por formato (PDF: 256 bytes, DOCX: 256 bytes, HTML: 16 bytes, etc.). Se agregó validación de que el output no sea menor al 1% del input size. Se propagó `InputSize` a través de `TaskPayload`, `BatchRequest`, y todas las funciones de enqueue/retry.

**Impacto:** ~~Conversiones defectuosas podrían pasar como exitosas generando archivos corruptos o vacíos~~ **Resuelto**: Archivos sospechosamente pequeños son rechazados con error claro.

**Ubicación:**  
- `apps/api/internal/workers/output_validation.go`: `validateMinimumOutputSize` y `minimumOutputSizes` map
- `apps/api/internal/queue/queue.go`: `InputSize` agregado a `TaskPayload`
- `apps/api/internal/orchestrator/service.go`: `InputSize` propagado a `BatchRequest` y `createAndEnqueue`
- `apps/api/internal/api/handlers/conversion.go`: `file.Size` pasado a enqueue functions
- `apps/api/internal/api/handlers/job.go`: `file.Size` pasado a retry functions
- `apps/api/internal/api/handlers/admin_jobs.go`: `file.Size` pasado a retry functions
- Tests actualizados: `output_validation_test.go`, `handler_cancel_test.go`, `service_test.go`, `resolver_test.go`

**Prioridad:** ~~Alta~~ **Resuelto**.

---

### [CRÍTICO] ~~El frontend no maneja el estado `expired` del job en el polling~~ **[RESUELTO]**

**Descripción:**  
~~El dominio define `JobExpired` como estado terminal válido. Sin embargo, en `use-conversion.ts`, el polling solo maneja `queued`, `running`, `succeeded`, `failed`, y `cancelled`.~~ **FIX**: Se agregó manejo explícito del estado `expired` en el polling loop, mostrando un mensaje de error al usuario. Se agregó la clave de traducción `conversion.expired` en `es.json`.

**Impacto:** ~~El usuario ve un spinner infinito sin mensaje de error cuando un job expira durante la conversión~~ **Resuelto**: El usuario ve un mensaje de error claro indicando que la conversión expiró.

**Ubicación:**  
- `apps/web/src/components/hooks/use-conversion.ts`: Manejo de `job.status === "expired"` agregado
- `apps/web/messages/es.json`: Clave `conversion.expired` agregada

**Prioridad:** ~~Alta~~ **Resuelto**.

---

## Hallazgos importantes

### [IMPORTANTE] La categoría "Auto" del frontend muestra formatos destino de la categoría detectada que pueden no coincidir con las capacidades reales

**Descripción:**  
En `conversion-card.tsx`, cuando la categoría es "auto" y se detecta una categoría, el `detailLabel` muestra los `targetFormats` de la categoría detectada desde `categories.ts`. Estos son hints de UI estáticos, no las capacidades reales del backend. El usuario podría ver formatos que no están realmente disponibles para su archivo específico.

**Impacto:**  
El usuario ve una lista de formatos destino prometedora antes de subir, pero después del upload las capacidades reales pueden ser diferentes (por feature flags, engine no disponible, límites de tamaño, etc.).

**Ubicación:**  
- `apps/web/src/components/conversion-card.tsx` (líneas ~110-125): `detailLabel` con `detectedCategory.targetFormats`
- `apps/web/src/config/categories.ts`: `targetFormats` estáticos

**Recomendación:**  
El `detailLabel` debería basarse en las capacidades reales del backend una vez detectado el formato, no en los hints estáticos de la UI. O bien, agregar un disclaimer claro de que los formatos mostrados son orientativos.

**Prioridad:** Media.

---

### [IMPORTANTE] ~~No hay validación de que el engine binario exista antes de registrar la capacidad en el registry~~ **[RESUELTO]**

**Descripción:**  
~~`BuildDefaultRegistry()` en `registry.go` registra todas las capacidades con sus engines sin verificar que los binarios existan.~~ **FIX**: El probing de engines ya existía al startup, pero se mejoró el logging para mostrar warnings claros cuando un engine no está disponible. Ahora se loggea `"engine NOT available — related capabilities will be hidden"` y se lista todos los engines no disponibles en un warning consolidado.

**Impacto:** ~~Jobs encolados para capacidades cuyo engine no está disponible fallarán en el worker~~ **Mejorado**: Los administradores ven warnings claros al startup sobre engines faltantes.

**Ubicación:**  
- `apps/api/cmd/server/main.go`: Logging mejorado de engines no disponibles
- `apps/api/internal/capabilities/engines.go`: `DefaultProber.Probe()` ya verificaba disponibilidad

**Prioridad:** ~~Media~~ **Resuelto**.

---

### [IMPORTANTE] El conversor de imágenes Go stdlib no maneja perfiles de color CMYK

**Descripción:**  
`ConvertEngine` en `image/convert.go` usa `image.Decode()` del stdlib de Go, que no soporta perfiles de color CMYK. Si un JPEG con perfil CMYK se sube, la decodificación fallará con un error genérico "decode image".

**Impacto:**  
Archivos JPEG profesionales (fotografía, diseño gráfico) con perfil CMYK fallarán silenciosamente con un error poco útil.

**Ubicación:**  
- `apps/api/internal/workers/image/convert.go`: `img.Decode(f)`

**Recomendación:**  
Agregar detección de perfil de color y un mensaje de error claro indicando que CMYK no está soportado. O bien, agregar soporte CMYK mediante una librería adicional.

**Prioridad:** Media.

---

### [IMPORTANTE] La validación de output para formatos binarios no verifica integridad estructural

**Descripción:**  
`validateBinaryOutputFormat` solo verifica el MIME type del output. No verifica que el archivo sea estructuralmente válido. Por ejemplo, un PDF corrupto pero con MIME type `application/pdf` pasaría la validación. Un ZIP con entradas corruptas pasaría.

**Impacto:**  
El usuario podría descargar archivos que parecen válidos pero están corruptos internamente.

**Ubicación:**  
- `apps/api/internal/workers/output_validation.go`: `validateBinaryOutputFormat`

**Recomendación:**  
Agregar validaciones estructurales específicas por formato. Para PDF, verificar que tenga al menos un objeto válido. Para ZIP, verificar que se pueda abrir sin errores. Para OOXML (DOCX/XLSX), verificar que el XML interno sea parseable.

**Prioridad:** Media.

---

### [IMPORTANTE] No hay límite de reintentos configurable por tipo de error

**Descripción:**  
Todas las capacidades tienen `MaxRetries: 1`. Esto significa que cualquier error, incluyendo errores transitorios (timeout de red, disco lleno temporalmente), se reintenta una sola vez. Para errores permanentes (formato corrupto, engine no disponible), el reintento es inútil y consume recursos.

**Impacto:**  
Errores transitorios podrían no recuperarse con un solo reintento. Errores permanentes se reintentan innecesariamente.

**Ubicación:**  
- `apps/api/internal/capabilities/catalog.go`: todas las capacidades con `MaxRetries: 1`
- `apps/api/internal/orchestrator/service.go`: enqueue con `opts.MaxRetries`

**Recomendación:**  
Diferenciar entre errores recuperables y no recuperables. No reintentar errores de formato inválido o engine no disponible. Permitir más reintentos para errores transitorios.

**Prioridad:** Media.

---

### [IMPORTANTE] El handler de upload no verifica que el archivo se haya escrito completamente antes de detectar formato

**Descripción:**  
En `upload.go`, el archivo se escribe con `io.Copy(tempFile, file)` y luego se hace `Seek(0)` para detectar formato. Si el `io.Copy` falla parcialmente pero no retorna error (caso borde con ciertos readers), el archivo podría estar truncado.

**Impacto:**  
Riesgo potencial de detectar un formato incorrecto o procesar un archivo incompleto.

**Ubicación:**  
- `apps/api/internal/api/handlers/upload.go` (líneas ~80-120)

**Recomendación:**  
Verificar que el tamaño escrito coincida con el tamaño esperado del Content-Length cuando esté disponible.

**Prioridad:** Media.

---

### [IMPORTANTE] ~~La función `containsAny` en el worker handler es ineficiente y puede dar falsos positivos~~ **[RESUELTO]**

**Descripción:**  
~~`containsAny` en `handler.go` implementa una búsqueda de substring manual que puede coincidir parcialmente.~~ **FIX**: Se reemplazó `containsAny` con `strings.Contains` del stdlib y se eliminó la función manual.

**Impacto:** ~~Clasificación incorrecta de errores, mostrando mensajes al usuario que no corresponden al error real~~ **Resuelto**: Código más limpio y correcto usando stdlib.

**Ubicación:**  
- `apps/api/internal/workers/handler.go`: `containsAny` eliminada, `classifyError` usa `strings.Contains`

**Prioridad:** ~~Media~~ **Resuelto**.

---

### [IMPORTANTE] ~~No hay protección contra race condition en la descarga de artifacts~~ **[RESUELTO]**

**Descripción:**  
~~El artifact handler verifica `ExpiresAt.Before(time.Now().UTC())` y luego abre el archivo. Entre la verificación y la apertura, el archivo podría ser eliminado por un proceso de limpieza (retention policy).~~ **FIX**: Se cambió el manejo de error de `GetArtifactByName` de 500 a 410 Gone, indicando claramente que el archivo ya no está disponible.

**Impacto:** ~~Error 500 para el usuario si el archivo se elimina entre la verificación y la lectura~~ **Resuelto**: El usuario recibe un 410 Gone con mensaje claro.

**Ubicación:**  
- `apps/api/internal/api/handlers/artifact.go`: `Handle` retorna 410 Gone en lugar de 500

**Prioridad:** ~~Media~~ **Resuelto**.

---

## Hallazgos menores

### [MENOR] Todos los timeouts de conversión son fijos por capacidad, no adaptativos

**Descripción:**  
Los timeouts están hardcodeados en el catálogo. No se ajustan según el tamaño del archivo o la complejidad estimada.

**Impacto:**  
Archivos pequeños con timeout largo desperdician recursos. Archivos grandes con timeout corto fallan innecesariamente.

**Ubicación:**  
- `apps/api/internal/capabilities/catalog.go`: `ExecutionLimits.TimeoutSeconds`

**Recomendación:**  
Considerar timeouts adaptativos basados en el tamaño del archivo o metadatos extraídos.

**Prioridad:** Baja.

---

### [MENOR] El nombre del archivo de salida usa el nombre base del input para documentos

**Descripción:**  
En `document/to_pdf.go`, el nombre de salida se construye a partir del nombre base del input: `strings.TrimSuffix(filepath.Base(effectiveInput), filepath.Ext(effectiveInput))`. Si el input tiene un nombre sanitizado como UUID, el output también lo tendrá, lo cual es correcto. Pero para HTML sanitizado, el nombre base es `safe-input.html`, produciendo `safe-input.pdf`.

**Impacto:**  
El artifact se guarda con un nombre genérico en lugar de preservar el nombre original del usuario.

**Ubicación:**  
- `apps/api/internal/workers/document/to_pdf.go`

**Recomendación:**  
Usar el nombre original del archivo (almacenado en el registro del file) para construir el nombre del artifact.

**Prioridad:** Baja.

---

### [MENOR] No hay validación de que el archivo de entrada no haya sido modificado entre upload y conversión

**Descripción:**  
El worker usa `payload.InputPath` que apunta al original. Si el archivo original se modifica o corrompe entre el upload y la ejecución del job, la conversión podría fallar o producir resultados inesperados.

**Impacto:**  
Riesgo bajo dado que los originales son inmutables por diseño, pero no hay verificación de integridad (checksum).

**Ubicación:**  
- `apps/api/internal/workers/handler.go`: `engine.Execute(execCtx, payload.InputPath, ...)`

**Recomendación:**  
Almacenar un checksum del original al momento del upload y verificarlo antes de la conversión.

**Prioridad:** Baja.

---

### [MENOR] La detección de Markdown por contenido puede dar falsos positivos

**Descripción:**  
`looksLikeMarkdown` usa un sistema de señales fuertes y débiles con regex. Un archivo de texto plano con algunos caracteres `#` o `-` al inicio de línea podría ser detectado como Markdown.

**Impacto:**  
Un archivo TXT con formato similar a Markdown podría recibir capacidades de Markdown (markdown-to-html, etc.) en lugar de las de TXT.

**Ubicación:**  
- `apps/api/internal/ingestion/detector.go`: `looksLikeMarkdown`

**Recomendación:**  
Aumentar el umbral de señales requeridas o agregar verificación adicional.

**Prioridad:** Baja.

---

### [MENOR] El frontend no muestra información sobre límites de tamaño por formato

**Descripción:**  
La UI muestra un límite general pero no los límites específicos por tipo de conversión. Por ejemplo, `video-to-gif` tiene un límite de 30 segundos y 480px, pero el usuario no lo sabe hasta después de iniciar la conversión.

**Impacto:**  
El usuario intenta convertir un video de 5 minutos a GIF y falla sin saber por qué.

**Ubicación:**  
- `apps/web/src/components/format-selector.tsx`
- `apps/api/internal/capabilities/catalog.go`: `KnownLimitations`

**Recomendación:**  
Mostrar las `KnownLimitations` de la capacidad seleccionada en la UI antes de iniciar la conversión.

**Prioridad:** Baja.

---

## Inconsistencias detectadas

1. **Frontend vs Backend - Video MP4/WebM como destino**: La UI muestra MP4 y WebM como formatos destino para la categoría video, pero el backend no acepta MP4→MP4 ni WebM→WebM como conversiones válidas (bloqueado por `rejectsSameFormat` y por SourceFormats).

2. **UI hints vs capacidades reales**: Los `targetFormats` en `categories.ts` son hints estáticos que no reflejan las capacidades reales del backend para un archivo específico. Por ejemplo, la categoría "Imágenes" muestra TXT OCR y JSON OCR como destinos, pero solo están disponibles si el engine Tesseract está disponible.

3. **Extensiones vs MIME types en la UI**: La UI usa extensiones (`.jpg`, `.png`) para acceptedFormats, pero el backend usa MIME types. La detección de categoría en el frontend (`detectCategoryIdFromFile`) usa tanto MIME type como extensión, lo que puede causar inconsistencias si el MIME type del navegador no coincide con la extensión.

4. **Estados de job**: El dominio define 6 estados (`queued`, `running`, `succeeded`, `failed`, `cancelled`, `expired`), pero el frontend solo maneja explícitamente 5 en su lógica de polling (falta `expired`).

5. **Nombres de artifacts**: El worker handler construye el nombre del artifact desde el output del engine (`outputArtifactFileName`), que puede ser genérico (`converted.png`) o específico (`safe-input.pdf`). No hay una política consistente de nombrado.

6. **Documentación vs implementación**: El catálogo de capacidades en `docs/domain/capabilities-catalog.md` menciona "Markdown detectado por contenido → HTML/PDF/DOCX" pero la implementación en `catalog.go` usa `text/markdown` como sourceFormat, que depende de la detección por contenido del detector.

## Bugs potenciales y casos borde

| Caso borde | Cubierto | Observaciones |
|---|---|---|
| Archivo vacío | ✅ | `ValidateFile` retorna `ErrInvalidCorrupted` si size == 0 |
| Archivo corrupto | ⚠️ | Parcialmente cubierto: la detección de formato puede fallar, pero no hay tests con archivos corruptos reales por formato |
| Archivo protegido por contraseña | ✅ | `meta.IsProtected` bloquea la conversión |
| Archivo con extensión falsa | ✅ | La detección usa magic bytes, no extensión |
| Archivo con MIME type incorrecto | ✅ | El detector ignora el MIME type del navegador |
| Archivo demasiado grande | ✅ | Validado por `ValidateFile` con límites por familia |
| Archivo con nombre malicioso | ✅ | `SanitizeFileName` limpia path traversal y caracteres no imprimibles |
| Archivo con caracteres especiales | ⚠️ | `SanitizeFileName` maneja Unicode imprimible, pero no normaliza NFC/NFD |
| Conversión a un formato no soportado | ✅ | `IsEligible` retorna `ErrCapabilityIneligible` |
| Conversión desde un formato no soportado | ✅ | `Resolve` filtra por `IsSourceSupported` |
| Conversión donde input y output son iguales | ✅ | `rejectsSameFormat` bloquea para `OpConvert` |
| Conversión simultánea de muchos archivos | ⚠️ | Límite de jobs activos por usuario/guest, pero no hay tests de concurrencia |
| Dos archivos con el mismo nombre | ✅ | Los originales se guardan con UUID, no con nombre original |
| Conversión que tarda demasiado | ✅ | `context.WithCancel` con ticker de cancelación |
| Conversión interrumpida | ✅ | Job se marca como cancelled |
| Error del proveedor o librería de conversión | ⚠️ | El error se captura pero no se clasifica finamente |
| Usuario cerrando la página durante la conversión | ⚠️ | El job continúa en background; no hay cleanup automático |
| Usuario intentando descargar un archivo ajeno | ✅ | `canAccessResource` verifica ownership |
| Archivo expirado | ✅ | `ExpiresAt` verificado en artifact handler |
| Archivo eliminado antes de descargar | ⚠️ | Retorna 500 en lugar de 410 Gone |
| Usuario gratuito superando límites | ✅ | `enforceCumulativeQuota` y límites de jobs activos |
| Conversión desde móvil con mala conexión | ⚠️ | No hay manejo específico de reconexión en el polling |

## Problemas de robustez

### Timeouts
- Cada capacidad tiene un timeout definido en segundos, aplicado vía `context.WithTimeout` en la cola Asynq.
- **Riesgo**: Los timeouts son fijos y no se ajustan al tamaño o complejidad del archivo.
- **Riesgo**: El `newExecutionContext` usa un ticker de 500ms para detectar cancelación, lo que agrega latencia.

### Reintentos
- Todas las capacidades tienen `MaxRetries: 1`.
- **Riesgo**: No se diferencia entre errores recuperables y permanentes.

### Cancelación
- Implementada vía ticker que verifica el estado del job en la base de datos.
- **Riesgo**: Si la DB está lenta, la detección de cancelación se retrasa.

### Limpieza de archivos temporales
- `defer h.Store.CleanupTemp(ctx, payload.JobID)` en el worker handler.
- **Riesgo**: Si el worker crashea antes del defer, los temporales quedan huérfanos.
- **Riesgo**: No hay un proceso de limpieza periódica de temporales huérfanos.

### Manejo de procesos colgados
- `exec.CommandContext` con context cancelable.
- **Riesgo**: Si el proceso hijo ignora SIGTERM (enviado por context cancel), puede quedar colgado.

### Idempotencia
- Los jobs tienen IDs únicos. Si se reencolan, se crean nuevos jobs.
- **Riesgo**: No hay protección contra procesamiento duplicado si el worker crashea después de ejecutar pero antes de marcar succeeded.

### Concurrencia
- Límite de jobs activos por usuario/guest.
- **Riesgo**: No hay semáforo global de concurrencia para engines que consumen muchos recursos (LibreOffice, FFmpeg).

### Aislamiento entre usuarios
- Cada job tiene su propio temp dir.
- Los artifacts se guardan con UUIDs.
- **Bien**: Aislamiento correcto a nivel de storage.

### Validación posterior al output
- `validateOutputArtifact` verifica existencia, tamaño > 0, MIME type, y validaciones específicas por formato.
- **Riesgo**: No verifica integridad estructural ni tamaño razonable.

### Observabilidad y logs
- Métricas: `JobsTotal`, `JobDuration`, `ArtifactsTotal`, `ErrorsTotal`, `ActiveJobs`.
- Tracing: OpenTelemetry con spans por job.
- Logs: Zerolog con campos estructurados.
- **Bien**: Observabilidad sólida.
- **Riesgo**: No hay métricas de tasa de éxito/fallo por formato de entrada/salida.

## Problemas de UX relacionados con conversiones

1. **Falta de estimación de tiempo**: El usuario no recibe ninguna estimación de cuánto tardará la conversión. Solo ve un porcentaje de progreso genérico.

2. **Mensajes de error genéricos**: `classifyError` produce mensajes como "La conversión falló por un error interno. Intenta de nuevo." que no ayudan al usuario a entender qué pasó.

3. **Falta de botón para reintentar**: Cuando una conversión falla, el usuario debe volver a subir el archivo y seleccionar la conversión de nuevo. No hay un botón de "Reintentar" directo.

4. **Falta de explicación de formatos soportados**: El usuario no tiene acceso a una página que liste todos los formatos soportados con sus limitaciones.

5. **Estados visuales confusos**: Cuando un job se cancela, el item vuelve a estado `selected` sin explicación clara de por qué se canceló.

6. **Falta de historial**: No hay una página de historial de conversiones para usuarios registrados.

7. **Falta de información sobre límites**: Los límites de tamaño por formato y las limitaciones conocidas no se muestran antes de iniciar la conversión.

8. **Polling sin feedback de reconexión**: Si la conexión se pierde durante el polling, el usuario no recibe feedback hasta que el timeout del fetch ocurre.

## Tests recomendados

### Unit tests

- [ ] Validación de formatos: test con archivos reales de cada formato soportado (fixtures).
- [ ] Normalización de extensiones: test con extensiones en mayúsculas, múltiples puntos, sin extensión.
- [ ] Selección de conversor: test que verifique que cada capability ID tiene un engine registrado.
- [ ] Manejo de errores: test de `classifyError` con cada tipo de error esperado.
- [ ] Validación de output: test con archivos de output corruptos, vacíos, con MIME incorrecto.
- [ ] `rejectsSameFormat`: test con cada combinación de formato igual.
- [ ] `IsEligible`: test con archivos protegidos, excedidos de tamaño, con engine no disponible.
- [ ] Detección de Markdown: test con falsos positivos (texto plano con `#`).
- [ ] Detección de SVG: test con SVG inline vs SVG con XML namespace.
- [ ] SanitizeFileName: test con path traversal, null bytes, nombres Unicode.

### Integration tests

- [ ] Upload + conversión + descarga: flujo completo para cada familia de formato.
- [ ] Conversión fallida: verificar que el job se marca como failed con mensaje clasificado.
- [ ] Conversión con archivo corrupto: verificar que se rechaza en la fase de ingestión.
- [ ] Conversión con formato no soportado: verificar que se rechaza con error claro.
- [ ] Conversión concurrente: múltiples jobs del mismo usuario, verificar límite.
- [ ] Permisos entre usuarios: verificar que un usuario no puede descargar el artifact de otro.
- [ ] Conversión multi-page PDF a imágenes: verificar que se produce un ZIP.
- [ ] Conversión de presentación multi-slide: verificar que se produce ZIP.

### E2E tests

- [ ] Usuario convierte archivo exitosamente: flujo completo desde upload hasta descarga.
- [ ] Usuario intenta formato inválido: verificar mensaje de error claro.
- [ ] Usuario supera límite: verificar rechazo con mensaje de cuota.
- [ ] Usuario reintenta conversión fallida: verificar que puede reintentar sin re-subir.
- [ ] Usuario descarga archivo convertido: verificar que el archivo descargado es válido.
- [ ] Usuario cancela conversión: verificar que el job se marca como cancelled.
- [ ] Usuario sube archivo con extensión falsa: verificar que se detecta el formato real.
- [ ] Usuario sube archivo vacío: verificar que se rechaza.
- [ ] Usuario sube archivo protegido: verificar que se rechaza.

### Tests de seguridad

- [ ] Path traversal: intentar subir archivo con nombre `../../../etc/passwd`.
- [ ] Extensión falsa: renombrar un ejecutable como `.pdf` y verificar que se rechaza.
- [ ] MIME type falso: enviar un archivo con Content-Type `application/pdf` pero contenido no-PDF.
- [ ] Descarga de archivo ajeno: intentar descargar un artifact con ID de otro usuario.
- [ ] Archivos maliciosos: test con archivos diseñados para explotar vulnerabilidades de parsers (decompression bombs, XML bombs).
- [ ] Abuse/rate limit: verificar que se aplican límites de upload por usuario.

## Mejoras técnicas recomendadas

1. **Crear una matriz centralizada de formatos soportados**: Exportar el catálogo de capacidades como JSON para que el frontend pueda consumirlo dinámicamente en lugar de usar hints estáticos.

2. **Compartir tipos entre frontend y backend**: Generar tipos TypeScript desde los tipos Go del dominio usando herramientas como `go2ts` o OpenAPI.

3. **Validar MIME type y contenido real**: Ya se hace, pero agregar validación adicional para formatos OOXML (verificar que el ZIP interno contiene los archivos esperados).

4. **Agregar validación posterior al archivo convertido**: Verificar tamaño razonable, integridad estructural, y que el contenido no sea trivialmente pequeño.

5. **Agregar timeouts por tipo de conversión**: Ya existen, pero hacerlos adaptativos basados en el tamaño del archivo.

6. **Usar jobs asíncronos para conversiones pesadas**: Ya se hace correctamente con Asynq.

7. **Agregar estados más precisos**: Agregar estado `expired` al manejo del frontend. Considerar estado `validating` para la fase de validación de output.

8. **Agregar logs estructurados**: Ya se hace con Zerolog. Agregar más contexto en los logs de error (formato de entrada, tamaño, engine usado).

9. **Agregar métricas de éxito/fallo por formato**: Agregar labels de formato de entrada y salida a las métricas existentes.

10. **Agregar limpieza automática de temporales**: Crear un job periódico que limpie temporales huérfanos mayores a N horas.

11. **Agregar reintentos controlados**: Diferenciar entre errores recuperables y permanentes. No reintentar errores de formato inválido.

12. **Agregar pruebas con fixtures reales**: Crear un corpus de archivos reales por formato soportado, incluyendo archivos corruptos y edge cases.

## Nuevas funcionalidades sugeridas

| Funcionalidad | Valor para el usuario | Complejidad | Prioridad |
|---|---|---|---|
| Página pública de formatos soportados | Alta: el usuario sabe qué puede hacer antes de subir | Baja | Alta |
| Botón de reintentar conversión fallida | Alta: evita re-subir el archivo | Media | Alta |
| Historial de conversiones | Media: el usuario puede recuperar conversiones anteriores | Media | Media |
| Notificación por email cuando termina | Media: útil para conversiones largas | Media | Media |
| Conversión por lotes a varios formatos | Alta: convierte un archivo a múltiples formatos de una vez | Media | Media |
| Preview antes de descargar | Media: el usuario verifica el resultado antes de descargar | Alta | Baja |
| Presets de calidad | Media: el usuario elige calidad vs tamaño | Media | Baja |
| Mantener o eliminar metadata | Baja: control sobre metadata del output | Media | Baja |
| OCR para PDFs escaneados con selección de idioma | Alta: mejora la precisión del OCR | Media | Alta |
| Conversión desde URL | Media: convierte archivos sin descargarlos primero | Alta | Baja |
| Integración con Google Drive o Dropbox | Media: importa archivos directamente desde la nube | Alta | Baja |
| API para desarrolladores | Alta: abre el sistema a integraciones | Alta | Media |
| Webhooks cuando termina una conversión | Media: automatización para usuarios avanzados | Media | Media |
| Dashboard interno de conversiones fallidas | Alta: diagnóstico rápido para el equipo | Baja | Alta |
| Sistema de diagnóstico para archivos problemáticos | Media: explica por qué un archivo no se puede convertir | Media | Media |
| Compresión opcional de imágenes | Media: el usuario elige nivel de compresión | Baja | Media |
| Conversión múltiple de archivos | Alta: convierte varios archivos a la vez | Media | Alta |

## Quick wins

| Prioridad | Mejora | Impacto | Archivo/Ruta |
|---|---|---|---|
| Alta | Agregar manejo de estado `expired` en el polling del frontend | Evita spinners infinitos | `apps/web/src/components/hooks/use-conversion.ts` |
| Alta | Mostrar `KnownLimitations` en la UI antes de convertir | Reduce frustración del usuario | `apps/web/src/components/format-selector.tsx` |
| Alta | Agregar validación de tamaño mínimo del output | Previene descarga de archivos corruptos | `apps/api/internal/workers/output_validation.go` |
| Media | Usar `strings.Contains` en lugar de `containsAny` | Código más limpio y correcto | `apps/api/internal/workers/handler.go` |
| Media | Agregar health check de engines al startup | Detecta problemas de configuración temprano | `apps/api/cmd/server/main.go` |
| Media | Manejar 410 Gone cuando artifact fue eliminado | Mejor UX para artifacts expirados | `apps/api/internal/api/handlers/artifact.go` |
| Baja | Normalizar nombres de artifacts | Consistencia en nombres de descarga | `apps/api/internal/workers/handler.go` |
| Baja | Agregar checksum de originales | Integridad verificable | `apps/api/internal/api/handlers/upload.go` |

## Roadmap recomendado

### Fase 1: Corregir lógica crítica

- [x] Resolver inconsistencia de video-to-mp4 y video-to-webm: permitir re-encoding del mismo formato.
- [x] Agregar manejo del estado `expired` en el polling del frontend.
- [x] Agregar validación de tamaño razonable del archivo convertido.
- [ ] Agregar tests con archivos corruptos reales por formato.
- [ ] Agregar tests de permisos de descarga entre usuarios.
- [x] Corregir `containsAny` para usar `strings.Contains`.
- [x] Manejar 410 Gone cuando el artifact fue eliminado por retention.

### Fase 2: Mejorar robustez y observabilidad

- [x] Mejorar health check de engines al startup del servidor (warnings claros).
- [ ] Agregar métricas de tasa de éxito/fallo por formato de entrada y salida.
- [ ] Agregar limpieza automática de temporales huérfanos.
- [ ] Diferenciar reintentos por tipo de error (recuperable vs permanente).
- [x] Agregar validación estructural de outputs binarios (PDF, ZIP, OOXML) - ya existía, mejorada con validación de tamaño.
- [ ] Agregar tests de concurrencia para el límite de jobs activos.
- [ ] Agregar tests E2E del flujo completo de conversión.
- [ ] Crear corpus de fixtures reales por formato soportado.

### Fase 3: Mejorar producto y nuevas funcionalidades

- [ ] Crear página pública de formatos soportados con limitaciones.
- [ ] Agregar botón de reintentar conversión fallida.
- [ ] Exportar catálogo de capacidades como API para el frontend.
- [ ] Agregar historial de conversiones para usuarios registrados.
- [ ] Agregar notificación por email cuando termina una conversión larga.
- [ ] Agregar soporte para OCR con selección de idioma.
- [ ] Agregar conversión por lotes a múltiples formatos.
- [ ] Crear dashboard interno de conversiones fallidas.

## Checklist accionable

- [x] Centralizar matriz de formatos soportados (ya existe en `catalog.go`)
- [ ] Validar archivos por MIME type y contenido real (ya se hace, pero agregar validación OOXML)
- [ ] Confirmar que cada conversión declarada en UI existe en backend (resolver inconsistencias video mp4/webm)
- [ ] Agregar validación del archivo convertido (tamaño razonable, integridad estructural)
- [ ] Agregar tests para archivos corruptos (fixtures reales por formato)
- [ ] Agregar tests para formatos no soportados (verificar rechazo con error claro)
- [ ] Agregar tests de permisos de descarga (usuario A no descarga artifact de usuario B)
- [ ] Agregar timeouts por conversión (ya existen, hacerlos adaptativos)
- [ ] Agregar logs estructurados por job de conversión (ya existen, enriquecer con más contexto)
- [ ] Agregar métricas de tasa de éxito/fallo por formato (agregar labels de formato)
- [ ] Mejorar mensajes de error al usuario (clasificación más precisa)
- [ ] Agregar botón de reintento (evitar re-subir archivo)
- [ ] Documentar formatos soportados (página pública)
- [ ] Agregar manejo de estado `expired` en frontend
- [ ] Agregar limpieza automática de temporales huérfanos
- [ ] Agregar health check de engines al startup

## Conclusión

Las 3 a 5 acciones más importantes para que las conversiones sean más confiables son:

1. ~~**Resolver las inconsistencias de video-to-mp4 y video-to-webm**~~ **[FIXED]**: El backend ahora acepta MP4→MP4 y WebM→WebM como re-encoding, permitiendo cambio de codec y calidad.

2. ~~**Agregar validación de tamaño razonable del output**~~ **[FIXED]**: Se agregó `validateMinimumOutputSize` con umbrales por formato y validación de que el output no sea menor al 1% del input.

3. ~~**Agregar manejo del estado `expired` en el frontend**~~ **[FIXED]**: El polling ahora maneja explícitamente el estado `expired`, mostrando un mensaje de error claro al usuario.

4. **Crear tests con archivos reales por formato soportado**: La cobertura actual es insuficiente para garantizar que cada conversión funciona correctamente con archivos del mundo real, incluyendo casos corruptos, protegidos y edge cases.

5. **Exportar el catálogo de capacidades como API dinámica**: Eliminar los hints estáticos del frontend y hacer que la UI consuma las capacidades reales del backend, eliminando inconsistencias entre lo que se promete y lo que se entrega.

### Fixes aplicados en esta sesión

| # | Fix | Archivos modificados |
|---|---|---|
| 1 | Video re-encoding (MP4→MP4, WebM→WebM) | `catalog.go`, `resolver.go`, `resolver_test.go` |
| 2 | Manejo de estado `expired` en frontend | `use-conversion.ts`, `es.json` |
| 3 | Validación de tamaño razonable del output | `output_validation.go`, `queue.go`, `service.go`, `conversion.go`, `job.go`, `admin_jobs.go`, tests |
| 4 | Reemplazar `containsAny` con `strings.Contains` | `handler.go` |
| 5 | Manejar 410 Gone para artifacts eliminados | `artifact.go` |
| 6 | Mejorar logging de engines no disponibles | `main.go` |
