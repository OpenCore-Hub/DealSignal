import { useReturnTo } from "@/hooks/useReturnTo";
import { BackButton } from "./BackButton";

interface SmartBackButtonProps {
  fallbackTo: string;
  fallbackLabel: string;
  className?: string;
}

export function SmartBackButton({ fallbackTo, fallbackLabel, className }: SmartBackButtonProps) {
  const { to, label } = useReturnTo(fallbackTo, fallbackLabel);
  return <BackButton to={to} label={label} className={className} />;
}
