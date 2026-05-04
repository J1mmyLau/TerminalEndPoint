#!/usr/bin/env python3
"""Minimal WebSocket debug test."""
import json, time, threading, websocket, sys, urllib.request

BASE = "http://127.0.0.1:8080"

def api(method, path, body=None):
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(f"{BASE}{path}", data=data, method=method)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read())

# Create session
session = api("POST", "/api/v1/sessions", {"label": "debug"})
sid = session["id"]
print(f"SESSION: {sid}")

received = []
connected = threading.Event()

def on_message(ws, msg):
    parsed = json.loads(msg)
    received.append(parsed)
    print(f"  RECV: {parsed['type']}")

def on_open(ws):
    connected.set()
    print("CONNECTED")
    ws.send(json.dumps({"type": "write", "data": "echo HELLO_WORLD\n"}))

def on_error(ws, err):
    print(f"ERROR: {err}")

def on_close(ws, code, msg):
    print(f"CLOSED: {code}")

ws = websocket.WebSocketApp(
    f"ws://127.0.0.1:8080/ws/sessions/{sid}",
    on_open=on_open, on_message=on_message, on_error=on_error, on_close=on_close,
)

t = threading.Thread(target=ws.run_forever)
t.daemon = True
t.start()

if not connected.wait(timeout=5):
    print("FAILED TO CONNECT")
    sys.exit(1)

time.sleep(2)
ws.close()
t.join(timeout=3)

print(f"\nReceived {len(received)} messages:")
for r in received:
    t = r['type']
    if t == 'output':
        import base64
        data = base64.b64decode(r['data']['data']).decode('utf-8', errors='replace')
        print(f"  [{t}] seq={r['data']['seq']} data={repr(data)}")
    else:
        print(f"  [{t}] {r.get('data', '')}")
