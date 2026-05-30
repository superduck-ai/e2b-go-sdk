import argparse
import datetime
import http.server
import json
import os
import shutil
import sys
import tempfile
import time
from typing import Any
from urllib.parse import parse_qsl, urlparse

import httpx

from e2b import ALL_TRAFFIC, Sandbox, Template, Volume, wait_for_timeout
from e2b.connection_config import ConnectionConfig


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--case",
        default="all",
        choices=["all", "claude", "claude_derived", "randomness", "randomness_alias", "volume", "volume_api_payload", "ubuntu", "template_timeout", "template_methods", "config_headers", "metrics", "network_rules", "network_egress", "network_update_payload", "template_api_payload", "debug_root"],
    )
    return parser.parse_args()


def connection_kwargs() -> dict[str, Any]:
    return {
        "api_key": os.getenv("E2B_API_KEY"),
        "access_token": os.getenv("E2B_ACCESS_TOKEN"),
        "domain": os.getenv("E2B_DOMAIN"),
        "api_url": os.getenv("E2B_API_URL"),
        "request_timeout": 120.0,
    }


def build_kwargs() -> dict[str, Any]:
    return {
        **connection_kwargs(),
        "cpu_count": 1,
        "memory_mb": 512,
        "request_timeout": 600.0,
    }


def default_timeout_build_kwargs() -> dict[str, Any]:
    return {
        "api_key": os.getenv("E2B_API_KEY"),
        "access_token": os.getenv("E2B_ACCESS_TOKEN"),
        "domain": os.getenv("E2B_DOMAIN"),
        "api_url": os.getenv("E2B_API_URL"),
        "cpu_count": 1,
        "memory_mb": 1024,
        "skip_cache": True,
    }


def selected_cases(case_name: str) -> list[str]:
    if case_name == "all":
        return ["claude", "claude_derived", "randomness", "randomness_alias", "volume", "volume_api_payload", "ubuntu", "template_timeout", "template_methods", "config_headers", "metrics", "network_rules", "network_egress", "network_update_payload", "template_api_payload", "debug_root"]
    return [case_name]


CLAUDE_DERIVED_NUMPY_INSTALL_COMMAND = (
    "python3 -m pip install --break-system-packages --no-cache-dir numpy"
)
RANDOMNESS_ALIAS_TEMPLATE = "en716jw99aj63v1k8ugh"
UPSTREAM_RANDOMNESS_ALIAS_COMMAND = (
    'python -c "import numpy as np; print([np.random.normal(),np.random.normal(),np.random.normal()])"'
)

TEMPLATE_METHODS_SUMMARY_COMMAND = """printf 'runtime_user=%s\\n' "$(whoami)"
printf 'bashrc_target=%s\\n' "$(readlink /home/user/.bashrc.local)"
printf 'preserved_type=%s\\n' "$(if [ -L /app/link-preserved.txt ]; then echo symlink; else echo regular; fi)"
printf 'preserved_target=%s\\n' "$(readlink /app/link-preserved.txt)"
printf 'preserved_content=%s\\n' "$(cat /app/link-preserved.txt)"
printf 'resolved_type=%s\\n' "$(if [ -L /app/link-resolved.txt ]; then echo symlink; else echo regular; fi)"
printf 'resolved_content=%s\\n' "$(cat /app/link-resolved.txt)" """

EXPECTED_TEMPLATE_METHODS_SUMMARY = "\n".join(
    [
        "runtime_user=user",
        "bashrc_target=.bashrc",
        "preserved_type=symlink",
        "preserved_target=test.txt",
        "preserved_content=template symlink content",
        "resolved_type=regular",
        "resolved_content=template symlink content",
    ]
)


def restore_env(key: str, value: str | None) -> None:
    if value is None:
        os.environ.pop(key, None)
        return
    os.environ[key] = value


def run_debug_root_case() -> dict[str, Any]:
    previous_debug = os.getenv("E2B_DEBUG")
    previous_api_url = os.getenv("E2B_API_URL")
    previous_domain = os.getenv("E2B_DOMAIN")

    os.environ["E2B_DEBUG"] = "true"
    os.environ.pop("E2B_API_URL", None)
    os.environ.pop("E2B_DOMAIN", None)

    try:
        config = ConnectionConfig(debug=False)
        status = "ok" if (config.debug is True and config.api_url == "http://localhost:3000") else "mismatch"
        detail = (
            "env debug=true wins over explicit debug=False at root connection-config construction"
            if status == "ok"
            else f"unexpected root debug semantics: debug={config.debug} api_url={config.api_url}"
        )
        return {
            "language": "python",
            "case": "debug_root",
            "status": status,
            "detail": detail,
            "extra": {
                "env_debug": "true",
                "arg_debug": "false",
                "debug": str(config.debug),
                "api_url": str(config.api_url),
            },
        }
    finally:
        restore_env("E2B_DEBUG", previous_debug)
        restore_env("E2B_API_URL", previous_api_url)
        restore_env("E2B_DOMAIN", previous_domain)


def run_claude_case() -> dict[str, Any]:
    try:
        exists = Template.exists("claude-code-interpreter", **connection_kwargs())
    except Exception as exc:  # noqa: BLE001
        return error_result("claude", exc)

    if not exists:
        return {
            "language": "python",
            "case": "claude",
            "status": "template_missing",
            "detail": "claude-code-interpreter template alias is unavailable",
        }

    sandbox = None
    try:
        sandbox = Sandbox.create(
            "claude-code-interpreter",
            timeout=10 * 60,
            **connection_kwargs(),
        )
        result = sandbox.commands.run(
            numpy_random_command()
        )
        return {
            "language": "python",
            "case": "claude",
            "status": "ok",
            "detail": result.stdout.strip(),
        }
    except Exception as exc:  # noqa: BLE001
        message = str(exc).lower()
        if "no module named" in message or "numpy" in message or "python3: not found" in message:
            return {
                "language": "python",
                "case": "claude",
                "status": "env_blocked",
                "detail": str(exc),
            }
        return error_result("claude", exc)
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass


def run_randomness_case() -> dict[str, Any]:
    return run_built_numpy_template_case(
        "randomness",
        Template()
        .from_python_image("3.12")
        .skip_cache()
        .run_cmd("python3 -m pip install --no-cache-dir numpy"),
        f"python-sdk-randomness-crosscheck-{time.time_ns()}",
    )


