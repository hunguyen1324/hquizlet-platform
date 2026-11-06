// HomePage — assembler: fetches study sets once, renders all 5 sections
import React from "react";
import { useQuery } from "@tanstack/react-query";
import type { StudySet } from "../../types";
import { useAuth } from "../../features/auth/AuthContext";
import { studySetApi } from "../../lib/api";
import { LoadingSkeleton } from "../ui";
import { ContinueLearningCarousel } from "./ContinueLearningCarousel";
import { RecentList } from "./RecentList";
import { LearnModePreview } from "./LearnModePreview";
import { SuggestedCarousel } from "./SuggestedCarousel";
import { PlaySection } from "./PlaySection";
import { WorkspaceEmpty, WorkspaceIcon } from "../ui/WorkspaceUI";

type Props = {
  onOpenSet: (id: number) => void;
  onNavigate: (view: string) => void;
};

export function HomePage({ onOpenSet, onNavigate }: Props) {
  const { token, user } = useAuth();

  const { data: result, isLoading: loading, error } = useQuery({
    queryKey: ["studySets", "updated", 50],
    queryFn: () => studySetApi.list(token, { sort: "updated", per_page: 50 }),
  });

  const sets = result?.items ?? [];

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto py-8">
        <LoadingSkeleton rows={5} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl mx-auto py-8 text-center">
        <p className="text-[var(--muted-foreground)] mb-4">
          {error instanceof Error ? error.message : "Không tải được dữ liệu."}
        </p>
        <button className="primary-button" onClick={() => window.location.reload()}>Thử lại</button>
      </div>
    );
  }

  return (
    <div className="home-page workspace-page">
      <section className="home-welcome">
        <div className="home-welcome-copy"><p className="eyebrow">MỖI NGÀY MỘT CHÚT TIẾN BỘ</p><h1>Hôm nay, bạn muốn học gì?</h1><p>Chào {user?.name || "bạn"}, cùng tiếp nối hành trình khám phá kiến thức.</p><div className="home-welcome-actions"><button className="primary-button" onClick={() => onNavigate("dashboard")}>Khám phá thư viện <WorkspaceIcon name="arrow" size={18} /></button><button className="ghost-button" onClick={() => onNavigate("create-type")}><WorkspaceIcon name="plus" size={18} /> Tạo học phần</button></div></div>
        <div className="home-welcome-art" aria-hidden="true"><span className="home-orbit" /><div className="home-art-card home-art-card-back" /><div className="home-art-card"><WorkspaceIcon name="book" size={40} /><span>Học một điều mới</span><i /><i /></div><span className="home-art-star"><WorkspaceIcon name="sparkles" size={25} /></span><span className="home-art-check"><WorkspaceIcon name="check" size={25} /></span></div>
      </section>
      {sets.length === 0 && <WorkspaceEmpty icon="book" title="Hành trình học tập bắt đầu từ đây" description="Tạo bộ thẻ đầu tiên của bạn hoặc cùng học với bạn bè trong một nhóm." action={<button className="primary-button" onClick={() => onNavigate("create-type")}><WorkspaceIcon name="plus" size={18} /> Tạo học phần đầu tiên</button>} />}
      <ContinueLearningCarousel sets={sets} onOpenSet={onOpenSet} />
      <RecentList sets={sets} onOpenSet={onOpenSet} />
      <LearnModePreview sets={sets} onOpenSet={onOpenSet} />
      <SuggestedCarousel sets={sets} onOpenSet={onOpenSet} />
      <PlaySection onNavigate={onNavigate} />
    </div>
  );
}
