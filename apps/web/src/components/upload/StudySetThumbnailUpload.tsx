// apps/web/src/components/upload/StudySetThumbnailUpload.tsx — Phase 9: Study set thumbnail
import { useState, useCallback } from "react";
import { FileUpload } from "./FileUpload";
import { studySetApi } from "../../lib/api/client";

type StudySetThumbnailUploadProps = {
  token: string;
  studySetId: number;
  currentThumbnailUrl?: string | null;
  onThumbnailUpdated: (newUrl: string | null) => void;
};

export function StudySetThumbnailUpload({
  token,
  studySetId,
  currentThumbnailUrl,
  onThumbnailUpdated,
}: StudySetThumbnailUploadProps) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  const handleUploadSuccess = useCallback(
    async (url: string) => {
      try {
        await studySetApi.update(token, studySetId, { thumbnailUrl: url } as any);
        setPreviewUrl(url);
        onThumbnailUpdated(url);
      } catch (err) {
        console.error("Failed to update study set thumbnail:", err);
      }
    },
    [token, studySetId, onThumbnailUpdated]
  );

  const handleRemove = useCallback(async () => {
    try {
      await studySetApi.update(token, studySetId, { thumbnailUrl: null } as any);
      setPreviewUrl(null);
      onThumbnailUpdated(null);
    } catch (err) {
      console.error("Failed to remove study set thumbnail:", err);
    }
  }, [token, studySetId, onThumbnailUpdated]);

  const displayUrl = previewUrl || currentThumbnailUrl;

  return (
    <div className="space-y-2">
      {displayUrl && (
        <div className="relative group">
          <img
            src={displayUrl}
            alt="Study set thumbnail"
            className="w-full h-40 object-cover rounded-lg border"
          />
          <button
            onClick={handleRemove}
            className="absolute top-1 right-1 bg-red-600 text-white rounded-full w-6 h-6 flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
            title="Remove thumbnail"
          >
            ×
          </button>
        </div>
      )}

      <FileUpload
        token={token}
        uploadType="study_set_thumbnail"
        accept="image/jpeg,image/png,image/webp"
        maxSizeMB={5}
        onSuccess={handleUploadSuccess}
        onError={(err) => console.error("Thumbnail upload error:", err)}
      >
        <button className="px-3 py-1.5 text-sm text-blue-600 border border-blue-300 rounded-lg hover:bg-blue-50 transition-colors">
          {displayUrl ? "Change Thumbnail" : "Add Thumbnail"}
        </button>
      </FileUpload>
    </div>
  );
}
