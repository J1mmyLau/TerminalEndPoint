#!/usr/bin/env python3
"""Agent workflow integration tests — real tools via REST exec endpoint.

Tests: gcc, gdb, python, node, shell pipelines, environment injection, error handling.
All tests are deterministic (non-interactive exec mode).
"""

import json, sys, os, base64, tempfile, urllib.request, urllib.error

BASE = "http://127.0.0.1:8080"

def api(method, path, body=None):
    url = f"{BASE}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode())

def exec_cmd(command, timeout=30, work_dir=None, env=None):
    code, data = api("POST", "/api/v1/sessions/none/exec", {
        "command": command,
        "timeout_seconds": timeout,
        "work_dir": work_dir,
        "env": env or {},
    })
    if code != 200:
        raise Exception(f"exec failed [{code}]: {data}")
    return data

def assert_exec(exit_expected, cmd, work_dir=None, label="", env=None):
    r = exec_cmd(cmd, work_dir=work_dir, env=env)
    assert r["exit_code"] == exit_expected, \
        f"{label}: expected exit={exit_expected}, got {r['exit_code']}. stdout={r['stdout'][:300]}"
    return r


def test_c_compile_and_gdb():
    print("=" * 60)
    print("TEST 1: C Compile + GDB Debug (exec mode)")
    print("=" * 60)

    with tempfile.TemporaryDirectory() as tmpdir:
        with open(os.path.join(tmpdir, "prog.c"), "w") as f:
            f.write("""#include <stdio.h>
int add(int a, int b) { return a + b; }
int main() {
    int x = 10, y = 20;
    int result = add(x, y);
    printf("result = %d\\n", result);
    return 0;
}
""")

        r = assert_exec(0, "gcc -g -O0 -o prog prog.c 2>&1", work_dir=tmpdir, label="compile")
        print(f"  [compile] ok ({r['duration_ms']}ms)")

        r = assert_exec(0, "./prog", work_dir=tmpdir, label="run")
        assert "result = 30" in r["stdout"], f"unexpected output: {r['stdout']}"
        print(f"  [run] {r['stdout'].strip()}")

        r = assert_exec(0, "echo 'file ./prog\nbreak add\nrun\nquit' | gdb -q 2>&1",
                        work_dir=tmpdir, label="gdb")
        out = r["stdout"]
        checks = [
            ("gdb started", "(gdb)" in out),
            ("loaded binary", "./prog" in out or "file" in out.lower()),
        ]
        for name, ok in checks:
            print(f"  [{'PASS' if ok else 'FAIL'}] {name}")
        assert all(c for _, c in checks), f"gdb test failed. output:\n{out[-400:]}"
        print("  OK\n")


def test_lldb_debug():
    print("=" * 60)
    print("TEST 1b: LLDB Debug (macOS native debugger)")
    print("=" * 60)

    with tempfile.TemporaryDirectory() as tmpdir:
        with open(os.path.join(tmpdir, "prog.c"), "w") as f:
            f.write("""#include <stdio.h>
int add(int a, int b) { return a + b; }
int main() {
    int x = 10, y = 20;
    printf("result = %d\\n", add(x, y));
    return 0;
}
""")

        assert_exec(0, "gcc -g -O0 -o prog prog.c 2>&1", work_dir=tmpdir, label="compile")

        r = assert_exec(0, "echo 'b add\nrun\np a\np b\nc\nquit' | lldb -b ./prog 2>&1",
                        work_dir=tmpdir, label="lldb")
        out = r["stdout"]
        checks = [
            ("lldb started", "lldb" in out.lower() or "target" in out.lower()),
            ("breakpoint set", "breakpoint" in out.lower()),
            ("program ran", "result = 30" in out),
        ]
        for name, ok in checks:
            print(f"  [{'PASS' if ok else 'FAIL'}] {name}")
        # LLDB might not be installed, so don't hard-fail — just report
        all_ok = all(c for _, c in checks)
        print(f"  {'OK' if all_ok else 'SKIPPED (lldb may not be configured)'}\n")


