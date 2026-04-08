import type { CategoryConfig, CategoryId } from "@/types";

export const categories: CategoryConfig[] = [
  {
    id: "pdf",
    label: "PDF",
    icon: "file-text",
    title: "Convierte tu PDF en segundos",
    subtitle:
      "Transforma documentos PDF a otros formatos sin perder calidad.",
    dropzoneText: "Arrastra tu PDF aquí",
    dropzoneHint: "PDF · máx. 100 MB",
    supportLabel: "hasta 100 MB",
    cta: "Convertir PDF",
    acceptedFormats: [{ value: "pdf", label: "PDF" }],
    targetFormats: [
      { value: "docx", label: "Word" },
      { value: "jpg", label: "JPG" },
      { value: "png", label: "PNG" },
      { value: "txt", label: "TXT" },
    ],
    acceptedMimeTypes: "application/pdf",
  },
  {
    id: "images",
    label: "Imágenes",
    icon: "image",
    title: "Transforma imágenes entre formatos sin perder claridad",
    subtitle: "Convierte entre JPG, PNG, WEBP y más con un solo clic.",
    dropzoneText: "Suelta una imagen para convertirla",
    dropzoneHint: "JPG, PNG, WEBP, GIF, BMP, TIFF",
    supportLabel: "hasta 100 MB",
    cta: "Convertir imagen",
    acceptedFormats: [
      { value: "jpg", label: "JPG" },
      { value: "png", label: "PNG" },
      { value: "webp", label: "WEBP" },
      { value: "gif", label: "GIF" },
      { value: "bmp", label: "BMP" },
      { value: "tiff", label: "TIFF" },
    ],
    targetFormats: [
      { value: "png", label: "PNG" },
      { value: "jpg", label: "JPG" },
      { value: "webp", label: "WEBP" },
      { value: "pdf", label: "PDF" },
    ],
    acceptedMimeTypes: "image/jpeg,image/png,image/webp,image/gif,image/bmp,image/tiff",
  },
  {
    id: "documents",
    label: "Documentos",
    icon: "file",
    title: "Convierte documentos de trabajo de forma rápida",
    subtitle:
      "Pasa entre formatos de oficina y texto plano sin complicaciones.",
    dropzoneText: "Selecciona un documento para convertir",
    dropzoneHint: "DOCX, ODT, TXT, RTF",
    supportLabel: "hasta 100 MB",
    cta: "Convertir documento",
    acceptedFormats: [
      { value: "docx", label: "DOCX" },
      { value: "odt", label: "ODT" },
      { value: "txt", label: "TXT" },
      { value: "rtf", label: "RTF" },
    ],
    targetFormats: [
      { value: "pdf", label: "PDF" },
      { value: "docx", label: "DOCX" },
      { value: "txt", label: "TXT" },
    ],
    acceptedMimeTypes:
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.oasis.opendocument.text,text/plain,application/rtf",
  },
  {
    id: "audio",
    label: "Audio",
    icon: "music",
    title: "Pasa tu audio al formato que necesitas",
    subtitle: "Convierte entre MP3, WAV, OGG y más formatos de audio.",
    dropzoneText: "Sube un archivo de audio",
    dropzoneHint: "MP3, WAV, OGG, FLAC, AAC",
    supportLabel: "hasta 250 MB",
    cta: "Convertir audio",
    acceptedFormats: [
      { value: "mp3", label: "MP3" },
      { value: "wav", label: "WAV" },
      { value: "ogg", label: "OGG" },
      { value: "flac", label: "FLAC" },
      { value: "aac", label: "AAC" },
    ],
    targetFormats: [
      { value: "mp3", label: "MP3" },
      { value: "wav", label: "WAV" },
      { value: "ogg", label: "OGG" },
    ],
    acceptedMimeTypes: "audio/mpeg,audio/wav,audio/ogg,audio/flac,audio/aac",
  },
  {
    id: "video",
    label: "Video",
    icon: "video",
    title: "Prepara tus videos en el formato adecuado",
    subtitle: "Convierte entre MP4, MOV, WEBM y más.",
    dropzoneText: "Arrastra un video",
    dropzoneHint: "MP4, MOV, WEBM, AVI · máx. 500 MB",
    supportLabel: "hasta 500 MB",
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
    ],
    acceptedMimeTypes: "video/mp4,video/quicktime,video/webm,video/x-msvideo",
  },
  {
    id: "archives",
    label: "Comprimidos",
    icon: "archive",
    title: "Convierte o empaqueta archivos comprimidos fácilmente",
    subtitle: "Cambia entre formatos de compresión sin instalar nada.",
    dropzoneText: "Sube un archivo comprimido",
    dropzoneHint: "ZIP, RAR, 7Z, TAR.GZ",
    supportLabel: "hasta 250 MB",
    cta: "Convertir archivo",
    acceptedFormats: [
      { value: "zip", label: "ZIP" },
      { value: "rar", label: "RAR" },
      { value: "7z", label: "7Z" },
      { value: "tar.gz", label: "TAR.GZ" },
    ],
    targetFormats: [
      { value: "zip", label: "ZIP" },
      { value: "7z", label: "7Z" },
    ],
    acceptedMimeTypes:
      "application/zip,application/x-rar-compressed,application/x-7z-compressed,application/gzip",
  },
];

export const DEFAULT_CATEGORY: CategoryId = "pdf";

export function getCategoryById(id: CategoryId): CategoryConfig {
  return categories.find((c) => c.id === id) ?? categories[0];
}
