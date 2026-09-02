// apps/web/src/components/upload/AvatarUpload.tsx — Phase 9: Avatar upload
import { useState, useCallback } from "react";
import { FileUpload } from "./FileUpload";
import { authApi } from "../../lib/api/client";

type AvatarUploadProps = {
  token: string;
  currentAvatarUrl?: string | null;
  onAvatarUpdated: (newUrl: string) => void;
};

export function AvatarUpload({ token, currentAvatarUrl, onAvatarUpdated }: AvatarUploadProps) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  const handleUploadSuccess = useCallback(
    async (url: string) => {
      // Update the profile with the new avatar URL
      try {
        await authApi.updateProfile(token, { image: url });
        setPreviewUrl(url);
        onAvatarUpdated(url);
      } catch (err) {
        console.error("Failed to update profile:", err);
      }
    },
    [token, onAvatarUpdated]
  );

  const displayUrl = previewUrl || currentAvatarUrl;

  return (
    <div className="flex items-center gap-4">
      {/* Avatar preview */}
      <div className="w-20 h-20 rounded-full overflow-hidden bg-gray-200 flex items-center justify-center flex-shrink-0">
        {displayUrl ? (
          <img src={displayUrl} alt="Avatar" className="w-full h-full object-cover" />
        ) : (
          <svg className="w-10 h-10 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
        )}
      </div>

      {/* Upload button */}
      <FileUpload
        token={token}
        uploadType="avatar"
        accept="image/jpeg,image/png,image/webp"
        maxSizeMB={5}
        onSuccess={handleUploadSuccess}
        onError={(err) => console.error("Avatar upload error:", err)}
      >
        <button className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors">
          Change Avatar
        </button>
      </FileUpload>
    </div>
  );
}