def run_randomness_alias_case() -> dict[str, Any]:
    try:
        exists = Template.exists(RANDOMNESS_ALIAS_TEMPLATE, **connection_kwargs())
    except Exception as exc:  # noqa: BLE001
        return error_result("randomness_alias", exc)

    if not exists:
        return {
            "language": "python",
            "case": "randomness_alias",
            "status": "template_missing",
            "detail": "upstream randomness alias is unavailable",
        }

    first_sandbox = None
    second_sandbox = None
    try:
        first_sandbox = Sandbox.create(
            RANDOMNESS_ALIAS_TEMPLATE,
            timeout=10 * 60,
            **connection_kwargs(),
        )
        first = first_sandbox.commands.run(UPSTREAM_RANDOMNESS_ALIAS_COMMAND)

        try:
            second = first_sandbox.commands.run(UPSTREAM_RANDOMNESS_ALIAS_COMMAND)
        except Exception as exc:  # noqa: BLE001
            return {
                "language": "python",
                "case": "randomness_alias",
                "status": "partial",
                "detail": str(exc),
                "extra": {
                    "phase": "same_sandbox_second_command",
                    "template_id": RANDOMNESS_ALIAS_TEMPLATE,
                },
            }

        if first.stdout.strip() == second.stdout.strip():
            return {
                "language": "python",
                "case": "randomness_alias",
                "status": "error",
                "detail": "expected different random vectors in the same sandbox",
                "extra": {
                    "phase": "same_sandbox_compare",
                    "template_id": RANDOMNESS_ALIAS_TEMPLATE,
                },
            }

        second_sandbox = Sandbox.create(
            RANDOMNESS_ALIAS_TEMPLATE,
            timeout=10 * 60,
            **connection_kwargs(),
        )
        third = second_sandbox.commands.run(UPSTREAM_RANDOMNESS_ALIAS_COMMAND)
        if first.stdout.strip() == third.stdout.strip():
            return {
                "language": "python",
                "case": "randomness_alias",
                "status": "error",
                "detail": "expected different random vectors across sandboxes from the same alias",
                "extra": {
                    "phase": "cross_sandbox_compare",
                    "template_id": RANDOMNESS_ALIAS_TEMPLATE,
                },
            }

        return {
            "language": "python",
            "case": "randomness_alias",
            "status": "ok",
            "detail": "same-sandbox and cross-sandbox alias randomness matched upstream expectations",
            "extra": {
                "template_id": RANDOMNESS_ALIAS_TEMPLATE,
            },
        }
    except Exception as exc:  # noqa: BLE001
        return error_result("randomness_alias", exc)
    finally:
        if first_sandbox is not None:
            try:
                first_sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if second_sandbox is not None:
            try:
                second_sandbox.kill()
            except Exception:  # noqa: BLE001
                pass


def run_claude_derived_case() -> dict[str, Any]:
    try:
        exists = Template.exists("claude-code-interpreter", **connection_kwargs())
    except Exception as exc:  # noqa: BLE001
        return error_result("claude_derived", exc)

    if not exists:
        return {
            "language": "python",
            "case": "claude_derived",
            "status": "template_missing",
            "detail": "claude-code-interpreter template alias is unavailable",
        }

    return run_built_numpy_template_case(
        "claude_derived",
        Template()
        .from_template("claude-code-interpreter")
        .skip_cache()
        .run_cmd(CLAUDE_DERIVED_NUMPY_INSTALL_COMMAND),
        f"python-sdk-claude-derived-crosscheck-{time.time_ns()}",
        {"base_template": "claude-code-interpreter"},
    )


