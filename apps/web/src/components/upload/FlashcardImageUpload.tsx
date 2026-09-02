// apps/web/src/components/upload/FlashcardImageUpload.tsx — Phase 9: Flashcard image
import { useState, useCallback } from "react";
import { FileUpload } from "./FileUpload";
import { flashcardApi } from "../../lib/api/client";

type FlashcardImageUploadProps = {
  token: string;
  flashcardId: number;
  currentImageUrl?: string | null;
  onImageUpdated: (newUrl: string | null) => void;
};

export function FlashcardImageUpload({
  token,
  flashcardId,
  currentImageUrl,
  onImageUpdated,
}: FlashcardImageUploadProps) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  const handleUploadSuccess = useCallback(
    async (url: string) => {
      try {
        await flashcardApi.update(token, flashcardId, { imageUrl: url } as any);
        setPreviewUrl(url);
        onImageUpdated(url);
      } catch (err) {
        console.error("Failed to update flashcard image:", err);
      }
    },
    [token, flashcardId, onImageUpdated]
  );

  const handleRemove = useCallback(async () => {
    try {
      await flashcardApi.update(token, flashcardId, { imageUrl: null } as any);
      setPreviewUrl(null);
      onImageUpdated(null);
    } catch (err) {
      console.error("Failed to remove flashcard image:", err);
    }
  }, [token, flashcardId, onImageUpdated]);

  const displayUrl = previewUrl || currentImageUrl;

  return (
    <div className="space-y-2">
      {displayUrl && (
        <div className="relative group">
          <img
            src={displayUrl}
            alt="Flashcard illustration"
            className="w-full h-32 object-cover rounded-lg border"
          />
          <button
            onClick={handleRemove}
            className="absolute top-1 right-1 bg-red-600 text-white rounded-full w-6 h-6 flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
            title="Remove image"
          >
            ×
          </button>
        </div>
      )}

      <FileUpload
        token={token}
        uploadType="flashcard_image"
        accept="image/jpeg,image/png,image/webp,image/gif"
        maxSizeMB={10}
        onSuccess={handleUploadSuccess}
        onError={(err) => console.error("Flashcard image error:", err)}
      >
        <button className="px-3 py-1.5 text-sm text-blue-600 border border-blue-300 rounded-lg hover:bg-blue-50 transition-colors">
          {displayUrl ? "Change Image" : "Add Image"}
        </button>
      </FileUpload>
    </div>
  );
}
