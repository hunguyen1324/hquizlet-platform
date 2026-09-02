// apps/web/src/lib/api/files.ts — Phase 9: File Upload API client
import { apiFetch } from "./client";

// Types
export type UploadType = "avatar" | "flashcard_image" | "study_set_thumbnail";

export type PresignRequest = {
  upload_type: UploadType;
  filename: string;
  content_type: string;
  file_size: number;
};

export type PresignResponse = {
  file_id: string;
  upload_url: string;
  upload_method: string;
  expires_at: string;
  headers_required: { "Content-Type": string };
};

export type ConfirmResponse = {
  file_id: string;
  url: string;
  upload_type: string;
  size_bytes: number;
  confirmed_at: string;
};

export type FileMetadata = {
  file_id: string;
  upload_type: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  url: string;
  status: string;
  created_at: string;
  confirmed_at?: string;
};

export type QuotaInfo = {
  active_files: number;
  max_files: number;
  used_bytes: number;
  max_bytes: number;
};

export type FileListResponse = {
  items: FileMetadata[];
  total: number;
  quota: QuotaInfo;
};

// Upload validation rules
const ALLOWED_MIME: Record<UploadType, string[]> = {
  avatar: ["image/jpeg", "image/png", "image/webp"],
  flashcard_image: ["image/jpeg", "image/png", "image/webp", "image/gif"],
  study_set_thumbnail: ["image/jpeg", "image/png", "image/webp"],
};

const MAX_SIZE: Record<UploadType, number> = {
  avatar: 5 * 1024 * 1024, // 5 MB
  flashcard_image: 10 * 1024 * 1024, // 10 MB
  study_set_thumbnail: 5 * 1024 * 1024, // 5 MB
};

// Client-side validation
export function validateFile(uploadType: UploadType, file: File): string | null {
  if (!ALLOWED_MIME[uploadType].includes(file.type)) {
    return `File type "${file.type}" is not allowed. Accepted: ${ALLOWED_MIME[uploadType].join(", ")}`;
  }
  if (file.size > MAX_SIZE[uploadType]) {
    const maxMB = MAX_SIZE[uploadType] / (1024 * 1024);
    return `File size exceeds ${maxMB}MB limit`;
  }
  return null;
}

// API methods
export async function presignUpload(token: string, req: PresignRequest): Promise<PresignResponse> {
  return apiFetch("/v1/files/presign", token, {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export async function confirmUpload(token: string, fileId: string): Promise<ConfirmResponse> {
  return apiFetch(`/v1/files/${fileId}/confirm`, token, { method: "POST" });
}

export async function deleteFile(token: string, fileId: string): Promise<void> {
  return apiFetch(`/v1/files/${fileId}`, token, { method: "DELETE" });
}

export async function listFiles(token: string, page?: number, perPage?: number): Promise<FileListResponse> {
  return apiFetch("/v1/files", token, {}, { page, per_page: perPage });
}

// Direct upload to storage (PUT with presigned URL)
export async function uploadToStorage(
  uploadUrl: string,
  file: File,
  contentType: string,
  onProgress?: (percent: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadUrl, true);
    xhr.setRequestHeader("Content-Type", contentType);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
      } else {
        reject(new Error(`Upload failed with status ${xhr.status}`));
      }
    };

    xhr.onerror = () => reject(new Error("Network error during upload"));
    xhr.send(file);
  });
}
