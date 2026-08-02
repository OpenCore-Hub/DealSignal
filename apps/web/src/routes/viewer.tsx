import { CanvasViewer } from "@/components/viewer/CanvasViewer";

export function ViewerPage() {
  return (
    <div className="relative flex h-[100dvh] flex-col overflow-hidden">
      <CanvasViewer />
    </div>
  );
}
