// apps/web/src/components/upload/FileUpload.tsx — Phase 9: Base upload component
import { useState, useRef, useCallback } from "react";
import {
  presignUpload,
  confirmUpload,
  uploadToStorage,
  validateFile,
  type UploadType,
} from "../../lib/api/files";

type UploadState = "idle" | "selecting" | "uploading" | "confirming" | "done" | "error";

type FileUploadProps = {
  token: string;
  uploadType: UploadType;
  accept: string;
  maxSizeMB: number;
  onSuccess: (url: string, fileId: string) => void;
  onError: (error: string) => void;
  children?: React.ReactNode;
  className?: string;
  disabled?: boolean;
};

export function FileUpload({
  token,
  uploadType,
  accept,
  maxSizeMB,
  onSuccess,
  onError,
  children,
  className = "",
  disabled = false,
}: FileUploadProps) {
  const [state, setState] = useState<UploadState>("idle");
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFileSelect = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

      // Client-side validation
      const validationError = validateFile(uploadType, file);
      if (validationError) {
        setError(validationError);
        setState("error");
        onError(validationError);
        return;
      }

      try {
        // Step 1: Get presigned URL
        setState("uploading");
        setProgress(0);
        const presign = await presignUpload(token, {
          upload_type: uploadType,
          filename: file.name,
          content_type: file.type,
          file_size: file.size,
        });

        // Step 2: Upload directly to storage
        await uploadToStorage(presign.upload_url, file, file.type, setProgress);

        // Step 3: Confirm upload
        setState("confirming");
        const confirmed = await confirmUpload(token, presign.file_id);

        setState("done");
        setProgress(100);
        onSuccess(confirmed.url, presign.file_id);
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Upload failed";
        setError(msg);
        setState("error");
        onError(msg);
      }

      // Reset input so same file can be selected again
      if (inputRef.current) {
        inputRef.current.value = "";
      }
    },
    [token, uploadType, onSuccess, onError]
  );

  const handleClick = () => {
    if (disabled || state === "uploading" || state === "confirming") return;
    setState("selecting");
    inputRef.current?.click();
  };

  const reset = () => {
    setState("idle");
    setProgress(0);
    setError(null);
  };

  return (
    <div className={className}>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleFileSelect}
        className="hidden"
        aria-hidden="true"
      />

      <div onClick={handleClick} style={{ cursor: disabled ? "not-allowed" : "pointer", opacity: disabled ? 0.5 : 1 }}>
        {children}
      </div>

      {(state === "uploading" || state === "confirming") && (
        <div className="mt-2">
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <p className="text-sm text-gray-500 mt-1">
            {state === "uploading" ? `Uploading... ${progress}%` : "Confirming..."}
          </p>
        </div>
      )}

      {state === "error" && error && (
        <div className="mt-2 text-sm text-red-600 flex items-center gap-2">
          <span>{error}</span>
          <button onClick={reset} className="text-blue-600 underline text-xs">
            Try again
          </button>
        </div>
      )}
    </div>
  );
}
