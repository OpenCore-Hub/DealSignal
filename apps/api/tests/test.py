import os
from pathlib import Path

from openai import OpenAI


def load_env_file(path: Path) -> None:
    if not path.exists():
        return

    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue

        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        os.environ.setdefault(key, value)


api_root = Path(__file__).resolve().parents[1]
load_env_file(api_root / ".env")

api_key = os.getenv("OPENAI_API_KEY") or os.getenv("OPENROUTER_API_KEY")
if not api_key:
    raise SystemExit(
        "Missing OPENAI_API_KEY. Set it in apps/api/.env or export it before running this script."
    )

client_kwargs = {
    "api_key": api_key,
}

base_url = os.getenv("OPENAI_BASE_URL")
if base_url:
    client_kwargs["base_url"] = base_url

client = OpenAI(**client_kwargs)
chat_model = os.getenv("OPENAI_CHAT_MODEL", "gpt-4o-mini")

prompts = [
    "Reply with the single word ok.",
    "What is 2+2? Reply with only the number.",
]

for i, prompt in enumerate(prompts):
    try:
        r = client.chat.completions.create(
            model=chat_model,
            messages=[{"role": "user", "content": prompt}],
        )
        content = (r.choices[0].message.content or "").strip()
        print(f"[{i}] OK: {content!r}")
    except Exception as e:
        print(f"[{i}] FAIL: {e}")
        with open(api_root / "tests" / "blocked_openai.txt", "a", encoding="utf-8") as f:
            f.write(f"{i}: {prompt}\n---\n")