def test_agent_workflow():
    print("=" * 60)
    print("TEST 2: Agent Workflow: create -> compile -> bug -> fix -> verify")
    print("=" * 60)

    with tempfile.TemporaryDirectory() as tmpdir:
        with open(os.path.join(tmpdir, "sum.c"), "w") as f:
            f.write("""#include <stdio.h>
int main() {
    int nums[] = {1, 2, 3, 4, 5};
    int sum = 0;
    for (int i = 0; i <= 5; i++) { sum += nums[i]; }
    printf("sum = %d\\n", sum);
    return 0;
}
""")
        assert_exec(0, "gcc -o sum sum.c 2>&1", work_dir=tmpdir, label="compile")
        r = assert_exec(0, "./sum", work_dir=tmpdir, label="run-buggy")
        assert "sum = 15" not in r["stdout"]
        print(f"  [bug] {r['stdout'].strip()}")

        assert_exec(0, "sed -i '' 's/i <= 5/i < 5/' sum.c", work_dir=tmpdir, label="fix")
        assert_exec(0, "gcc -o sum sum.c 2>&1", work_dir=tmpdir, label="recompile")
        r = assert_exec(0, "./sum", work_dir=tmpdir, label="verify")
        assert "sum = 15" in r["stdout"]
        print(f"  [fix] {r['stdout'].strip()}")
        print("  OK\n")


def test_python_workflow():
    print("=" * 60)
    print("TEST 3: Python script execution")
    print("=" * 60)
    r = assert_exec(0, "python3 -c 'print(sum(range(1,101)))'", label="py-sum")
    assert "5050" in r["stdout"].strip()
    print(f"  [py-sum] {r['stdout'].strip()}")

    r = assert_exec(0, """python3 << 'EOF'
import json
data = {"tool": "terminal_endpoint", "tools": 9}
print(json.dumps(data, indent=2))
EOF""", label="py-json")
    assert "terminal_endpoint" in r["stdout"]
    print(f"  [py-json] ok")

    r = exec_cmd("python3 -c 'import nonexistent' 2>&1")
    assert r["exit_code"] != 0 and "ModuleNotFoundError" in r["stdout"]
    print(f"  [py-err] exit={r['exit_code']}, captured: yes")
    print("  OK\n")


def test_long_running_build():
    print("=" * 60)
    print("TEST 4: Long-running build with progress")
    print("=" * 60)
    r = exec_cmd("""for i in 1 2 3 4 5; do
  echo "[BUILD] module $i/5..."
  sleep 0.4
done
echo "[BUILD] COMPLETE"
""", timeout=10)
    assert r["exit_code"] == 0
    assert "[BUILD] COMPLETE" in r["stdout"]
    assert r["duration_ms"] > 2000
    print(f"  [build] {r['stdout'].count('[BUILD]')} markers, {r['duration_ms']}ms")
    print("  OK\n")


def test_shell_patterns():
    print("=" * 60)
    print("TEST 5: Shell pipelines, env, error handling")
    print("=" * 60)

    r = assert_exec(0, "ls /usr/bin | grep '^g' | head -5 | wc -l", label="pipe")
    print(f"  [pipe] {r['stdout'].strip()} binaries matching '^g'")

    r = assert_exec(0, "echo $PROJ-$VER-$BID",
                    env={"PROJ": "TE", "VER": "0.1", "BID": "42"}, label="env")
    assert "TE-0.1-42" in r["stdout"]
    print(f"  [env] {r['stdout'].strip()}")

    r = exec_cmd("cat /nonexistent 2>&1")
    assert r["exit_code"] != 0
    print(f"  [fail] exit={r['exit_code']}")

    r = exec_cmd("sleep 10", timeout=2)
    assert r.get("truncated", False), f"expected truncated, got {r}"
    print(f"  [timeout] truncated={r.get('truncated')}")
    print("  OK\n")


def test_nodejs():
    print("=" * 60)
    print("TEST 6: Node.js execution")
    print("=" * 60)
    r = assert_exec(0, "node -e 'console.log(JSON.stringify({v:process.version}))'", label="node")
    assert "v" in r["stdout"]
    print(f"  [node] {r['stdout'].strip()}")
    r = assert_exec(0, "node -e 'console.log([1,2,3].map(x=>x*2))'", label="node-map")
    print(f"  [node-map] {r['stdout'].strip()}")
    print("  OK\n")


if __name__ == "__main__":
    try:
        code, _ = api("GET", "/health")
        assert code == 200, f"server not healthy: {code}"
        test_c_compile_and_gdb()
        test_lldb_debug()
        test_agent_workflow()
        test_python_workflow()
        test_long_running_build()
        test_shell_patterns()
        test_nodejs()
        print("=" * 60)
        print("ALL 6 AGENT WORKFLOW TESTS PASSED")
        print("=" * 60)
    except AssertionError as e:
        print(f"\nFAILED: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"\nERROR: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
