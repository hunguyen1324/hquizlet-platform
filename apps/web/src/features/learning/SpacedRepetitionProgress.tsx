import React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SpacedRepetitionPanel } from "../../components/progress";
import { useAuth } from "../auth/AuthContext";
import { repetitionApi, repetitionKey } from "./spacedRepetitionApi";

export function SpacedRepetitionProgress({ studySetId }: { studySetId: number }) {
 const { token, user } = useAuth();
 const client = useQueryClient();
 const [confirmReset, setConfirmReset] = React.useState(false);
 const queryKey = repetitionKey(String(user?.id ?? ""), studySetId);
 const stats = useQuery({ queryKey, queryFn: () => repetitionApi.stats(token, studySetId), enabled: !!token });
 const reset = useMutation({ mutationFn: () => repetitionApi.reset(token, studySetId), onSuccess: async () => { setConfirmReset(false); await client.invalidateQueries({ queryKey }); } });
 if (!token) return <p>Đăng nhập để lưu tiến độ ôn tập.</p>;
 if (stats.isError) return <div role="alert">Không tải được tiến độ ôn tập. <button onClick={() => void stats.refetch()}>Thử lại</button></div>;
 return <div className="sd-repetition">
  <SpacedRepetitionPanel stats={stats.data ?? { newCards: 0, learning: 0, almostMastered: 0, mastered: 0 }} loading={stats.isPending} onReset={stats.data ? () => setConfirmReset(true) : undefined} />
  {stats.data && <p className="sr-due" role="status">{stats.data.due} thẻ đến hạn ôn tập · Lật thẻ và tự đánh giá để cập nhật tiến độ.</p>}
  {confirmReset && <div className="sr-reset-confirm" role="alertdialog" aria-label="Học lại từ đầu">
   <p>Đặt lại tiến độ ôn tập của bạn trong học phần này?</p>
   <button disabled={reset.isPending} onClick={() => reset.mutate()}> {reset.isPending ? "Đang đặt lại…" : "Đặt lại tiến độ"}</button>
   <button disabled={reset.isPending} onClick={() => setConfirmReset(false)}>Hủy</button>
   {reset.isError && <p role="alert">Không đặt lại được tiến độ. Vui lòng thử lại.</p>}
  </div>}
 </div>;
}