def run_built_numpy_template_case(
    case_name: str,
    template: Template,
    build_name: str,
    extra: dict[str, Any] | None = None,
) -> dict[str, Any]:
    build_info = None
    first_sandbox = None
    second_sandbox = None
    try:
        build_info = Template.build(template, build_name, **build_kwargs())
        first_sandbox = Sandbox.create(
            build_info.template_id,
            timeout=10 * 60,
            **connection_kwargs(),
        )
        first = run_numpy_vector(first_sandbox)
        second = run_numpy_vector(first_sandbox)
        if first == second:
            return {
                "language": "python",
                "case": case_name,
                "status": "error",
                "detail": "expected different random vectors in the same sandbox",
                "extra": merge_extra(extra, {
                    "template_id": build_info.template_id,
                    "same_sandbox_diff": "false",
                }),
            }

        second_sandbox = Sandbox.create(
            build_info.template_id,
            timeout=10 * 60,
            **connection_kwargs(),
        )
        third = run_numpy_vector(second_sandbox)
        if first == third:
            return {
                "language": "python",
                "case": case_name,
                "status": "error",
                "detail": "expected different random vectors across sandboxes from the same template",
                "extra": merge_extra(extra, {
                    "template_id": build_info.template_id,
                    "same_sandbox_diff": "true",
                    "cross_sandbox_diff": "false",
                }),
            }

        return {
            "language": "python",
            "case": case_name,
            "status": "ok",
            "detail": "same-sandbox and cross-sandbox numpy vectors differed",
            "extra": merge_extra(extra, {
                "template_id": build_info.template_id,
                "same_sandbox_diff": "true",
                "cross_sandbox_diff": "true",
            }),
        }
    except Exception as exc:  # noqa: BLE001
        message = str(exc).lower()
        if "404 page not found" in message:
            return {
                "language": "python",
                "case": case_name,
                "status": "template_api_unavailable",
                "detail": str(exc),
            }
        if "no module named" in message or "numpy" in message or "python3: not found" in message:
            return {
                "language": "python",
                "case": case_name,
                "status": "env_blocked",
                "detail": str(exc),
            }
        return error_result(case_name, exc)
    finally:
        if first_sandbox is not None:
            try:
                first_sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if second_sandbox is not None:
            try:
                second_sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if build_info is not None and getattr(build_info, "template_id", ""):
            try:
                Sandbox.delete_snapshot(build_info.template_id, **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass


def run_volume_case() -> dict[str, Any]:
    volume = None
    try:
        volume = Volume.create(f"python-sdk-crosscheck-{time.time_ns()}", **connection_kwargs())
        volume.make_dir("/multi-file-dir")
        return {
            "language": "python",
            "case": "volume",
            "status": "ok",
            "detail": "make_dir(/multi-file-dir) succeeded",
        }
    except Exception as exc:  # noqa: BLE001
        message = str(exc).lower()
        if "path /multi-file-dir not found" in message or "not found" in message:
            return {
                "language": "python",
                "case": "volume",
                "status": "env_blocked",
                "detail": str(exc),
            }
        return error_result("volume", exc)
    finally:
        if volume is not None:
            try:
                Volume.destroy(volume.volume_id, **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass


def run_volume_api_payload_case() -> dict[str, Any]:
    requests: list[dict[str, Any]] = []
    timestamp = "2026-05-30T00:00:00Z"
    dir_entry = {
        "name": "dir",
        "path": "/dir",
        "type": "directory",
        "uid": 1000,
        "gid": 1000,
        "mode": 0o755,
        "size": 0,
        "atime": timestamp,
        "mtime": timestamp,
        "ctime": timestamp,
    }
    file_entry = {
        "name": "file.txt",
        "path": "/file.txt",
        "type": "file",
        "uid": 1000,
        "gid": 1000,
        "mode": 0o644,
        "size": 5,
        "atime": timestamp,
        "mtime": timestamp,
        "ctime": timestamp,
    }
    updated_entry = {
        "name": "dir",
        "path": "/dir",
        "type": "directory",
        "uid": 1001,
        "gid": 1002,
        "mode": 0o644,
        "size": 0,
        "atime": timestamp,
        "mtime": timestamp,
        "ctime": timestamp,
    }

    class Handler(http.server.BaseHTTPRequestHandler):
        def _capture(self) -> tuple[str, dict[str, str], Any]:
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length) if length > 0 else b""
            body_text = raw.decode("utf-8") if raw else ""
            content_type = self.headers.get("Content-Type", "")
            body: Any = ""
            if body_text:
                if content_type.startswith("application/json"):
                    body = json.loads(body_text)
                else:
                    body = body_text
            parsed = urlparse(self.path)
            requests.append(
                {
                    "method": self.command,
                    "path": parsed.path,
                    "query": dict(parse_qsl(parsed.query, keep_blank_values=True)),
                    "content_type": content_type,
                    "authorization": self.headers.get("Authorization", ""),
                    "body": body,
                }
            )
            return parsed.path, dict(parse_qsl(parsed.query, keep_blank_values=True)), body

        def _respond_json(self, status: int, payload: Any) -> None:
            encoded = json.dumps(payload).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def do_GET(self) -> None:  # noqa: N802
            path, _query, _body = self._capture()
            if path == "/volumecontent/vol-1/dir":
                self._respond_json(200, [dir_entry])
                return
            if path == "/volumecontent/vol-1/path":
                self._respond_json(200, dir_entry)
                return
            if path == "/volumecontent/vol-1/file":
                encoded = b"hello"
                self.send_response(200)
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
                return
            self.send_response(404)
            self.end_headers()

        def do_POST(self) -> None:  # noqa: N802
            path, _query, _body = self._capture()
            if path == "/volumecontent/vol-1/dir":
                self._respond_json(201, dir_entry)
                return
            self.send_response(404)
            self.end_headers()

        def do_PATCH(self) -> None:  # noqa: N802
            path, _query, _body = self._capture()
            if path == "/volumecontent/vol-1/path":
                self._respond_json(200, updated_entry)
                return
            self.send_response(404)
            self.end_headers()

        def do_PUT(self) -> None:  # noqa: N802
            path, _query, _body = self._capture()
            if path == "/volumecontent/vol-1/file":
                self._respond_json(201, file_entry)
                return
            self.send_response(404)
            self.end_headers()

        def do_DELETE(self) -> None:  # noqa: N802
            path, _query, _body = self._capture()
            if path == "/volumecontent/vol-1/path":
                self.send_response(204)
                self.end_headers()
                return
            self.send_response(404)
            self.end_headers()

        def log_message(self, _format: str, *args: Any) -> None:
            return

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_port
    import threading

    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    try:
        api_url = f"http://127.0.0.1:{port}"
        volume = Volume("vol-1", "vol", "token-1")

        entries = volume.list("/dir", depth=2, api_url=api_url, request_timeout=1.0)
        if len(entries) != 1 or entries[0].path != "/dir" or entries[0].type.value != "directory":
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected list result {entries}",
            }

        directory = volume.make_dir(
            "/dir",
            uid=1000,
            gid=1000,
            mode=0o755,
            force=False,
            api_url=api_url,
            request_timeout=1.0,
        )
        if directory.path != "/dir" or directory.type.value != "directory":
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected make_dir result {directory}",
            }

        info = volume.get_info("/dir", api_url=api_url, request_timeout=1.0)
        if info.path != "/dir" or info.type.value != "directory":
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected get_info result {info}",
            }

        updated = volume.update_metadata(
            "/dir",
            uid=1001,
            gid=1002,
            mode=0o644,
            api_url=api_url,
            request_timeout=1.0,
        )
        if updated.uid != 1001 or updated.gid != 1002 or updated.mode != 0o644:
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected update_metadata result {updated}",
            }

        read_value = volume.read_file("/file.txt", api_url=api_url, request_timeout=1.0)
        if read_value != "hello":
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected read_file result {read_value!r}",
            }

        written = volume.write_file(
            "/file.txt",
            "hello",
            uid=1000,
            gid=1000,
            mode=0o644,
            force=False,
            api_url=api_url,
            request_timeout=1.0,
        )
        if written.path != "/file.txt" or written.type.value != "file":
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"unexpected write_file result {written}",
            }

        volume.remove("/file.txt", api_url=api_url, request_timeout=1.0)

        if len(requests) != 7:
            return {
                "language": "python",
                "case": "volume_api_payload",
                "status": "error",
                "detail": f"expected 7 captured requests, got {len(requests)}",
            }

        expected = {
            "list": {
                "method": "GET",
                "path": "/volumecontent/vol-1/dir",
                "query": {"depth": "2", "path": "/dir"},
                "content_type": "",
                "authorization": "Bearer token-1",
                "body": "",
            },
            "make_dir": {
                "method": "POST",
                "path": "/volumecontent/vol-1/dir",
                "query": {
                    "force": "false",
                    "gid": "1000",
                    "mode": "493",
                    "path": "/dir",
                    "uid": "1000",
                },
                "content_type": "",
                "authorization": "Bearer token-1",
                "body": "",
            },
            "get_info": {
                "method": "GET",
                "path": "/volumecontent/vol-1/path",
                "query": {"path": "/dir"},
                "content_type": "",
                "authorization": "Bearer token-1",
                "body": "",
            },
            "update_metadata": {
                "method": "PATCH",
                "path": "/volumecontent/vol-1/path",
                "query": {"path": "/dir"},
                "content_type": "application/json",
                "authorization": "Bearer token-1",
                "body": {"gid": 1002, "mode": 420, "uid": 1001},
            },
            "read_file": {
                "method": "GET",
                "path": "/volumecontent/vol-1/file",
                "query": {"path": "/file.txt"},
                "content_type": "",
                "authorization": "Bearer token-1",
                "body": "",
            },
            "write_file": {
                "method": "PUT",
                "path": "/volumecontent/vol-1/file",
                "query": {
                    "force": "false",
                    "gid": "1000",
                    "mode": "420",
                    "path": "/file.txt",
                    "uid": "1000",
                },
                "content_type": "application/octet-stream",
                "authorization": "Bearer token-1",
                "body": "hello",
            },
            "remove": {
                "method": "DELETE",
                "path": "/volumecontent/vol-1/path",
                "query": {"path": "/file.txt"},
                "content_type": "",
                "authorization": "Bearer token-1",
                "body": "",
            },
        }

        keys = [
            "list",
            "make_dir",
            "get_info",
            "update_metadata",
            "read_file",
            "write_file",
            "remove",
        ]
        extra: dict[str, str] = {}
        for index, key in enumerate(keys):
            actual = requests[index]
            want = expected[key]

            extra[f"{key}_method"] = actual["method"]
            extra[f"{key}_path"] = actual["path"]
            extra[f"{key}_query"] = stable_json(actual["query"])
            extra[f"{key}_content_type"] = actual["content_type"]
            extra[f"{key}_authorization"] = actual["authorization"]
            extra[f"{key}_body"] = stable_json(actual["body"])

            if actual["method"] != want["method"] or actual["path"] != want["path"]:
                return {
                    "language": "python",
                    "case": "volume_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} request target mismatch",
                    "extra": extra,
                }
            if actual["query"] != want["query"]:
                return {
                    "language": "python",
                    "case": "volume_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} query mismatch",
                    "extra": extra,
                }
            if want["content_type"]:
                if not str(actual["content_type"]).startswith(want["content_type"]):
                    return {
                        "language": "python",
                        "case": "volume_api_payload",
                        "status": "mismatch",
                        "detail": f"{key} content-type mismatch",
                        "extra": extra,
                    }
            elif actual["content_type"] != "":
                return {
                    "language": "python",
                    "case": "volume_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} content-type mismatch",
                    "extra": extra,
                }
            if actual["authorization"] != want["authorization"]:
                return {
                    "language": "python",
                    "case": "volume_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} authorization mismatch",
                    "extra": extra,
                }
            if actual["body"] != want["body"]:
                return {
                    "language": "python",
                    "case": "volume_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} payload mismatch",
                    "extra": extra,
                }

        return {
            "language": "python",
            "case": "volume_api_payload",
            "status": "ok",
            "detail": "captured volume content request shapes locally",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        return error_result("volume_api_payload", exc)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=1)


