Corpus real para conversiones criticas de esta pasada.

- `heif/valid-basic.heif`: contenedor HEIF simple con marca `mif1`, usado para validar deteccion real y conversion via `libheif`.
- `heif/valid-complex.heif`: variante valida con dimensiones mayores y contenido sintetico menos trivial para revalidar conversiones reales.
- `heif/corrupted-truncated.heif`: fixture corrupto con cabecera conservada y payload truncado para fallos de decode.
- `doc/valid-basic.doc`: documento Word legacy real generado via LibreOffice, usado para detectar `application/msword` y verificar conversiones a PDF/DOCX.
- `protected/ooxml-encrypted-marker.docx`: contenedor OOXML persistido con marcadores reales de paquete cifrado (`EncryptionInfo`, `EncryptedPackage`) para metadata protegida.
- `protected/odf-encrypted-manifest.odt`: contenedor ODF persistido con `manifest:encryption-data` para metadata protegida.
- `security/zip-bomb-controlled.docx`: ZIP OOXML controlado con ratio de compresion extremo y payload pequeno, usado solo para validar rechazo anti ZIP-bomb.
- `pdf/corrupted-truncated.pdf`: PDF truncado con cabecera conservada para ejercer limites entre deteccion y validacion.
- `docx/corrupted-broken-zip.docx`: DOCX con ZIP roto para rechazo temprano de documento contenedor invalido.
- `image/corrupted-truncated.png`: PNG truncado con firma conservada para corpus corrupto de imagenes.
- `audio/corrupted-truncated.wav`: RIFF/WAV truncado para corpus corrupto de audio.
- `video/corrupted-truncated.mp4`: MP4 truncado con caja `ftyp` minima para corpus corrupto de video.
- `presentation/valid-two-slides.pptx`: presentacion de dos slides para verificar salidas multipagina agrupadas en ZIP.
- `presentation/valid-three-slides.pptx`: deck valido con tres slides para cubrir lotes ZIP mas grandes y relaciones OOXML adicionales.
- `presentation/corrupted-invalid.pptx`: archivo PPTX con bytes invalidos y firma rota para provocar un fallo de apertura no recuperable.
- `spreadsheet/valid-basic.xlsx`: hoja simple para round-trips reales de exportacion documental.
- `spreadsheet/valid-multi-sheet.xlsx`: workbook valido con varias hojas para cubrir entradas de mayor complejidad.
- `spreadsheet/corrupted-invalid.xlsx`: archivo XLSX con bytes invalidos y firma rota para provocar un fallo de apertura no recuperable.

Reglas de mantenimiento:

- mantener fixtures pequenos y legibles
- preferir variantes reales del ecosistema antes que mocks sinteticos cuando el formato lo permita
- cuando se agregue una variante corrupta, intentar preservar suficiente estructura para que el fallo aparezca en la capa correcta (deteccion, validacion o worker)
- cuando se agregue una variante compleja, documentar que aspecto del formato ejerce (mas slides, mas hojas, dimensiones mayores, etc.)
