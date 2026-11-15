import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { classAccessApi, memberApi } from "../../lib/api";
import { useAuth } from "../auth/AuthContext";
import { ClassVideos } from "../classes/ClassVideos";
import "../classes/class-detail.css";
export function ClassVideoLibrary({ classId }: { classId: number }) {
  const { token, user } = useAuth();
  const [mode, setMode] = useState<"videos" | "upload">("videos");
  const access = useQuery({
    queryKey: ["media-class-access", user?.id, classId],
    queryFn: () => classAccessApi.me(token, classId),
  });
  const can = (action: string) =>
    access.data?.actions.includes(action) ?? false;
  const members = useQuery({
    queryKey: ["media-class-members", user?.id, classId],
    queryFn: () => memberApi.list(token, classId),
    enabled: can("class-member:List"),
  });
  if (access.isPending)
    return <p role="status">Đang kiểm tra quyền lớp học…</p>;
  if (access.isError)
    return <p role="alert">Không truy cập được video lớp học.</p>;
  return (
    <div className="group-detail-page">
      <div className="media-actions">
        <button onClick={() => setMode("videos")}>Video của lớp</button>
        {can("class-content:UploadVideo") && (
          <button onClick={() => setMode("upload")}>Tải video vào lớp</button>
        )}
      </div>
      {(
        mode === "upload"
          ? can("class-content:UploadVideo")
          : can("class-content:ViewVideo")
      ) ? (
        <ClassVideos
          classId={classId}
          members={members.data ?? []}
          mode={mode}
          canManage={can("class-content:UploadVideo")}
          canManageAudience={can("class-video:ManageAudience")}
          canSetCategory={can("class-video:Update")}
          canEditSubtitles={can("class-video:Update")}
        />
      ) : (
        <p>Bạn chưa được cấp quyền xem video của lớp này.</p>
      )}
    </div>
  );
}
