#!/usr/bin/env python3
"""Integration test script for TerminalEndPoint Phase 2 & 3 features."""

import json
import sys
import time
import subprocess
import urllib.request
import urllib.error
import threading
import websocket

BASE_URL = "http://127.0.0.1:8080"
WS_URL = "ws://127.0.0.1:8080"

def api(method, path, body=None):
    url = f"{BASE_URL}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode())
    except Exception as e:
        return -1, str(e)

def test_health():
    print("=== Test: Health ===")
    code, data = api("GET", "/health")
    assert code == 200, f"Expected 200, got {code}"
    assert data["status"] == "ok"
    print(f"  PASS: {data}")

def test_create_session():
    print("=== Test: Create Session ===")
    code, data = api("POST", "/api/v1/sessions", {"label": "ws-test"})
    assert code == 201, f"Expected 201, got {code}: {data}"
    assert "id" in data
    assert data["status"] == "running"
    print(f"  PASS: session {data['id']}")
    return data["id"]

def test_list_sessions(expected_count):
    print("=== Test: List Sessions ===")
    code, data = api("GET", "/api/v1/sessions")
    assert code == 200
    assert data["count"] >= expected_count
    print(f"  PASS: {data['count']} sessions")

def test_websocket_repl(session_id):
    print("=== Test: WebSocket Interactive REPL ===")
    received = []
    ws_connected = threading.Event()
    ws_done = threading.Event()

    def on_message(ws, message):
        msg = json.loads(message)
        received.append(msg)
        msg_type = msg.get("type", "unknown")
        if msg_type == "output":
            data_obj = msg.get("data", {})
            raw = websocket_b64decode(data_obj.get("data", ""))
            text = raw.decode("utf-8", errors="replace").rstrip()
            print(f"  WS output: {repr(text[:80])}...")
        elif msg_type == "exit":
            print(f"  WS exit: code={msg['data']['code']}")
        elif msg_type == "error":
            print(f"  WS error: {msg['data']['message']}")
        else:
            print(f"  WS {msg_type}")

    def on_open(ws):
        ws_connected.set()
        print("  WS connected")
        ws.send(json.dumps({"type": "write", "data": "echo 'Hello REPL'\n"}))
        time.sleep(0.3)
        ws.send(json.dumps({"type": "write", "data": "echo 'Second line'\n"}))
        time.sleep(0.5)
        ws_done.set()

    def on_error(ws, error):
        print(f"  WS error: {error}")

    def on_close(ws, code, msg):
        print(f"  WS closed: {code}")

    ws = websocket.WebSocketApp(
        f"{WS_URL}/ws/sessions/{session_id}",
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close,
    )

    thread = threading.Thread(target=ws.run_forever, kwargs={"ping_interval": 10})
    thread.daemon = True
    thread.start()

    assert ws_connected.wait(timeout=5), "WebSocket failed to connect"
    ws_done.wait(timeout=5)
    time.sleep(0.5)
    ws.close()
    thread.join(timeout=5)

    output_events = [m for m in received if m.get("type") == "output"]
    assert len(output_events) > 0, "No output events received"
    for e in output_events:
        data_obj = e.get("data", {})
        raw = websocket_b64decode(data_obj.get("data", ""))
        assert len(raw) > 0, "Output event has empty data"
    print(f"  PASS: received {len(output_events)} output events")

def test_since_seq_recovery(session_id):
    print("=== Test: since_seq Recovery ===")
    received = []
    ws_connected = threading.Event()
    ws_done = threading.Event()

    def on_message(ws, message):
        msg = json.loads(message)
        received.append(msg)
        if msg.get("type") == "history":
            print(f"  WS history: {len(msg.get('data',{}).get('entries',[]))} entries")
        elif msg.get("type") == "output":
            data_obj = msg.get("data", {})
            raw = websocket_b64decode(data_obj.get("data", ""))
            print(f"  WS output seq={msg.get('data',{}).get('seq')}: {raw.decode('utf-8', errors='replace').rstrip()[:60]}")

    def on_open(ws):
        ws_connected.set()
        ws.send(json.dumps({"type": "write", "data": "echo 'reconnect test'\n"}))
        time.sleep(0.5)
        ws_done.set()

    def on_error(ws, error):
        print(f"  WS error: {error}")

    def on_close(ws, code, msg):
        pass

    ws = websocket.WebSocketApp(
        f"{WS_URL}/ws/sessions/{session_id}?since_seq=1",
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close,
    )

    thread = threading.Thread(target=ws.run_forever, kwargs={"ping_interval": 10})
    thread.daemon = True
    thread.start()

    assert ws_connected.wait(timeout=5), "WebSocket reconnection failed"
    ws_done.wait(timeout=5)
    time.sleep(0.5)
    ws.close()
    thread.join(timeout=5)

    history_events = [m for m in received if m.get("type") == "history"]
    assert len(history_events) > 0, "No history event on reconnect"
    print(f"  PASS: history replay with {len(history_events[0].get('data',{}).get('entries',[]))} entries")

def test_output_api(session_id):
    print("=== Test: Output Query API ===")
    code, data = api("GET", f"/api/v1/sessions/{session_id}/output")
    assert code == 200, f"Expected 200, got {code}"
    assert "entries" in data
    assert "latest_seq" in data
    print(f"  PASS: {len(data['entries'])} entries, latest_seq={data['latest_seq']}")

    # Test with since param
    code, data2 = api("GET", f"/api/v1/sessions/{session_id}/output?since=1&limit=5")
    assert code == 200
    print(f"  PASS: since=1,limit=5 → {len(data2['entries'])} entries")

def test_signal(session_id):
    print("=== Test: Signal ===")
    code, data = api("POST", f"/api/v1/sessions/{session_id}/signal", {"signal": "SIGKILL"})
    assert code == 200, f"Expected 200, got {code}: {data}"
    assert data["status"] == "signaled"
    print(f"  PASS: signal sent")

    time.sleep(0.5)
    code, data = api("GET", f"/api/v1/sessions/{session_id}")
    assert code == 200
    assert data["status"] in ("exited", "killed"), f"Expected exited/killed, got {data['status']}"
    print(f"  PASS: session status = {data['status']}")

def test_resize(session_id):
    print("=== Test: Resize ===")
    code, data = api("POST", f"/api/v1/sessions/{session_id}/resize", {"rows": 40, "cols": 120})
    assert code == 200, f"Expected 200, got {code}: {data}"
    assert data["status"] == "resized"
    print(f"  PASS: terminal resized")

def websocket_b64decode(s):
    import base64
    return base64.b64decode(s)

if __name__ == "__main__":
    try:
        test_health()
        sid = test_create_session()
        test_list_sessions(1)
        test_websocket_repl(sid)
        test_since_seq_recovery(sid)
        test_resize(sid)
        test_signal(sid)
        test_output_api(sid)
        print("\n✅ ALL TESTS PASSED")
    except AssertionError as e:
        print(f"\n❌ TEST FAILED: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ ERROR: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
