import { toast } from "sonner";

export async function copyToClipboard(text: string, description = "已复制到剪贴板"): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(description);
    return true;
  } catch {
    toast.error("复制失败，请手动复制");
    return false;
  }
}
