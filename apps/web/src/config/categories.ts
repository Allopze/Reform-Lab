import type { CategoryConfig, CategoryId } from "@/types";

/**
 * Category definitions for the conversion UI.
 *
 * IMPORTANT: `targetFormats` here are UI hints only (shown before a file is uploaded).
 * After upload, the actual available formats come exclusively from the backend
 * via GET /api/files/{fileId}/capabilities. Never use these for conversion logic.
 */
export const categories: CategoryConfig[] = [
  {
    id: "auto",
    label: "Auto",
    icon: "search",
    title: "Detecta el formato real y muestra solo opciones compatibles",
    subtitle:
      "Sube un archivo y Reform Lab identifica su tipo para adaptar la conversión.",
    dropzoneText: "Arrastra tu archivo aquí",
    dropzoneHint: "PDF, imágenes, documentos, audio y video",
    supportLabel: "limites reales segun archivo y cuenta",
    cta: "Detectar y convertir",
    acceptedFormats: [],
    targetFormats: [],
    acceptedMimeTypes: "*/*",
  },
  {
    id: "pdf",
    label: "PDF",
    icon: "file-text",
    title: "Convierte tu PDF en segundos",
    subtitle:
      "Transforma documentos PDF a otros formatos sin perder calidad.",
    dropzoneText: "Arrastra tu PDF aquí",
    dropzoneHint: "PDF",
    supportLabel: "limite real segun tu cuenta",
    cta: "Convertir PDF",
    acceptedFormats: [{ value: "pdf", label: "PDF" }],
    targetFormats: [
      { value: "docx", label: "Word" },
      { value: "jpg", label: "JPG" },
      { value: "png", label: "PNG" },
      { value: "txt", label: "TXT" },
      { value: "pdf", label: "PDF optimizado" },
      { value: "html", label: "HTML" },
      { value: "json", label: "JSON OCR" },
    ],
    acceptedMimeTypes: "application/pdf",
  },
  {
    id: "images",
    label: "Imágenes",
    icon: "image",
    title: "Transforma imágenes entre formatos sin perder claridad",
    subtitle: "Convierte entre JPG, PNG y más con un solo clic.",
    dropzoneText: "Suelta una imagen para convertirla",
    dropzoneHint: "JPG, PNG, WEBP, GIF, BMP, TIFF, HEIC, SVG",
    supportLabel: "limite real segun tu cuenta",
    cta: "Convertir imagen",
    acceptedFormats: [
      { value: "jpg", label: "JPG" },
      { value: "png", label: "PNG" },
      { value: "webp", label: "WEBP" },
      { value: "gif", label: "GIF" },
      { value: "bmp", label: "BMP" },
      { value: "tiff", label: "TIFF" },
      { value: "heic", label: "HEIC" },
      { value: "svg", label: "SVG" },
    ],
    targetFormats: [
      { value: "png", label: "PNG" },
      { value: "jpg", label: "JPG" },
      { value: "webp", label: "WEBP" },
      { value: "avif", label: "AVIF" },
      { value: "pdf", label: "PDF" },
      { value: "txt", label: "TXT OCR" },
      { value: "json", label: "JSON OCR" },
    ],
    acceptedMimeTypes: "image/jpeg,image/png,image/webp,image/gif,image/bmp,image/tiff,image/heic,image/heif,image/svg+xml",
  },
  {
    id: "documents",
    label: "Documentos",
    icon: "files",
    title: "Convierte documentos de trabajo de forma rápida",
    subtitle:
      "Pasa entre formatos de oficina y texto plano sin complicaciones.",
    dropzoneText: "Selecciona un documento para convertir",
    dropzoneHint: "DOCX, ODT, TXT, RTF, MD, HTML, CSV, XLSX, ODS, PPTX, ODP",
    supportLabel: "limite real segun tu cuenta",
    cta: "Convertir documento",
    acceptedFormats: [
      { value: "docx", label: "DOCX" },
      { value: "odt", label: "ODT" },
      { value: "txt", label: "TXT" },
      { value: "rtf", label: "RTF" },
      { value: "md", label: "Markdown" },
      { value: "html", label: "HTML" },
      { value: "csv", label: "CSV" },
      { value: "xlsx", label: "XLSX" },
      { value: "ods", label: "ODS" },
      { value: "pptx", label: "PPTX" },
      { value: "odp", label: "ODP" },
    ],
    targetFormats: [
      { value: "pdf", label: "PDF" },
      { value: "docx", label: "DOCX" },
      { value: "txt", label: "TXT" },
      { value: "html", label: "HTML" },
      { value: "md", label: "Markdown" },
      { value: "csv", label: "CSV" },
      { value: "xlsx", label: "XLSX" },
      { value: "jpg", label: "JPG" },
      { value: "png", label: "PNG" },
    ],
    acceptedMimeTypes:
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.oasis.opendocument.text,text/plain,application/rtf,text/rtf,text/markdown,text/html,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.oasis.opendocument.spreadsheet,application/vnd.openxmlformats-officedocument.presentationml.presentation,application/vnd.oasis.opendocument.presentation",
  },
  {
    id: "audio",
    label: "Audio",
    icon: "music",
    title: "Pasa tu audio al formato que necesitas",
    subtitle: "Convierte entre MP3, WAV, OGG y más formatos de audio.",
    dropzoneText: "Sube un archivo de audio",
    dropzoneHint: "MP3, WAV, OGG, FLAC, AAC, M4A, OPUS",
    supportLabel: "limite real segun tu cuenta",
    cta: "Convertir audio",
    acceptedFormats: [
      { value: "mp3", label: "MP3" },
      { value: "wav", label: "WAV" },
      { value: "ogg", label: "OGG" },
      { value: "flac", label: "FLAC" },
      { value: "aac", label: "AAC" },
      { value: "m4a", label: "M4A" },
      { value: "opus", label: "OPUS" },
    ],
    targetFormats: [
      { value: "mp3", label: "MP3" },
      { value: "m4a", label: "M4A" },
      { value: "wav", label: "WAV" },
      { value: "flac", label: "FLAC" },
      { value: "aac", label: "AAC" },
      { value: "ogg", label: "OGG" },
      { value: "opus", label: "Opus" },
      { value: "png", label: "Waveform PNG" },
    ],
    acceptedMimeTypes: "audio/mpeg,audio/wav,audio/x-wav,audio/ogg,audio/flac,audio/x-flac,audio/aac,audio/mp4,audio/x-m4a,audio/opus",
  },
  {
    id: "video",
    label: "Video",
    icon: "video",
    title: "Prepara tus videos en el formato adecuado",
    subtitle: "Convierte entre MP4, MOV, WEBM y más.",
    dropzoneText: "Arrastra un video",
    dropzoneHint: "MP4, MOV, WEBM, AVI",
    supportLabel: "limite real segun tu cuenta",
    cta: "Convertir video",
    acceptedFormats: [
      { value: "mp4", label: "MP4" },
      { value: "mov", label: "MOV" },
      { value: "webm", label: "WEBM" },
      { value: "avi", label: "AVI" },
    ],
    targetFormats: [
      { value: "mp4", label: "MP4" },
      { value: "webm", label: "WEBM" },
      { value: "gif", label: "GIF" },
      { value: "mp3", label: "Audio MP3" },
      { value: "m4a", label: "Audio M4A" },
      { value: "wav", label: "Audio WAV" },
      { value: "flac", label: "Audio FLAC" },
      { value: "aac", label: "Audio AAC" },
      { value: "opus", label: "Audio Opus" },
      { value: "zip", label: "Thumbnails ZIP" },
      { value: "jpg", label: "Contact sheet JPG" },
      { value: "png", label: "Waveform PNG" },
    ],
    acceptedMimeTypes: "video/mp4,video/quicktime,video/webm,video/x-msvideo",
  },
];

