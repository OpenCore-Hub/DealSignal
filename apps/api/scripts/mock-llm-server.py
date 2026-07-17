#!/usr/bin/env python3
"""Minimal OpenAI-compatible mock server for local E2E AI flow testing.

Supports:
- POST /v1/embeddings -> returns deterministic 3-dim vectors
- POST /v1/chat/completions -> returns a canned assistant answer
"""
import json
import os
from http.server import HTTPServer, BaseHTTPRequestHandler

EMBEDDING_DIM = int(os.environ.get("EMBEDDING_DIM", "1536"))


def make_vector(text: str):
    """Return a deterministic vector based on the input text."""
    h = hash(text) & 0xFFFFFFFF
    vals = []
    for i in range(EMBEDDING_DIM):
        vals.append(round(((h + i * 7919) % 1000) / 1000.0, 6))
    return vals


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        # Reduce noise; rely on explicit prints if needed.
        pass

    def _json(self, status, body):
        data = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b"{}"
        try:
            payload = json.loads(body)
        except json.JSONDecodeError:
            return self._json(400, {"error": "invalid json"})

        auth = self.headers.get("Authorization", "")
        if not auth.startswith("Bearer "):
            return self._json(401, {"error": "missing authorization"})

        if self.path == "/v1/embeddings":
            inputs = payload.get("input", [])
            if isinstance(inputs, str):
                inputs = [inputs]
            data = []
            for i, text in enumerate(inputs):
                data.append({
                    "object": "embedding",
                    "index": i,
                    "embedding": make_vector(text),
                })
            return self._json(200, {
                "object": "list",
                "data": data,
                "model": payload.get("model", "text-embedding-3-small"),
                "usage": {"prompt_tokens": len(inputs) * 4, "total_tokens": len(inputs) * 4},
            })

        if self.path == "/v1/chat/completions":
            # Some unified gateways route embedding requests through /v1/chat/completions.
            inputs = payload.get("input")
            if inputs is not None:
                if isinstance(inputs, str):
                    inputs = [inputs]
                data = []
                for i, text in enumerate(inputs):
                    data.append({
                        "object": "embedding",
                        "index": i,
                        "embedding": make_vector(text),
                    })
                return self._json(200, {
                    "object": "list",
                    "data": data,
                    "model": payload.get("model", "text-embedding-3-small"),
                    "usage": {"prompt_tokens": len(inputs) * 4, "total_tokens": len(inputs) * 4},
                })
            return self._json(200, {
                "id": "chatcmpl-mock",
                "object": "chat.completion",
                "created": 1234567890,
                "model": payload.get("model", "gpt-4o-mini"),
                "choices": [{
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": "This is a mock answer based on the provided evidence.",
                    },
                    "finish_reason": "stop",
                }],
                "usage": {"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
            })

        return self._json(404, {"error": "unknown endpoint"})

    def do_GET(self):
        if self.path == "/healthz":
            return self._json(200, {"status": "ok"})
        return self._json(404, {"error": "unknown endpoint"})


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8000"))
    server = HTTPServer(("0.0.0.0", port), Handler)
    print(f"mock-llm listening on :{port}")
    server.serve_forever()
