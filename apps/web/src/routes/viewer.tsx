import { CanvasViewer } from "@/components/viewer/CanvasViewer";
import { AIChat } from "@/components/viewer/AIChat";

export function ViewerPage() {
  return (
    <div className="relative flex h-[100dvh] flex-col overflow-hidden">
      <CanvasViewer />
      <AIChat />
    </div>
  );
}
