"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, Loader2, Mail, ShieldCheck, XCircle } from "lucide-react";
import { Suspense, useEffect, useState } from "react";

import { BrandMark } from "@/components/brand-mark";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/product/components/ui/card";
import { appStore, appStoreActions } from "@/product/app-store";
import { acceptWorkspaceInvitation, previewWorkspaceInvitation, type WorkspaceInvitation } from "@/product/api";

export default function InviteRoute() {
  return <Suspense fallback={<InviteLoading />}><InviteContent /></Suspense>;
}

function InviteContent() {
  const token = useSearchParams().get("token") || "";
  const [invitation, setInvitation] = useState<WorkspaceInvitation | null>(null);
  const [loading, setLoading] = useState(true);
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState("");
  const [signedIn, setSignedIn] = useState(false);

  useEffect(() => {
    void Promise.all([previewWorkspaceInvitation(token), appStoreActions.ensureAuthUser()])
      .then(([nextInvitation, user]) => { setInvitation(nextInvitation); setSignedIn(Boolean(user)); })
      .catch((err) => setError(err instanceof Error ? err.message : "Invitation could not be loaded"))
      .finally(() => setLoading(false));
  }, [token]);

  async function accept() {
    setAccepting(true);
    setError("");
    try {
      await acceptWorkspaceInvitation(token);
      window.location.href = "/overview";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invitation could not be accepted");
      setAccepting(false);
    }
  }

  if (loading) return <InviteLoading />;
  const currentEmail = appStore.state.auth.session?.user.email;
  const returnPath = `/invite?token=${encodeURIComponent(token)}`;

  return (
    <main className="grid min-h-screen place-items-center bg-muted/30 p-4">
      <Card className="w-full max-w-lg">
        <CardHeader className="gap-4">
          <BrandMark />
          <div><CardTitle className="text-xl">Workspace invitation</CardTitle><CardDescription>Join a shared Open Spanner workspace.</CardDescription></div>
        </CardHeader>
        <CardContent className="grid gap-4">
          {error || !invitation ? <State icon={<XCircle />} title="Invitation unavailable" text={error || "This invitation does not exist."} /> : (
            <>
              <div className="grid gap-3 rounded-md border bg-muted/30 p-4">
                <div className="flex items-center gap-3"><ShieldCheck className="size-5 text-primary" /><div><span className="block text-xs text-muted-foreground">Workspace</span><strong>{invitation.workspace_name}</strong></div></div>
                <div className="flex items-center gap-3"><Mail className="size-5 text-primary" /><div><span className="block text-xs text-muted-foreground">Invited account</span><strong>{invitation.email}</strong></div></div>
                <div className="flex items-center gap-3"><CheckCircle2 className="size-5 text-primary" /><div><span className="block text-xs text-muted-foreground">Role</span><strong className="capitalize">{invitation.role}</strong></div></div>
              </div>
              {invitation.status !== "pending" ? <State icon={<XCircle />} title={`Invitation ${invitation.status}`} text="Ask a workspace admin to create a new invitation." /> : !signedIn ? (
                <div className="grid gap-2 sm:grid-cols-2">
                  <Button asChild><Link href={`/login?next=${encodeURIComponent(returnPath)}&email=${encodeURIComponent(invitation.email)}`}>Sign in to accept</Link></Button>
                  <Button asChild variant="outline"><Link href={`/register?next=${encodeURIComponent(returnPath)}&email=${encodeURIComponent(invitation.email)}`}>Create account</Link></Button>
                </div>
              ) : currentEmail !== invitation.email ? (
                <State icon={<XCircle />} title="Wrong signed-in account" text={`Sign out and use ${invitation.email} to accept this invitation.`} />
              ) : (
                <Button disabled={accepting} onClick={() => void accept()}>{accepting ? <Loader2 className="animate-spin" /> : <CheckCircle2 />} Accept invitation</Button>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </main>
  );
}

function InviteLoading() { return <main className="grid min-h-screen place-items-center bg-muted/30"><Loader2 aria-label="Loading invitation" className="size-7 animate-spin text-primary" /></main>; }
function State({ icon, text, title }: { icon: React.ReactNode; text: string; title: string }) { return <div className="flex gap-3 rounded-md border p-4 text-sm"><span className="text-muted-foreground">{icon}</span><div><strong className="block">{title}</strong><span className="text-muted-foreground">{text}</span></div></div>; }
