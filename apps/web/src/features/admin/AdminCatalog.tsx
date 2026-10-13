import React, { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DataTable, EmptyState, SectionCard, type Column } from "../../components/dashboard";
import { apiFetch } from "../../lib/api/client";
import { useAuth } from "../auth/AuthContext";

type Row = { id: number; name?: string; title?: string; ownerUserId: number; memberCount?: number; cardCount?: number; quizCount?: number; hidden?: boolean };
type Preview = { studySet: { title: string; description: string }; flashcards: Array<{term: string; definition: string}>; questions: Array<{question_text?: string; text?: string}> };

export function AdminCatalog({kind}: {kind: "content" | "classes"}) {
 const {token} = useAuth();
 const qc = useQueryClient();
 const [search,setSearch] = useState("");
 const [offset,setOffset] = useState(0);
 const [previewId,setPreviewId] = useState<number | null>(null);
 const [notice,setNotice] = useState("");
 const path = kind === "content" ? "/v1/admin/study-sets" : "/v1/admin/classes";
 const query = useQuery({queryKey:["admin-catalog",kind,search,offset],queryFn:()=>apiFetch<{studySets?:Row[];classes?:Row[];total:number}>(path,token,{}, {search,limit:25,offset}), enabled:!!token});
 const preview = useQuery({queryKey:["admin-preview",previewId],queryFn:()=>apiFetch<Preview>(`${path}/${previewId}`,token),enabled:!!token && !!previewId && kind === "content",staleTime:0});
 const moderation = useMutation({mutationFn:({row,reason}: {row: Row;reason:string})=>apiFetch(`${path}/${row.id}/moderation`,token,{method:"POST",body:JSON.stringify({hidden:!row.hidden,reason})}),onSuccess:()=>{void qc.invalidateQueries({queryKey:["admin-catalog"]});setNotice("Đã cập nhật trạng thái nội dung");},onError:(e)=>setNotice(e instanceof Error?e.message:"Thao tác thất bại")});
 const columns: Column<Row>[] = [{key:"id",header:"ID"},{key:"title",header:kind==="content"?"Học phần":"Lớp",render:r=>r.title??r.name},{key:"ownerUserId",header:"Chủ sở hữu"},kind==="content"?{key:"cardCount",header:"Nội dung",render:r=>`${r.cardCount??0} thẻ · ${r.quizCount??0} câu hỏi`}:{key:"memberCount",header:"Sĩ số"}];
 if(kind==="classes") columns.push({key:"actions",header:"Tiến độ",render:row=><a className="ghost-button" href={`/classes/${row.id}/dashboard`}>Xem dashboard</a>});
 if(kind==="content") columns.push({key:"actions",header:"Kiểm duyệt",render:row=><div className="flex gap-2"><button className="ghost-button" onClick={()=>setPreviewId(row.id)}>Xem</button><button disabled={moderation.isPending} className="danger-button" onClick={()=>{const reason=window.prompt(row.hidden?"Lý do khôi phục nội dung":"Lý do ẩn nội dung");if(reason?.trim())moderation.mutate({row,reason:reason.trim()});}}>{row.hidden?"Khôi phục":"Ẩn"}</button></div>});
 return <div className="space-y-4"><h1>{kind==="content"?"Nội dung":"Lớp học"}</h1><input aria-label="Tìm kiếm" type="search" value={search} onChange={e=>{setSearch(e.target.value);setOffset(0);}} placeholder="Tìm theo tên" />{notice&&<p role="status">{notice}</p>}{query.isError&&<div role="alert">Không tải được dữ liệu. <button onClick={()=>void query.refetch()}>Thử lại</button></div>}<SectionCard title={`${query.data?.total??0} kết quả`}><DataTable columns={columns} rows={query.data?.studySets??query.data?.classes??[]} rowKey={r=>r.id} loading={query.isLoading} empty={<EmptyState title="Chưa có dữ liệu phù hợp" />} /><div className="flex gap-2"><button disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-25))}>Trước</button><button disabled={offset+25>=(query.data?.total??0)} onClick={()=>setOffset(offset+25)}>Sau</button></div></SectionCard>{previewId&&<SectionCard title={preview.data?.studySet.title??"Xem nội dung"} actions={<button onClick={()=>setPreviewId(null)}>Đóng</button>}>{preview.isLoading?<p>Đang tải…</p>:preview.isError?<p role="alert">Không tải được nội dung</p>:<><p>{preview.data?.studySet.description}</p><ul>{preview.data?.flashcards.map((f,i)=><li key={i}><strong>{f.term}</strong>: {f.definition}</li>)}</ul><ol>{preview.data?.questions.map((q,i)=><li key={i}>{q.question_text??q.text}</li>)}</ol></>}</SectionCard>}</div>;
}
