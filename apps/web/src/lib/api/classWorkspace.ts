import { apiFetch } from './client';
import { gatewayUrl } from './gateway';
export type ClassAction = {
    key: string;
    name: string;
    group: string;
};
export type ClassRole = {
    key: string;
    name: string;
    description: string;
    managed: boolean;
    allow: string[];
    deny: string[];
};
export type AccessMember = {
    name?: string;
    email?: string;
    userId: number;
    legacyRole: string;
    roleKeys: string[];
};
export type ClassAccess = {
    catalog: ClassAction[];
    roles: ClassRole[];
    members: AccessMember[];
};
export const classAccessApi = {
    me: (token: string, id: number) => apiFetch<{
        actions: string[];
    }>(`/v1/classes/${id}/access/me`, token),
    get: (token: string, id: number) => apiFetch<ClassAccess>(`/v1/classes/${id}/access`, token),
    saveRole: (token: string, id: number, role: Pick<ClassRole, 'name' | 'description' | 'allow' | 'deny'>, key?: string) => apiFetch<ClassRole>(`/v1/classes/${id}/access/roles${key ? `/${key}` : ''}`, token, { method: key ? 'PUT' : 'POST', body: JSON.stringify(role) }),
    removeRole: (token: string, id: number, key: string) => apiFetch<void>(`/v1/classes/${id}/access/roles/${key}`, token, { method: 'DELETE' }),
    assign: (token: string, id: number, userId: number, roleKeys: string[]) => apiFetch(`/v1/classes/${id}/access/members/${userId}`, token, { method: 'PUT', body: JSON.stringify({ roleKeys }) }),
};
export type AssignmentFile = {
    id: number;
    name: string;
    contentType: string;
    sizeBytes: number;
};
export type ClassAssignment = {
    id: number;
    title: string;
    instructions: string;
    dueAt: string | null;
    status: 'draft' | 'published';
    createdBy: number;
    createdAt: string;
    updatedAt: string;
    files: AssignmentFile[];
};
export type AssignmentInput = Pick<ClassAssignment, 'title' | 'instructions' | 'dueAt' | 'status'> & {
    deleteFileIds: number[];
};
async function downloadPrivate(token: string, path: string, name: string) {
    const response = await fetch(`${gatewayUrl}${path}`, { headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok)
        throw new Error('Không tải được file. Kiểm tra quyền truy cập và thử lại.');
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = name;
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export const classAssignmentApi = {
    list: (token: string, id: number) => apiFetch<ClassAssignment[]>(`/v1/classes/${id}/assignments`, token),
    save: (token: string, id: number, input: AssignmentInput, files: File[], assignmentId?: number) => {
        const data = new FormData();
        data.append('metadata', JSON.stringify(input));
        files.forEach(file => data.append('files', file));
        return apiFetch<ClassAssignment>(`/v1/classes/${id}/assignments${assignmentId ? `/${assignmentId}` : ''}`, token, { method: assignmentId ? 'PUT' : 'POST', body: data });
    },
    remove: (token: string, id: number, assignmentId: number) => apiFetch<void>(`/v1/classes/${id}/assignments/${assignmentId}`, token, { method: 'DELETE' }),
    download: (token: string, id: number, assignmentId: number, file: AssignmentFile) => downloadPrivate(token, `/v1/classes/${id}/assignments/${assignmentId}/files/${file.id}`, file.name),
};
export const renameClassDocument = (token: string, id: number, docId: number, name: string) => apiFetch<void>(`/v1/classes/${id}/documents/${docId}`, token, { method: 'PUT', body: JSON.stringify({ name }) });
export function effectiveRoleActions(keys: string[], roles: ClassRole[]): Set<string> {
    const selected = roles.filter(r => keys.includes(r.key));
    const denied = new Set(selected.flatMap(r => r.deny));
    return new Set(selected.flatMap(r => r.allow).filter(a => !denied.has(a)));
}
export const setVideoAudience = (token: string, classId: number, id: number, audience: string, viewerIds: number[]) => apiFetch<void>(`/v1/classes/${classId}/videos/${id}/audience`, token, { method: "PUT", body: JSON.stringify({ audience, viewerIds }) });
