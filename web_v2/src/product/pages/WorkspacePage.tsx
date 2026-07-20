"use client";

import { useSelector } from "@tanstack/react-store";
import { Check, Clipboard, Loader2, UserPlus, Users } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";

import { appStore } from "../app-store";
import {
  createWorkspaceInvitation,
  deleteWorkspaceInvitation,
  deleteWorkspaceMember,
  listWorkspaceInvitations,
  listWorkspaceMembers,
  updateWorkspaceMember,
  type WorkspaceInvitation,
  type WorkspaceMember,
} from "../api";
import { DataTable, PageHeader } from "../components/dashboard";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";

export function WorkspacePage() {
  const user = useSelector(appStore, (state) => state.auth.session?.user);
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [invitations, setInvitations] = useState<WorkspaceInvitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [inviteLink, setInviteLink] = useState("");
  const [copied, setCopied] = useState(false);
  const canManage = user?.role === "owner" || user?.role === "admin";

  const load = useCallback(async () => {
    if (!canManage) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [memberResponse, invitationResponse] = await Promise.all([listWorkspaceMembers(), listWorkspaceInvitations()]);
      setMembers(memberResponse.items);
      setInvitations(invitationResponse.items);
    } catch (err) {
      setError(message(err, "Unable to load workspace access"));
    } finally {
      setLoading(false);
    }
  }, [canManage]);

  useEffect(() => { void load(); }, [load, user?.workspace_id]);

  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setSaving(true);
    setError("");
    try {
      const invitation = await createWorkspaceInvitation({ email: String(form.get("email") || ""), role: String(form.get("role") || "viewer") as "admin" | "viewer" });
      setInvitations((items) => [invitation, ...items]);
      setInviteLink(`${window.location.origin}/invite?token=${encodeURIComponent(invitation.token || "")}`);
      event.currentTarget.reset();
    } catch (err) {
      setError(message(err, "Unable to create invitation"));
    } finally {
      setSaving(false);
    }
  }

  async function changeRole(member: WorkspaceMember, role: WorkspaceMember["role"]) {
    setError("");
    try {
      await updateWorkspaceMember(member.user_id, role);
      setMembers((items) => items.map((item) => item.user_id === member.user_id ? { ...item, role } : item));
    } catch (err) { setError(message(err, "Unable to update member")); }
  }

  async function removeMember(member: WorkspaceMember) {
    if (!window.confirm(`Remove ${member.email} from this workspace?`)) return;
    setError("");
    try {
      await deleteWorkspaceMember(member.user_id);
      setMembers((items) => items.filter((item) => item.user_id !== member.user_id));
    } catch (err) { setError(message(err, "Unable to remove member")); }
  }

  async function revokeInvitation(invitation: WorkspaceInvitation) {
    setError("");
    try {
      await deleteWorkspaceInvitation(invitation.id);
      setInvitations((items) => items.map((item) => item.id === invitation.id ? { ...item, status: "revoked" } : item));
    } catch (err) { setError(message(err, "Unable to revoke invitation")); }
  }

  return (
    <>
      <PageHeader action={null} description="Manage who can view and change metering data in this workspace." eyebrow="Access" icon={<Users />} title="Workspace access" />
      <section className="page-content grid gap-4">
        {error ? <div className="rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive" role="alert">{error}</div> : null}
        {!canManage ? (
          <Card><CardHeader><CardTitle>Viewer access</CardTitle><CardDescription>Only workspace owners and admins can view members and invitations.</CardDescription></CardHeader></Card>
        ) : (
          <>
            <Card>
              <CardHeader><CardTitle>Invite a teammate</CardTitle><CardDescription>Create a secure, single-use link valid for seven days. Email delivery is not required.</CardDescription></CardHeader>
              <CardContent>
                <form className="grid gap-3 md:grid-cols-[minmax(220px,1fr)_160px_auto] md:items-end" onSubmit={(event) => void invite(event)}>
                  <Label className="grid gap-1.5">Email<Input name="email" placeholder="teammate@example.com" required type="email" /></Label>
                  <Label className="grid gap-1.5">Invite role<select className="h-9 rounded-md border bg-background px-3 text-sm" defaultValue="viewer" name="role"><option value="viewer">Viewer</option><option value="admin">Admin</option></select></Label>
                  <Button disabled={saving} type="submit">{saving ? <Loader2 className="animate-spin" /> : <UserPlus />} Create invite</Button>
                </form>
                {inviteLink ? (
                  <div className="mt-4 grid gap-2 rounded-md border bg-muted/40 p-3">
                    <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Copy this link now</span>
                    <div className="flex gap-2"><Input aria-label="Invitation link" readOnly value={inviteLink} /><Button onClick={() => void navigator.clipboard.writeText(inviteLink).then(() => { setCopied(true); window.setTimeout(() => setCopied(false), 1500); })} type="button" variant="outline">{copied ? <Check /> : <Clipboard />} {copied ? "Copied" : "Copy"}</Button></div>
                  </div>
                ) : null}
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle>Members</CardTitle><CardDescription>Viewers are read-only. Admins can manage product data and invitations.</CardDescription></CardHeader>
              <CardContent>
                {loading ? <Loading /> : <DataTable emptyLabel="No workspace members" headers={["Member", "Role", "Joined", "Actions"]} rows={members.map((member) => [
                  <div key="member"><strong className="block text-sm">{member.email}</strong>{member.user_id === user?.id ? <span className="text-xs text-muted-foreground">You</span> : null}</div>,
                  user?.role === "owner" ? <select aria-label={`Role for ${member.email}`} className="h-8 rounded-md border bg-background px-2 text-xs capitalize" key="role" onChange={(event) => void changeRole(member, event.target.value as WorkspaceMember["role"])} value={member.role}><option value="owner">Owner</option><option value="admin">Admin</option><option value="viewer">Viewer</option></select> : <Badge className="capitalize" key="role" variant="muted">{member.role}</Badge>,
                  new Date(member.created_at).toLocaleDateString(),
                  user?.role === "owner" && member.user_id !== user.id ? <Button key="remove" onClick={() => void removeMember(member)} size="sm" variant="outline">Remove</Button> : <span className="text-xs text-muted-foreground" key="none">—</span>,
                ])} />}
              </CardContent>
            </Card>

            <Card>
              <CardHeader><CardTitle>Invitations</CardTitle><CardDescription>Expired, accepted, and revoked invitations remain visible for audit context.</CardDescription></CardHeader>
              <CardContent>
                {loading ? <Loading /> : <DataTable emptyLabel="No invitations" headers={["Email", "Role", "Status", "Expires", "Actions"]} rows={invitations.map((invitation) => [
                  invitation.email,
                  <span className="capitalize" key="role">{invitation.role}</span>,
                  <Badge className="capitalize" key="status" variant={invitation.status === "pending" ? "default" : "muted"}>{invitation.status}</Badge>,
                  new Date(invitation.expires_at).toLocaleString(),
                  invitation.status === "pending" ? <Button key="revoke" onClick={() => void revokeInvitation(invitation)} size="sm" variant="outline">Revoke</Button> : <span className="text-xs text-muted-foreground" key="none">—</span>,
                ])} />}
              </CardContent>
            </Card>
          </>
        )}
      </section>
    </>
  );
}

function Loading() { return <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground"><Loader2 className="size-4 animate-spin" /> Loading…</div>; }
function message(error: unknown, fallback: string) { return error instanceof Error && error.message ? error.message : fallback; }
