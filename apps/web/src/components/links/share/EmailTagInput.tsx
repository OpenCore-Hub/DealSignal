import { useState, useRef, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface EmailTagInputProps {
  id?: string;
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  hint?: string;
  disabled?: boolean;
}

function normalize(value: string): string {
  const trimmed = value.trim().toLowerCase();
  if (trimmed.startsWith("@")) {
    return `*${trimmed}`;
  }
  return trimmed;
}

function isValid(value: string): boolean {
  if (value.startsWith("*@") && value.length > 2) {
    const domain = value.slice(2);
    return domain.includes(".") && !domain.includes("@");
  }
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

const SEPARATORS = [",", ";", " ", "\n", "\t"];

export function EmailTagInput({
  id,
  values,
  onChange,
  placeholder,
  hint,
  disabled,
}: EmailTagInputProps) {
  const { t } = useTranslation("linkShare");
  const [raw, setRaw] = useState("");
  const [invalid, setInvalid] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  const commit = (input: string) => {
    const parts = input
      .split(new RegExp(`[${SEPARATORS.map((s) => `\\${s}`).join("")}]`))
      .map(normalize)
      .filter(Boolean);

    const next = [...values];
    const nextInvalid: string[] = [];

    for (const part of parts) {
      if (!isValid(part)) {
        nextInvalid.push(part);
        continue;
      }
      if (!next.includes(part)) {
        next.push(part);
      }
    }

    const changed =
      next.length !== values.length || next.some((v, i) => v !== values[i]);
    if (changed) {
      onChange(next);
    }
    setRaw(nextInvalid.join(", "));
    setInvalid(nextInvalid);
  };

  const remove = (value: string) => {
    onChange(values.filter((v) => v !== value));
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      commit(raw);
      return;
    }
    if (e.key === "Backspace" && raw === "" && values.length > 0) {
      e.preventDefault();
      remove(values[values.length - 1]);
    }
  };

  const handleBlur = () => {
    if (raw.trim()) {
      commit(raw);
    }
  };

  return (
    <div className="space-y-2">
      <div
        className={cn(
          "flex min-h-[40px] flex-wrap items-center gap-2 rounded-md border border-input bg-background px-2 py-1.5 focus-within:ring-1 focus-within:ring-ring",
          disabled && "cursor-not-allowed opacity-50",
          invalid.length > 0 && "border-destructive focus-within:ring-destructive"
        )}
        onClick={() => inputRef.current?.focus()}
      >
        {values.map((value) => (
          <span
            key={value}
            className="inline-flex animate-in fade-in zoom-in items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary duration-200"
          >
            {value}
            {!disabled && (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  remove(value);
                }}
                className="rounded-full p-0.5 hover:bg-primary/20"
                aria-label={t("emailTagInput.remove", { value })}
              >
                <X size={12} />
              </button>
            )}
          </span>
        ))}
        <Input
          id={id}
          ref={inputRef}
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={handleBlur}
          placeholder={values.length === 0 ? placeholder : ""}
          disabled={disabled}
          className="h-auto min-w-[120px] flex-1 border-0 bg-transparent px-1 py-1 shadow-none focus-visible:ring-0"
        />
      </div>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      {invalid.length > 0 && (
        <p className="text-xs text-destructive">
          {invalid.join(", ")} — {t("emailTagInput.invalid")}
        </p>
      )}
    </div>
  );
}
