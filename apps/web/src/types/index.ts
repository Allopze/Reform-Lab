export type CategoryId =
  | "auto"
  | "pdf"
  | "images"
  | "documents"
  | "audio"
  | "video";

export interface FormatOption {
  value: string;
  label: string;
}

export interface CategoryConfig {
  id: CategoryId;
  label: string;
  icon: string;
  title: string;
  subtitle: string;
  dropzoneText: string;
  dropzoneHint: string;
  supportLabel: string;
  cta: string;
  acceptedFormats: FormatOption[];
  targetFormats: FormatOption[];
  acceptedMimeTypes: string;
}

export type FileState =
  | { status: "idle" }
  | { status: "selected"; file: File; outputFormat: string }
  | { status: "converting"; file: File; outputFormat: string; progress: number }
  | { status: "done"; file: File; outputFormat: string; resultUrl: string }
  | { status: "error"; file: File; outputFormat: string; message: string };