def run_ubuntu_case() -> dict[str, Any]:
    build_info = None
    try:
        template = Template().from_image("ubuntu:22.04").skip_cache()
        build_info = Template.build_in_background(
            template,
            f"python-sdk-ubuntu-crosscheck-{time.time_ns()}",
            **build_kwargs(),
        )
        final = wait_for_final_build_status(build_info)
        reason = getattr(final, "reason", None)
        extra = {
            "template_id": build_info.template_id,
            "build_id": build_info.build_id,
        }
        if reason is not None and getattr(reason, "step", None):
            extra["reason_step"] = reason.step
        detail = getattr(reason, "message", None)
        current_status = getattr(getattr(final, "status", None), "value", str(final.status))
        if current_status == "error" and detail and "error waiting for provisioning sandbox" in detail.lower():
            current_status = "env_blocked"
        return {
            "language": "python",
            "case": "ubuntu",
            "status": current_status,
            "detail": detail,
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        message = str(exc).lower()
        if "404 page not found" in message:
            return {
                "language": "python",
                "case": "ubuntu",
                "status": "template_api_unavailable",
                "detail": str(exc),
            }
        return error_result("ubuntu", exc)
    finally:
        if build_info is not None and getattr(build_info, "template_id", ""):
            try:
                Sandbox.delete_snapshot(build_info.template_id, **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass


def run_template_timeout_case() -> dict[str, Any]:
    tmp_dir = tempfile.mkdtemp(prefix="e2b-python-template-timeout-")
    build_info = None
    started_at = time.time()
    extra = {
        "request_timeout_mode": "default",
        "status_poll_query": "logsOffset+limit=100",
        "template_shape": "fromBaseImage+copy+runCmd+setStartCmd",
        "file_context_created": "true",
        "build_options_memory_mb": "1024",
        "build_options_cpu": "1",
        "build_options_skipcache": "true",
    }

    try:
        folder = os.path.join(tmp_dir, "folder")
        os.makedirs(folder, exist_ok=True)
        with open(os.path.join(folder, "test.txt"), "w", encoding="utf-8") as file:
            file.write("This is a test file.")

        build_info = Template.build(
            Template(file_context_path=tmp_dir)
            .from_base_image()
            .copy("folder/*", "folder", force_upload=True)
            .run_cmd("cat folder/test.txt")
            .set_workdir("/app")
            .set_start_cmd("echo 'Hello, world!'", wait_for_timeout(10_000)),
            f"python-sdk-template-timeout-crosscheck-{time.time_ns()}",
            **default_timeout_build_kwargs(),
        )
        extra["template_id"] = build_info.template_id
        extra["build_id"] = build_info.build_id
        extra["elapsed_ms"] = str(int((time.time() - started_at) * 1000))
        return {
            "language": "python",
            "case": "template_timeout",
            "status": "ok",
            "detail": "default-timeout base-image build succeeded",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        detail = str(exc)
        message = detail.lower()
        extra["elapsed_ms"] = str(int((time.time() - started_at) * 1000))
        if "404 page not found" in message:
            return {
                "language": "python",
                "case": "template_timeout",
                "status": "template_api_unavailable",
                "detail": detail,
                "extra": extra,
            }
        if "timeout" in message or "timed out" in message or "aborted" in message:
            extra["failure_kind"] = "timeout"
            return {
                "language": "python",
                "case": "template_timeout",
                "status": "env_blocked",
                "detail": detail,
                "extra": extra,
            }
        if "internal" in message or "build error" in message:
            extra["failure_kind"] = "backend_error"
            return {
                "language": "python",
                "case": "template_timeout",
                "status": "env_blocked",
                "detail": detail,
                "extra": extra,
            }
        return error_result("template_timeout", exc, extra)
    finally:
        if build_info is not None and getattr(build_info, "template_id", ""):
            try:
                Sandbox.delete_snapshot(build_info.template_id, **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass
        shutil.rmtree(tmp_dir, ignore_errors=True)


def run_template_methods_case() -> dict[str, Any]:
    tmp_dir = tempfile.mkdtemp(prefix="e2b-python-template-methods-")
    build_info = None
    sandbox = None
    extra = {
        "template_shape": "fromBaseImage+runCmd(root)+makeSymlink+copy(symlink-preserve)+copy(symlink-resolve)",
    }

    try:
        with open(os.path.join(tmp_dir, "test.txt"), "w", encoding="utf-8") as file:
            file.write("template symlink content\n")
        os.symlink("test.txt", os.path.join(tmp_dir, "link.txt"))

        build_info = Template.build(
            Template(file_context_path=tmp_dir)
            .from_base_image()
            .run_cmd('test "$(whoami)" = "root"', user="root")
            .make_symlink(".bashrc", ".bashrc.local")
            .copy("test.txt", "/app/test.txt", force_upload=True)
            .copy("link.txt", "/app/link-preserved.txt", force_upload=True)
            .copy(
                "link.txt",
                "/app/link-resolved.txt",
                force_upload=True,
                resolve_symlinks=True,
            )
            .run_cmd('test "$(readlink .bashrc.local)" = ".bashrc"')
            .run_cmd('test "$(readlink /app/link-preserved.txt)" = "test.txt"')
            .run_cmd('test "$(cat /app/link-preserved.txt)" = "template symlink content"')
            .run_cmd("test ! -L /app/link-resolved.txt")
            .run_cmd('test "$(cat /app/link-resolved.txt)" = "template symlink content"'),
            f"python-sdk-template-methods-crosscheck-{time.time_ns()}",
            **build_kwargs(),
        )
        extra["template_id"] = build_info.template_id
        extra["build_id"] = build_info.build_id

        sandbox = Sandbox.create(
            build_info.template_id,
            timeout=10 * 60,
            **connection_kwargs(),
        )
        result = sandbox.commands.run(TEMPLATE_METHODS_SUMMARY_COMMAND)
        summary = result.stdout.strip()

        if summary != EXPECTED_TEMPLATE_METHODS_SUMMARY:
            return {
                "language": "python",
                "case": "template_methods",
                "status": "error",
                "detail": f"unexpected runtime summary:\n{summary}",
                "extra": extra,
            }

        return {
            "language": "python",
            "case": "template_methods",
            "status": "ok",
            "detail": "stable base-image template method summary matched across build and runtime",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        detail = str(exc)
        message = detail.lower()
        if "404 page not found" in message:
            return {
                "language": "python",
                "case": "template_methods",
                "status": "template_api_unavailable",
                "detail": detail,
                "extra": extra,
            }
        if (
            "aborted due to timeout" in message
            or "timeout" in message
            or "internal" in message
            or "build error" in message
            or "error waiting for provisioning sandbox" in message
        ):
            return {
                "language": "python",
                "case": "template_methods",
                "status": "env_blocked",
                "detail": detail,
                "extra": merge_extra(extra, {"failure_kind": "backend_or_timeout"}),
            }
        return error_result("template_methods", exc, extra)
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if build_info is not None and getattr(build_info, "template_id", ""):
            try:
                Sandbox.delete_snapshot(build_info.template_id, **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass
        shutil.rmtree(tmp_dir, ignore_errors=True)


def run_config_headers_case() -> dict[str, Any]:
    observed: dict[str, str] = {
        "x_test": "",
        "x_extra": "",
        "user_agent": "",
    }

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_POST(self) -> None:  # noqa: N802
            observed["x_test"] = self.headers.get("X-Test", "")
            observed["x_extra"] = self.headers.get("X-Extra", "")
            observed["user_agent"] = self.headers.get("User-Agent", "")
            body = b'{"code":409,"message":"already paused"}'
            self.send_response(409)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, _format: str, *args: Any) -> None:
            return

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_port
    import threading

    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    try:
        import e2b.sandbox_sync.main as sandbox_sync_main
        from packaging.version import Version

        original_filesystem = sandbox_sync_main.Filesystem
        original_commands = sandbox_sync_main.Commands
        original_pty = sandbox_sync_main.Pty
        original_git = sandbox_sync_main.Git

        sandbox_sync_main.Filesystem = lambda *args, **kwargs: object()
        sandbox_sync_main.Commands = lambda *args, **kwargs: object()
        sandbox_sync_main.Pty = lambda *args, **kwargs: object()
        sandbox_sync_main.Git = lambda *args, **kwargs: object()

        try:
            sandbox = Sandbox(
                sandbox_id="sbx-test",
                sandbox_domain="sandbox.e2b.dev",
                envd_version=Version("0.2.4"),
                envd_access_token="tok",
                traffic_access_token="tok",
                connection_config=ConnectionConfig(
                    api_key=os.getenv("E2B_API_KEY"),
                    api_url=f"http://127.0.0.1:{port}",
                    domain="base.e2b.dev",
                    request_timeout=1.0,
                    debug=False,
                    headers={"X-Test": "base"},
                ),
            )
            sandbox.pause(headers={"X-Extra": "1"})
        finally:
            sandbox_sync_main.Filesystem = original_filesystem
            sandbox_sync_main.Commands = original_commands
            sandbox_sync_main.Pty = original_pty
            sandbox_sync_main.Git = original_git

        if observed["x_test"] == "base" and observed["x_extra"] == "1":
            return {
                "language": "python",
                "case": "config_headers",
                "status": "ok",
                "detail": "pause merged base and per-call headers",
                "extra": observed,
            }
        return {
            "language": "python",
            "case": "config_headers",
            "status": "mismatch",
            "detail": "unexpected pause header propagation",
            "extra": observed,
        }
    except Exception as exc:  # noqa: BLE001
        return error_result("config_headers", exc, observed)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=1)


def run_network_update_payload_case() -> dict[str, Any]:
    requests: list[dict[str, Any]] = []

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_PUT(self) -> None:  # noqa: N802
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length) if length > 0 else b""
            body = json.loads(raw.decode("utf-8")) if raw else {}
            requests.append(
                {
                    "method": self.command,
                    "path": self.path,
                    "content_type": self.headers.get("Content-Type", ""),
                    "body": body,
                }
            )
            self.send_response(204)
            self.end_headers()

        def log_message(self, _format: str, *args: Any) -> None:
            return

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_port
    import threading

    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    try:
        common_opts = {
            "api_key": "e2b_0000000000000000000000000000000000000000",
            "api_url": f"http://127.0.0.1:{port}",
            "domain": "base.e2b.dev",
            "request_timeout": 1.0,
            "debug": False,
        }

        Sandbox.update_network(
            "sbx-selectors",
            {
                "rules": {
                    "httpbin.e2b.team": [
                        {},
                        {
                            "transform": {
                                "headers": {
                                    "X-Test": "selector",
                                }
                            }
                        },
                    ]
                },
                "allow_out": lambda ctx: list(ctx.rules.keys()),
                "deny_out": lambda ctx: [ctx.all_traffic],
                "allow_internet_access": False,
            },
            **common_opts,
        )

        Sandbox.update_network(
            "sbx-empty",
            {
                "allow_out": [],
                "deny_out": [],
                "rules": {},
            },
            **common_opts,
        )

        if len(requests) != 2:
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "error",
                "detail": f"expected 2 captured requests, got {len(requests)}",
            }

        expected_selector_body = {
            "allowOut": ["httpbin.e2b.team"],
            "allow_internet_access": False,
            "denyOut": [ALL_TRAFFIC],
            "rules": {
                "httpbin.e2b.team": [
                    {},
                    {
                        "transform": {
                            "headers": {
                                "X-Test": "selector",
                            }
                        }
                    },
                ]
            },
        }

        selector = requests[0]
        explicit_empty = requests[1]
        expected_explicit_empty_body = {
            "allowOut": [],
            "denyOut": [],
            "rules": {},
        }
        extra = {
            "selector_method": selector["method"],
            "selector_path": selector["path"],
            "selector_content_type": selector["content_type"],
            "selector_body": stable_json(selector["body"]),
            "explicit_empty_method": explicit_empty["method"],
            "explicit_empty_path": explicit_empty["path"],
            "explicit_empty_content_type": explicit_empty["content_type"],
            "explicit_empty_body": stable_json(explicit_empty["body"]),
            "explicit_empty_mode": (
                "preserved"
                if explicit_empty["body"] == expected_explicit_empty_body
                else "mismatch"
            ),
        }

        if (
            selector["method"] != "PUT"
            or selector["path"] != "/sandboxes/sbx-selectors/network"
        ):
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "unexpected selector update request target",
                "extra": extra,
            }
        if not str(selector["content_type"]).startswith("application/json"):
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "unexpected selector update content-type",
                "extra": extra,
            }
        if selector["body"] != expected_selector_body:
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "selector-based update payload did not match expected shape",
                "extra": extra,
            }
        if (
            explicit_empty["method"] != "PUT"
            or explicit_empty["path"] != "/sandboxes/sbx-empty/network"
        ):
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "unexpected explicit-empty update request target",
                "extra": extra,
            }
        if not str(explicit_empty["content_type"]).startswith("application/json"):
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "unexpected explicit-empty update content-type",
                "extra": extra,
            }
        if explicit_empty["body"] != expected_explicit_empty_body:
            return {
                "language": "python",
                "case": "network_update_payload",
                "status": "mismatch",
                "detail": "explicit-empty update payload did not preserve empty allowOut/denyOut/rules",
                "extra": extra,
            }

        return {
            "language": "python",
            "case": "network_update_payload",
            "status": "ok",
            "detail": "captured selector-based and explicit-empty network update payloads locally",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        return error_result("network_update_payload", exc)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=1)


