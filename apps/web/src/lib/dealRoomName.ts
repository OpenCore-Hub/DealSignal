export const DEAL_ROOM_NAME_MIN = 2;
export const DEAL_ROOM_NAME_MAX = 80;
export const DEAL_ROOM_NAME_RAW_MAX = 4096;

export type DealRoomNameIssue = "empty" | "short" | "long" | "format";

const WHITE_SPACE = /^\p{White_Space}$/u;
const CONTROL_OR_FORMAT = /^\p{Cc}$|^\p{Cf}$/u;
const LETTER_OR_NUMBER = /[\p{L}\p{N}]/u;
const FORBIDDEN = /[<>\uFF1C\uFF1E]/;
const LATIN_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const LATIN_SLUG_MAX = 64;

function matches(re: RegExp, ch: string): boolean {
  re.lastIndex = 0;
  return re.test(ch);
}

/** Keep in sync with apps/api/internal/dealroom/name.go */
export function normalizeDealRoomName(value: string): string {
  if (value.length > DEAL_ROOM_NAME_RAW_MAX) {
    return "";
  }
  let out = "";
  let pendingSpace = false;
  for (const ch of value.normalize("NFC")) {
    if (matches(WHITE_SPACE, ch)) {
      pendingSpace = true;
      continue;
    }
    if (matches(CONTROL_OR_FORMAT, ch)) {
      continue;
    }
    if (pendingSpace && out.length > 0) {
      out += " ";
    }
    pendingSpace = false;
    out += ch;
  }
  return out;
}

function runeCount(value: string): number {
  return Array.from(value).length;
}

export function validateDealRoomName(value: string): DealRoomNameIssue | null {
  if (value.length > DEAL_ROOM_NAME_RAW_MAX) return "long";
  const name = normalizeDealRoomName(value);
  if (!name) return "empty";
  const n = runeCount(name);
  if (n < DEAL_ROOM_NAME_MIN) return "short";
  if (n > DEAL_ROOM_NAME_MAX) return "long";
  if (FORBIDDEN.test(name)) return "format";
  LETTER_OR_NUMBER.lastIndex = 0;
  if (!LETTER_OR_NUMBER.test(name)) return "format";
  return null;
}

export function latinSlugFromName(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, LATIN_SLUG_MAX);
}

export function dealRoomSlugFromName(name: string): string {
  const latin = latinSlugFromName(name);
  if (latin.length >= 2 && LATIN_SLUG.test(latin)) {
    return latin;
  }
  const bytes = new Uint8Array(5);
  crypto.getRandomValues(bytes);
  let hex = "";
  for (const byte of bytes) {
    hex += byte.toString(16).padStart(2, "0");
  }
  return `room-${hex}`;
}
