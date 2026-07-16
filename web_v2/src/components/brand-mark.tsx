import { cn } from "@/lib/utils";

export function BrandMark({ className }: { className?: string }) {
  return (
    <div className={cn("grid size-9 place-items-center rounded-[10px] bg-neutral-950 text-white", className)} aria-hidden>
      <div className="size-4 rounded-full border-[3px] border-white" />
    </div>
  );
}