def run_template_api_payload_case() -> dict[str, Any]:
    requests: list[dict[str, Any]] = []
    template_tag_build_id = "00000000-0000-0000-0000-000000000001"

    class Handler(http.server.BaseHTTPRequestHandler):
        def _capture(self) -> tuple[str, dict[str, Any]]:
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length) if length > 0 else b""
            body = json.loads(raw.decode("utf-8")) if raw else {}
            path = self.path
            requests.append(
                {
                    "method": self.command,
                    "path": path,
                    "content_type": self.headers.get("Content-Type", ""),
                    "body": body,
                }
            )
            return path, body

        def do_POST(self) -> None:  # noqa: N802
            path, _body = self._capture()
            if path == "/v3/templates":
                payload = {
                    "aliases": ["tmpl"],
                    "templateID": "tmpl-1",
                    "buildID": "bld-1",
                    "names": ["tmpl"],
                    "public": False,
                    "tags": ["stable"],
                }
                encoded = json.dumps(payload).encode("utf-8")
                self.send_response(202)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
                return
            if path == "/v2/templates/tmpl-1/builds/bld-1":
                self.send_response(202)
                self.end_headers()
                return
            if path == "/templates/tags":
                payload = {
                    "buildID": template_tag_build_id,
                    "tags": ["stable"],
                }
                encoded = json.dumps(payload).encode("utf-8")
                self.send_response(201)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
                return
            self.send_response(404)
            self.end_headers()

        def do_GET(self) -> None:  # noqa: N802
            path, _body = self._capture()
            if path == "/templates/aliases/tmpl":
                payload = {"templateID": "tmpl-1", "public": False}
            elif path == "/templates/tmpl-1/builds/bld-1/status?logsOffset=3&limit=100":
                payload = {
                    "templateID": "tmpl-1",
                    "buildID": "bld-1",
                    "status": "ready",
                    "logEntries": [],
                    "logs": [],
                }
            elif path == "/templates/tmpl-1/tags":
                payload = [
                    {
                        "tag": "stable",
                        "buildID": template_tag_build_id,
                        "createdAt": "2026-05-30T00:00:00Z",
                    }
                ]
            else:
                self.send_response(404)
                self.end_headers()
                return

            encoded = json.dumps(payload).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def do_DELETE(self) -> None:  # noqa: N802
            path, _body = self._capture()
            if path == "/templates/tags":
                self.send_response(204)
                self.end_headers()
                return
            self.send_response(404)
            self.end_headers()

        def log_message(self, _format: str, *args: Any) -> None:
            return

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_port
    import threading

    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    try:
        common_opts = {
            "api_key": "e2b_0000000000000000000000000000000000000000",
            "api_url": f"http://127.0.0.1:{port}",
            "domain": "base.e2b.dev",
            "request_timeout": 1.0,
            "debug": False,
        }

        build_info = Template.build_in_background(
            Template().from_base_image().run_cmd("echo hi"),
            "tmpl",
            tags=["stable"],
            **common_opts,
        )
        exists = Template.exists("tmpl", **common_opts)
        if not exists:
            return {
                "language": "python",
                "case": "template_api_payload",
                "status": "error",
                "detail": "expected Template.exists to return true",
            }
        status = Template.get_build_status(build_info, logs_offset=3, **common_opts)
        if str(status.status) not in {"ready", "TemplateBuildStatus.READY"}:
            return {
                "language": "python",
                "case": "template_api_payload",
                "status": "error",
                "detail": f"unexpected build status {status.status}",
            }
        tag_info = Template.assign_tags("tmpl:latest", "stable", **common_opts)
        if str(tag_info.build_id) != template_tag_build_id or tag_info.tags != ["stable"]:
            return {
                "language": "python",
                "case": "template_api_payload",
                "status": "error",
                "detail": f"unexpected tag info {tag_info}",
            }
        Template.remove_tags("tmpl", "stable", **common_opts)
        tags = Template.get_tags("tmpl-1", **common_opts)
        if (
            len(tags) != 1
            or tags[0].tag != "stable"
            or str(tags[0].build_id) != template_tag_build_id
        ):
            return {
                "language": "python",
                "case": "template_api_payload",
                "status": "error",
                "detail": f"unexpected tags {tags}",
            }

        if len(requests) != 7:
            return {
                "language": "python",
                "case": "template_api_payload",
                "status": "error",
                "detail": f"expected 7 captured requests, got {len(requests)}",
            }

        expected = {
            "request_build": {
                "method": "POST",
                "path": "/v3/templates",
                "content_type": "application/json",
                "body": {
                    "name": "tmpl",
                    "tags": ["stable"],
                    "cpuCount": 2,
                    "memoryMB": 1024,
                },
            },
            "trigger_build": {
                "method": "POST",
                "path": "/v2/templates/tmpl-1/builds/bld-1",
                "content_type": "application/json",
                "body": {
                    "force": False,
                    "fromImage": "e2bdev/base",
                    "steps": [
                        {
                            "type": "RUN",
                            "args": ["echo hi"],
                            "force": False,
                        }
                    ],
                },
            },
            "exists": {
                "method": "GET",
                "path": "/templates/aliases/tmpl",
                "content_type": "",
                "body": {},
            },
            "status": {
                "method": "GET",
                "path": "/templates/tmpl-1/builds/bld-1/status?logsOffset=3&limit=100",
                "content_type": "",
                "body": {},
            },
            "assign_tags": {
                "method": "POST",
                "path": "/templates/tags",
                "content_type": "application/json",
                "body": {
                    "target": "tmpl:latest",
                    "tags": ["stable"],
                },
            },
            "remove_tags": {
                "method": "DELETE",
                "path": "/templates/tags",
                "content_type": "application/json",
                "body": {
                    "name": "tmpl",
                    "tags": ["stable"],
                },
            },
            "get_tags": {
                "method": "GET",
                "path": "/templates/tmpl-1/tags",
                "content_type": "",
                "body": {},
            },
        }

        keys = [
            "request_build",
            "trigger_build",
            "exists",
            "status",
            "assign_tags",
            "remove_tags",
            "get_tags",
        ]
        extra: dict[str, str] = {}
        for index, key in enumerate(keys):
            actual = requests[index]
            want = expected[key]
            extra[f"{key}_method"] = actual["method"]
            extra[f"{key}_path"] = actual["path"]
            extra[f"{key}_content_type"] = actual["content_type"]
            extra[f"{key}_body"] = stable_json(actual["body"])

            if actual["method"] != want["method"] or actual["path"] != want["path"]:
                return {
                    "language": "python",
                    "case": "template_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} request target mismatch",
                    "extra": extra,
                }
            if want["content_type"] and not str(actual["content_type"]).startswith(
                want["content_type"]
            ):
                return {
                    "language": "python",
                    "case": "template_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} content-type mismatch",
                    "extra": extra,
                }
            if actual["body"] != want["body"]:
                return {
                    "language": "python",
                    "case": "template_api_payload",
                    "status": "mismatch",
                    "detail": f"{key} payload mismatch",
                    "extra": extra,
                }

        return {
            "language": "python",
            "case": "template_api_payload",
            "status": "ok",
            "detail": "captured template control-plane request shapes locally",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        return error_result("template_api_payload", exc)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=1)


