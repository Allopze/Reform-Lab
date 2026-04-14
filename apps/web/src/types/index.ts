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

export type BatchFileStatus =
  | "uploading"
  | "selected"
  | "converting"
  | "done"
  | "error";

export interface BatchFileItem {
  localId: string;
  file: File;
  uploadedFileId?: string;
  jobId?: string;
  detectedFamily?: string;
  selectedCapabilityId: string;
  outputFormat: string;
  status: BatchFileStatus;
  progress: number;
  artifactId?: string;
  artifactFileName?: string;
  artifactMimeType?: string;
  artifactSize?: number;
  message?: string;
}

export interface FileState {
  items: BatchFileItem[];
}
