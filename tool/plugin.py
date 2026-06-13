"""Hermes Remote plugin — registers 5 tools for remote agent control.

Tools registered:
  - remote_agent_list   : GET  /api/agents
  - remote_shell         : POST /api/agent/{id}/shell
  - remote_fs_read       : POST /api/agent/{id}/fs-read
  - remote_fs_write      : POST /api/agent/{id}/fs-write
  - remote_screenshot    : POST /api/agent/{id}/screenshot

Environment:
  HERMES_REMOTE_TOKEN   : bearer token (required)
  HERMES_REMOTE_URL     : server base URL (default http://localhost:7700)
"""

from __future__ import annotations

import json
import os
from typing import Any

import requests


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _base_url() -> str:
    return os.environ.get("HERMES_REMOTE_URL", "http://localhost:7700")


def _token() -> str:
    return os.environ.get("HERMES_REMOTE_TOKEN", "")


def _req(method: str, path: str, json_body: dict | None = None) -> dict:
    """Make an HTTP request to the hermes-remote server.  Returns a dict
    of the form {ok, status, body} for downstream formatting."""
    url = f"{_base_url().rstrip('/')}{path}"
    headers = {}
    token = _token()
    if token:
        headers["Authorization"] = f"Bearer {token}"
    try:
        resp = requests.request(method, url, json=json_body, headers=headers, timeout=120)
        try:
            body = resp.json()
        except Exception:
            body = resp.text
        return {"ok": resp.ok, "status": resp.status_code, "body": body}
    except requests.RequestException as exc:
        return {"ok": False, "status": 0, "body": str(exc)}


def _format_result(r: dict) -> str:
    if r["ok"]:
        payload = r["body"]
        if isinstance(payload, str):
            return payload
        return json.dumps(payload)
    status = r["status"]
    body = r["body"]
    return f"HTTP {status}: {body}"


def _check_available() -> bool:
    """Return True when the hermes-remote server is reachable."""
    try:
        requests.get(f"{_base_url().rstrip('/')}/api/agents", timeout=5)
        return True
    except Exception:
        return False


# ---------------------------------------------------------------------------
# Handlers
# ---------------------------------------------------------------------------

def _handle_agent_list(args: dict, **kw) -> str:
    """List all registered agents."""
    r = _req("GET", "/api/agents")
    return _format_result(r)


def _handle_shell(args: dict, **kw) -> str:
    """Execute a shell command on a remote agent."""
    agent_id = str(args["agent_id"])
    command = str(args["command"])
    timeout = int(args.get("timeout", 60))
    if timeout <= 0:
        timeout = 60
    r = _req("POST", f"/api/agent/{agent_id}/shell", {"command": command, "timeout": timeout})
    return _format_result(r)


def _handle_fs_read(args: dict, **kw) -> str:
    """Read a file from a remote agent's filesystem."""
    agent_id = str(args["agent_id"])
    path = str(args["path"])
    offset = int(args.get("offset", 0))
    limit = int(args.get("limit", 1024))
    r = _req("POST", f"/api/agent/{agent_id}/fs-read", {"path": path, "offset": offset, "limit": limit})
    return _format_result(r)


def _handle_fs_write(args: dict, **kw) -> str:
    """Write data to a remote agent's filesystem."""
    agent_id = str(args["agent_id"])
    path = str(args["path"])
    data = str(args["data"])
    mode = str(args.get("mode", "0644"))
    r = _req("POST", f"/api/agent/{agent_id}/fs-write", {"path": path, "data": data, "mode": mode})
    return _format_result(r)


def _handle_screenshot(args: dict, **kw) -> str:
    """Capture a screenshot from a remote agent."""
    agent_id = str(args["agent_id"])
    display = str(args.get("display", ":0"))
    quality = int(args.get("quality", 80))
    r = _req("POST", f"/api/agent/{agent_id}/screenshot", {"display": display, "quality": quality})
    return _format_result(r)


# ---------------------------------------------------------------------------
# Tool schemas
# ---------------------------------------------------------------------------

AGENT_LIST_SCHEMA = {
    "name": "remote_agent_list",
    "description": "List all registered remote agents from the hermes-remote server.",
    "parameters": {
        "type": "object",
        "properties": {},
        "required": [],
    },
}

SHELL_SCHEMA = {
    "name": "remote_shell",
    "description": "Execute a shell command on a remote agent and return stdout, stderr, and exit code.",
    "parameters": {
        "type": "object",
        "properties": {
            "agent_id": {"type": "string", "description": "ID of the remote agent to execute the command on."},
            "command": {"type": "string", "description": "Shell command to execute."},
            "timeout": {"type": "integer", "description": "Timeout in seconds (default 60)."},
        },
        "required": ["agent_id", "command"],
    },
}

FS_READ_SCHEMA = {
    "name": "remote_fs_read",
    "description": "Read a file from a remote agent's filesystem. Returns base64-encoded data.",
    "parameters": {
        "type": "object",
        "properties": {
            "agent_id": {"type": "string", "description": "ID of the remote agent."},
            "path": {"type": "string", "description": "Absolute path to the file on the remote agent."},
            "offset": {"type": "integer", "description": "Byte offset to start reading from (default 0)."},
            "limit": {"type": "integer", "description": "Maximum bytes to read (default 1024)."},
        },
        "required": ["agent_id", "path"],
    },
}

FS_WRITE_SCHEMA = {
    "name": "remote_fs_write",
    "description": "Write base64-encoded data to a file on a remote agent's filesystem.",
    "parameters": {
        "type": "object",
        "properties": {
            "agent_id": {"type": "string", "description": "ID of the remote agent."},
            "path": {"type": "string", "description": "Absolute path to write to on the remote agent."},
            "data": {"type": "string", "description": "Base64-encoded data to write."},
            "mode": {"type": "string", "description": "Unix file permission mode (default '0644')."},
        },
        "required": ["agent_id", "path", "data"],
    },
}

SCREENSHOT_SCHEMA = {
    "name": "remote_screenshot",
    "description": "Capture a screenshot from a remote agent's display.",
    "parameters": {
        "type": "object",
        "properties": {
            "agent_id": {"type": "string", "description": "ID of the remote agent."},
            "display": {"type": "string", "description": "X11 display to capture (default ':0')."},
            "quality": {"type": "integer", "description": "Image quality 1-100 (default 80)."},
        },
        "required": ["agent_id"],
    },
}


# ---------------------------------------------------------------------------
# Plugin registration entry point
# ---------------------------------------------------------------------------

def register(ctx) -> None:
    """Register all 5 hermes-remote tools. Called by the plugin loader."""
    tools = (
        ("remote_agent_list", AGENT_LIST_SCHEMA, _handle_agent_list, "📋"),
        ("remote_shell", SHELL_SCHEMA, _handle_shell, "💻"),
        ("remote_fs_read", FS_READ_SCHEMA, _handle_fs_read, "📖"),
        ("remote_fs_write", FS_WRITE_SCHEMA, _handle_fs_write, "📝"),
        ("remote_screenshot", SCREENSHOT_SCHEMA, _handle_screenshot, "📸"),
    )

    for name, schema, handler, emoji in tools:
        ctx.register_tool(
            name=name,
            toolset="hermes-remote",
            schema=schema,
            handler=handler,
            check_fn=_check_available,
            emoji=emoji,
        )