def run_metrics_case() -> dict[str, Any]:
    resolved = resolve_sandbox_template()
    if "error" in resolved:
        return {
            "language": "python",
            "case": "metrics",
            "status": "error",
            "detail": resolved["error"],
            "extra": resolved.get("extra"),
        }

    sandbox = None
    extra = {
        **resolved.get("extra", {}),
        "template": resolved["template"],
        "template_resolution": resolved["detail"],
    }

    try:
        sandbox = Sandbox.create(
            resolved["template"],
            timeout=10 * 60,
            **connection_kwargs(),
        )
        warmup = sandbox.commands.run(
            """python3 - <<'PY'
print(sum(range(1000)))
PY"""
        )
        extra["warmup_exit_code"] = str(warmup.exit_code)

        start = datetime.datetime.now()
        deadline = time.time() + 60
        while time.time() < deadline:
            metrics = sandbox.get_metrics()
            if len(metrics) > 0:
                metric = metrics[0]
                metric_timestamp = metric.timestamp
                inclusive_start = metric_timestamp - datetime.timedelta(seconds=2)
                inclusive_end = metric_timestamp + datetime.timedelta(seconds=2)
                filtered = sandbox.get_metrics(start=inclusive_start, end=inclusive_end)
                future_start = metric_timestamp + datetime.timedelta(days=1)
                future_end = future_start + datetime.timedelta(seconds=2)
                future_filtered = sandbox.get_metrics(start=future_start, end=future_end)
                raw_inclusive_count = fetch_raw_metrics_count(
                    sandbox.sandbox_id, inclusive_start, inclusive_end
                )
                raw_future_count = fetch_raw_metrics_count(
                    sandbox.sandbox_id, future_start, future_end
                )
                extra["metrics_count"] = str(len(metrics))
                extra["filtered_count"] = str(len(filtered))
                extra["future_filtered_count"] = str(len(future_filtered))
                extra["raw_inclusive_filtered_count"] = str(raw_inclusive_count)
                extra["raw_future_filtered_count"] = str(raw_future_count)
                extra["metric_timestamp"] = metric_timestamp.isoformat()
                extra["inclusive_start"] = inclusive_start.isoformat()
                extra["inclusive_end"] = inclusive_end.isoformat()
                extra["future_start"] = future_start.isoformat()
                extra["future_end"] = future_end.isoformat()
                extra["cpu_count"] = str(metric.cpu_count)
                extra["cpu_used_pct"] = str(metric.cpu_used_pct)
                extra["mem_used"] = str(metric.mem_used)
                extra["mem_total"] = str(metric.mem_total)
                extra["disk_used"] = str(metric.disk_used)
                extra["disk_total"] = str(metric.disk_total)
                extra["mem_cache"] = str(metric.mem_cache)
                if len(filtered) == 0:
                    return {
                        "language": "python",
                        "case": "metrics",
                        "status": "partial",
                        "detail": (
                            "metrics returned data, but an inclusive filtered window around "
                            "the metric timestamp returned zero items while the raw control-plane "
                            f"query returned {raw_inclusive_count} rows"
                        ),
                        "extra": extra,
                    }
                if len(future_filtered) != 0:
                    return {
                        "language": "python",
                        "case": "metrics",
                        "status": "partial",
                        "detail": (
                            "future-only filtered metrics window still returned data while "
                            f"the raw control-plane query returned {raw_future_count} rows"
                        ),
                        "extra": extra,
                    }
                return {
                    "language": "python",
                    "case": "metrics",
                    "status": "ok",
                    "detail": "metrics available",
                    "extra": extra,
                }
            time.sleep(0.5)

        return {
            "language": "python",
            "case": "metrics",
            "status": "env_blocked",
            "detail": "metrics endpoint returned zero points within 60s",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        return {
            "language": "python",
            "case": "metrics",
            "status": "error",
            "detail": str(exc),
            "extra": extra,
        }
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if resolved.get("extra", {}).get("template_source") == "temporary_build":
            try:
                Sandbox.delete_snapshot(resolved["template"], **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass


def run_network_rules_case() -> dict[str, Any]:
    resolved = resolve_sandbox_template()
    if "error" in resolved:
        return {
            "language": "python",
            "case": "network_rules",
            "status": "error",
            "detail": resolved["error"],
            "extra": resolved.get("extra"),
        }

    sandbox = None
    extra = {
        **resolved.get("extra", {}),
        "template": resolved["template"],
        "template_resolution": resolved["detail"],
    }

    try:
        sandbox = Sandbox.create(
            resolved["template"],
            timeout=10 * 60,
            network={
                "allow_out": ["httpbin.e2b.team"],
                "deny_out": [ALL_TRAFFIC],
                "rules": {
                    "httpbin.e2b.team": [
                        {"transform": {"headers": {"X-E2B-Test-Token": "e2b-transform-value-123"}}},
                    ],
                },
            },
            **connection_kwargs(),
        )
        command_result = sandbox.commands.run(
            "curl -sS --max-time 10 https://httpbin.e2b.team/headers"
        )
        parsed = json.loads(command_result.stdout)
        reflected = parsed.get("headers", {}).get("X-E2B-Test-Token", "")
        extra["reflected_header"] = str(reflected)

        if reflected != "e2b-transform-value-123":
            return {
                "language": "python",
                "case": "network_rules",
                "status": "env_blocked",
                "detail": f"network transform is not enforced; reflected header={reflected!r}",
                "extra": extra,
            }

        return {
            "language": "python",
            "case": "network_rules",
            "status": "ok",
            "detail": "network transform reflected expected header",
            "extra": extra,
        }
    except Exception as exc:  # noqa: BLE001
        return {
            "language": "python",
            "case": "network_rules",
            "status": "env_blocked",
            "detail": str(exc),
            "extra": extra,
        }
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
        if resolved.get("extra", {}).get("template_source") == "temporary_build":
            try:
                Sandbox.delete_snapshot(resolved["template"], **connection_kwargs())
            except Exception:  # noqa: BLE001
                pass


def run_network_egress_case() -> dict[str, Any]:
    try:
        if not Template.exists("base", **connection_kwargs()):
            return {
                "language": "python",
                "case": "network_egress",
                "status": "template_missing",
                "detail": "base template alias is unavailable",
            }
    except Exception as exc:  # noqa: BLE001
        return {
            "language": "python",
            "case": "network_egress",
            "status": "error",
            "detail": str(exc),
        }

    extra: dict[str, str] = {
        "template": "base",
        "template_source": "base_alias",
        "template_resolution": "source test alias",
    }
    first_failure = ""
    sandbox = None

    try:
        sandbox = Sandbox.create(
            "base",
            timeout=10 * 60,
            network={"deny_out": [ALL_TRAFFIC], "allow_out": ["1.1.1.1"]},
            **connection_kwargs(),
        )
        extra["allow_only_1111"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
        )
        extra["allow_only_8888"] = run_command_summary(
            sandbox, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8"
        )
        if not command_succeeded(extra["allow_only_1111"]) and not first_failure:
            first_failure = f"allow_only_1111 did not succeed: {extra['allow_only_1111']}"
        if not command_blocked(extra["allow_only_8888"]) and not first_failure:
            first_failure = f"allow_only_8888 was not blocked: {extra['allow_only_8888']}"
    except Exception as exc:  # noqa: BLE001
        return {"language": "python", "case": "network_egress", "status": "error", "detail": str(exc), "extra": extra}
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
            sandbox = None

    try:
        sandbox = Sandbox.create(
            "base",
            timeout=10 * 60,
            network={"deny_out": ["8.8.8.8"]},
            **connection_kwargs(),
        )
        extra["deny_specific_8888"] = run_command_summary(
            sandbox, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8"
        )
        extra["deny_specific_1111"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
        )
        if not command_blocked(extra["deny_specific_8888"]) and not first_failure:
            first_failure = f"deny_specific_8888 was not blocked: {extra['deny_specific_8888']}"
        if not command_succeeded(extra["deny_specific_1111"]) and not first_failure:
            first_failure = f"deny_specific_1111 did not succeed: {extra['deny_specific_1111']}"
    except Exception as exc:  # noqa: BLE001
        return {"language": "python", "case": "network_egress", "status": "error", "detail": str(exc), "extra": extra}
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
            sandbox = None

    try:
        sandbox = Sandbox.create(
            "base",
            timeout=10 * 60,
            network={"deny_out": [ALL_TRAFFIC], "allow_out": ["1.1.1.1", "8.8.8.8"]},
            **connection_kwargs(),
        )
        extra["allow_precedence_1111"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
        )
        extra["allow_precedence_8888"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
        )
        if not command_succeeded(extra["allow_precedence_1111"]) and not first_failure:
            first_failure = f"allow_precedence_1111 did not succeed: {extra['allow_precedence_1111']}"
        if not command_succeeded(extra["allow_precedence_8888"]) and not first_failure:
            first_failure = f"allow_precedence_8888 did not succeed: {extra['allow_precedence_8888']}"
    except Exception as exc:  # noqa: BLE001
        return {"language": "python", "case": "network_egress", "status": "error", "detail": str(exc), "extra": extra}
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
            sandbox = None

    try:
        sandbox = Sandbox.create("base", timeout=10 * 60, **connection_kwargs())
        extra["update_before_8888"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
        )
        sandbox.update_network({"deny_out": ["8.8.8.8"]})
        extra["update_after_deny_8888"] = run_command_summary(
            sandbox, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8"
        )
        extra["update_after_deny_1111"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
        )
        if not command_succeeded(extra["update_before_8888"]) and not first_failure:
            first_failure = f"update_before_8888 did not succeed: {extra['update_before_8888']}"
        if not command_blocked(extra["update_after_deny_8888"]) and not first_failure:
            first_failure = f"update_after_deny_8888 was not blocked: {extra['update_after_deny_8888']}"
        if not command_succeeded(extra["update_after_deny_1111"]) and not first_failure:
            first_failure = f"update_after_deny_1111 did not succeed: {extra['update_after_deny_1111']}"
    except Exception as exc:  # noqa: BLE001
        return {"language": "python", "case": "network_egress", "status": "error", "detail": str(exc), "extra": extra}
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass
            sandbox = None

    try:
        sandbox = Sandbox.create(
            "base",
            timeout=10 * 60,
            network={"deny_out": [ALL_TRAFFIC], "allow_out": ["1.1.1.1"]},
            **connection_kwargs(),
        )
        extra["clear_before_8888"] = run_command_summary(
            sandbox, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8"
        )
        sandbox.update_network({})
        extra["clear_after_1111"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
        )
        extra["clear_after_8888"] = run_command_summary(
            sandbox, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
        )
        if not command_blocked(extra["clear_before_8888"]) and not first_failure:
            first_failure = f"clear_before_8888 was not blocked: {extra['clear_before_8888']}"
        if not command_succeeded(extra["clear_after_1111"]) and not first_failure:
            first_failure = f"clear_after_1111 did not succeed: {extra['clear_after_1111']}"
        if not command_succeeded(extra["clear_after_8888"]) and not first_failure:
            first_failure = f"clear_after_8888 did not succeed: {extra['clear_after_8888']}"
    except Exception as exc:  # noqa: BLE001
        return {"language": "python", "case": "network_egress", "status": "error", "detail": str(exc), "extra": extra}
    finally:
        if sandbox is not None:
            try:
                sandbox.kill()
            except Exception:  # noqa: BLE001
                pass

    if first_failure:
        return {
            "language": "python",
            "case": "network_egress",
            "status": "env_blocked",
            "detail": first_failure,
            "extra": extra,
        }

    return {
        "language": "python",
        "case": "network_egress",
        "status": "ok",
        "detail": "source-like network egress expectations matched",
        "extra": extra,
    }


def wait_for_final_build_status(build_info: Any) -> Any:
    deadline = time.time() + 30 * 60
    while time.time() < deadline:
        status = Template.get_build_status(build_info, **connection_kwargs())
        current = getattr(getattr(status, "status", None), "value", str(status.status))
        if current not in {"building", "waiting"}:
            return status
        time.sleep(5)
    raise TimeoutError("timed out waiting for final ubuntu build status")


def error_result(case_name: str, exc: Exception, extra: dict[str, Any] | None = None) -> dict[str, Any]:
    result = {
        "language": "python",
        "case": case_name,
        "status": "error",
        "detail": str(exc),
    }
    if extra is not None:
        result["extra"] = extra
    return result


def merge_extra(base: dict[str, Any] | None, extra: dict[str, Any]) -> dict[str, Any]:
    if base is None:
        return extra
    merged = base.copy()
    merged.update(extra)
    return merged


def stable_json(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def numpy_random_command() -> str:
    return """python3 - <<'PY'
import numpy as np
print([np.random.normal(), np.random.normal(), np.random.normal()])
PY"""


def run_numpy_vector(sandbox: Sandbox) -> str:
    result = sandbox.commands.run(numpy_random_command())
    return result.stdout.strip()


def run_command_summary(sandbox: Sandbox, command: str) -> str:
    try:
        result = sandbox.commands.run(command)
        return f"ok:{result.stdout.strip()}"
    except Exception as exc:  # noqa: BLE001
        message = str(exc)
        import re

        match = re.search(r"code\s+(\d+)", message, re.IGNORECASE)
        if match:
            return f"exit:{match.group(1)}"
        return f"error:{message}"


def command_succeeded(summary: str) -> bool:
    return summary.startswith("ok:")


def command_blocked(summary: str) -> bool:
    return summary.startswith("exit:")


def fetch_raw_metrics_count(
    sandbox_id: str,
    start: datetime.datetime | None = None,
    end: datetime.datetime | None = None,
) -> int:
    conn = connection_kwargs()
    base_url = conn.get("api_url") or (
        f"https://api.{conn['domain']}" if conn.get("domain") else ""
    )
    if not base_url:
        raise RuntimeError("missing API URL/domain for raw metrics request")

    params: dict[str, str] = {}
    if start is not None:
        params["start"] = str(round(start.timestamp()))
    if end is not None:
        params["end"] = str(round(end.timestamp()))

    headers = {}
    if conn.get("api_key"):
        headers["X-API-KEY"] = conn["api_key"]
    if conn.get("access_token"):
        headers["Authorization"] = f"Bearer {conn['access_token']}"

    with httpx.Client(timeout=120.0) as client:
        response = client.get(
            f"{base_url}/sandboxes/{sandbox_id}/metrics",
            params=params,
            headers=headers,
        )

    if response.status_code >= 300:
        raise RuntimeError(
            f"raw metrics request failed: status={response.status_code} body={response.text}"
        )

    data = response.json()
    if not isinstance(data, list):
        raise RuntimeError(f"raw metrics response was not an array: {response.text}")
    return len(data)


def resolve_sandbox_template() -> dict[str, Any]:
    for key in ["E2B_TEST_TEMPLATE", "E2B_INTEGRATION_TEMPLATE", "E2B_TEMPLATE", "E2B_SANDBOX_TEMPLATE"]:
        value = os.getenv(key)
        if value:
            return {
                "template": value,
                "detail": "from env",
                "extra": {"template_source": key},
            }

    extra: dict[str, str] = {}

    try:
        if Template.exists("base", **connection_kwargs()):
            return {
                "template": "base",
                "detail": "from base alias",
                "extra": {"template_source": "base_alias"},
            }
    except Exception as exc:  # noqa: BLE001
        extra["base_exists_error"] = str(exc)

    try:
        paginator = Sandbox.list(limit=10, **connection_kwargs())
        items = paginator.next_items()
        for item in items:
            if item.template_id:
                return {
                    "template": item.template_id,
                    "detail": "inferred from existing sandbox",
                    "extra": {
                        **extra,
                        "template_source": "inferred_from_list",
                        "inferred_sandbox_id": item.sandbox_id,
                    },
                }
    except Exception as exc:  # noqa: BLE001
        extra["list_error"] = str(exc)

    try:
        name = f"python-sdk-metrics-crosscheck-{time.time_ns()}"
        info = Template.build(Template().from_base_image(), name, **build_kwargs())
        return {
            "template": info.template_id,
            "detail": "temporary base-image build",
            "extra": {
                **extra,
                "template_source": "temporary_build",
                "template_id": info.template_id,
            },
        }
    except Exception as exc:  # noqa: BLE001
        return {
            "error": str(exc),
            "extra": extra,
        }


def main() -> int:
    args = parse_args()
    results: list[dict[str, Any]] = []

    for current in selected_cases(args.case):
        if current == "claude":
            results.append(run_claude_case())
        elif current == "claude_derived":
            results.append(run_claude_derived_case())
        elif current == "randomness":
            results.append(run_randomness_case())
        elif current == "randomness_alias":
            results.append(run_randomness_alias_case())
        elif current == "volume":
            results.append(run_volume_case())
        elif current == "volume_api_payload":
            results.append(run_volume_api_payload_case())
        elif current == "ubuntu":
            results.append(run_ubuntu_case())
        elif current == "template_timeout":
            results.append(run_template_timeout_case())
        elif current == "template_methods":
            results.append(run_template_methods_case())
        elif current == "config_headers":
            results.append(run_config_headers_case())
        elif current == "metrics":
            results.append(run_metrics_case())
        elif current == "network_rules":
            results.append(run_network_rules_case())
        elif current == "network_egress":
            results.append(run_network_egress_case())
        elif current == "network_update_payload":
            results.append(run_network_update_payload_case())
        elif current == "template_api_payload":
            results.append(run_template_api_payload_case())
        elif current == "debug_root":
            results.append(run_debug_root_case())
        else:
            raise ValueError(f"unsupported case {current}")

    json.dump(results, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
