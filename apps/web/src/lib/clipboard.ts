import { toast } from "sonner";
import i18next from "i18next";

export async function copyToClipboard(text: string, successMessage?: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMessage || i18next.t("common:copied"));
    return true;
  } catch {
    toast.error(i18next.t("common:error.copyFailed"));
    return false;
  }
}