export const DEFAULT_CATEGORY: CategoryId = "pdf";

function getNormalizedExtension(fileName: string): string {
  const lowerName = fileName.toLowerCase();

  if (lowerName.endsWith(".tar.gz")) return "tar.gz";
  if (lowerName.endsWith(".tar.bz2")) return "tar.bz2";
  if (lowerName.endsWith(".tar.xz")) return "tar.xz";

  const extension = lowerName.split(".").pop();
  return extension ?? "";
}

export function detectCategoryIdFromFile(file: File): Exclude<CategoryId, "auto"> | null {
  const mimeType = file.type.toLowerCase();
  const extension = getNormalizedExtension(file.name);

  if (mimeType === "application/pdf" || extension === "pdf") {
    return "pdf";
  }

  if (
    mimeType.startsWith("image/") ||
    ["jpg", "jpeg", "png", "webp", "gif", "bmp", "tiff", "tif", "svg", "heic", "heif"].includes(extension)
  ) {
    return "images";
  }

  if (
    mimeType.startsWith("audio/") ||
    ["mp3", "wav", "ogg", "flac", "aac", "m4a", "opus"].includes(extension)
  ) {
    return "audio";
  }

  if (
    mimeType.startsWith("video/") ||
    ["mp4", "mov", "webm", "avi"].includes(extension)
  ) {
    return "video";
  }

  if (
    [
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/vnd.oasis.opendocument.text",
      "text/plain",
      "application/rtf",
      "text/rtf",
      "text/markdown",
      "text/html",
      "text/csv",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "application/vnd.oasis.opendocument.spreadsheet",
      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      "application/vnd.oasis.opendocument.presentation",
    ].includes(mimeType) ||
    ["docx", "odt", "txt", "rtf", "md", "html", "csv", "xlsx", "ods", "pptx", "odp"].includes(extension)
  ) {
    return "documents";
  }

  return null;
}

export function getCategoryById(id: CategoryId): CategoryConfig {
  return categories.find((c) => c.id === id) ?? categories[0];
}

export function categoryIdFromDetectedFamily(
  family: string
): Exclude<CategoryId, "auto"> | null {
  switch (family) {
    case "pdf":
      return "pdf";
    case "image":
      return "images";
    case "document":
      return "documents";
    case "audio":
      return "audio";
    case "video":
      return "video";
    default:
      return null;
  }
}
