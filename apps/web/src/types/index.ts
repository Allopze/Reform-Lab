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
  | {
      status: "selected";
      file: File;
      selectedCapabilityId: string;
      outputFormat: string;
    }
  | {
      status: "converting";
      file: File;
      selectedCapabilityId: string;
      outputFormat: string;
      progress: number;
    }
  | {
      status: "done";
      file: File;
      selectedCapabilityId: string;
      outputFormat: string;
      artifactId: string;
      artifactFileName?: string;
      artifactMimeType?: string;
      artifactSize?: number;
    }
  | {
      status: "error";
      file: File;
      selectedCapabilityId: string;
      outputFormat: string;
      message: string;
    };
