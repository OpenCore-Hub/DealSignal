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

headers = {}
referer = os.getenv("OPENAI_REFERER")
if referer:
    headers["HTTP-Referer"] = referer

app_title = os.getenv("OPENAI_APP_TITLE")
if app_title:
    headers["X-Title"] = app_title

if headers:
    client_kwargs["default_headers"] = headers

client = OpenAI(**client_kwargs)
embedding_model = os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")

texts = [
    "待测文本1...",
    "待测文本2...",
    # ...
]

for i, t in enumerate(texts):
    try:
        r = client.embeddings.create(model=embedding_model, input=t)
        print(f"[{i}] ✅ 通过")
    except Exception as e:
        print(f"[{i}] ❌ 被拒: {e}")
        # 把违规文本保存下来
        with open(api_root / "tests" / "blocked_openai.txt", "a", encoding="utf-8") as f:
            f.write(f"{i}: {t}\n---\n")
